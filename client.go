package nutanix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	libraryVersion             = "v3"
	absolutePath               = "api/nutanix/" + libraryVersion
	defaultV2BaseURL           = "PrismGateway/services/rest/v2.0"
	userAgent                  = "nutanix/" + "cmd.Version"
	itemsPerPage         int64 = 500
	mediaTypeJSON              = "application/json"
	mediaTypeUpload            = "application/octet-stream"
	ResponseMaxSizeBytes       = 10 * 1024 * 1024
)

// ClientOption ...
type ClientOption func(*Client)

// Client Config Configuration of the client
type Client struct {
	baseURL     *url.URL
	credentials *Credentials
	httpClient  *http.Client
	userAgent   string
	skipVerify  bool

	Image            ImageClient
	Cluster          ClusterClient
	Project          ProjectClient
	VM               VMClient
	Subnet           SubnetClient
	Category         CategoryClient
	Task             TaskClient
	Snapshot         SnapshotClient
	AvailabilityZone AvailabilityZoneClient
	VMRecoveryPoint  VMRecoveryPointClient
}

// Credentials needed username and password
type Credentials struct {
	Username string
	Password string
}

// WithCredentials configures a Client to use the specified credentials for authentication.
func WithCredentials(cred *Credentials) ClientOption {
	return func(client *Client) {
		client.credentials = cred
	}
}

// WithEndpoint configures a Client to use the specified credentials for authentication.
func WithEndpoint(endpoint string) ClientOption {
	return func(client *Client) {
		passedURL := endpoint

		// Required because url.Parse returns an empty string for the hostname if there was no schema
		if !strings.HasPrefix(passedURL, "https://") && !strings.HasPrefix(passedURL, "http://") {
			passedURL = "https://" + passedURL
		}

		client.baseURL, _ = url.Parse(passedURL)
	}
}

// WithHTTPClient allows to specify a custom http client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// WithSkipVerify returns a ClientOption that configure the client connection to not verify https connectins
func WithSkipVerify() ClientOption {
	return func(client *Client) {
		client.skipVerify = true
	}
}

// NewClient creates a new client.
func NewClient(options ...ClientOption) *Client {
	client := &Client{}

	for _, option := range options {
		option(client)
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{}
	}

	client.userAgent = userAgent
	transCfg := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: client.skipVerify},
	}
	client.httpClient.Transport = transCfg

	client.Image = ImageClient{client: client}
	client.Cluster = ClusterClient{client: client}
	client.Project = ProjectClient{client: client}
	client.VM = VMClient{client: client}
	client.Subnet = SubnetClient{client: client}
	client.Category = CategoryClient{client: client}
	client.Task = TaskClient{client: client}
	client.Snapshot = SnapshotClient{client: client}
	client.AvailabilityZone = AvailabilityZoneClient{client: client}
	client.VMRecoveryPoint = VMRecoveryPointClient{client: client}
	return client
}

// Do performs request passed
func (c *Client) Do(r *http.Request, v interface{}) error {
	resp, err := c.httpClient.Do(r)
	if err != nil {
		return err
	}

	defer func() {
		if rerr := resp.Body.Close(); err == nil {
			err = rerr
		}
	}()

	if c := resp.StatusCode; c >= 200 && c <= 299 && resp.Request.Method == http.MethodDelete {
		return nil
	}

	if c := resp.StatusCode; c >= 500 || c == 401 || c == 404 {
		return fmt.Errorf("statusCode: %d", c)
	}

	var res map[string]interface{}
	err = json.NewDecoder(io.LimitReader(resp.Body, ResponseMaxSizeBytes)).Decode(&res)
	if err != nil {
		return err
	}
	if err = parseError(res); err != nil {
		return err
	}

	err = mapstructure.Decode(res, &v)
	return err
}

func parseError(res map[string]interface{}) error {
	// if has entities -> return nil
	// if has message_list -> check_error["state"]
	// if has status -> check_error["status.state"]
	var err error
	errRes := &schema.ErrorResponse{}
	if status, ok := res["status"]; ok {
		_, sok := status.(string)
		if sok {
			return nil
		}
		err = fillStruct(status.(map[string]interface{}), errRes)
	} else if _, ok := res["state"]; ok {
		err = fillStruct(res, errRes)
	} else if _, ok := res["entities"]; ok {
		return nil
	}

	if err != nil {
		return err
	}

	if errRes.State != "ERROR" {
		return nil
	}

	pretty, _ := json.MarshalIndent(errRes, "", "  ")
	return fmt.Errorf(string(pretty))
}

func (c *Client) setHeaders(req *http.Request) {
	req.SetBasicAuth(c.credentials.Username, c.credentials.Password)
	req.Header.Set("User-Agent", c.userAgent)
}

// NewV3PCRequest ...
func (c *Client) NewV3PCRequest(ctx context.Context, method string, path string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(absolutePath + path)
	if err != nil {
		return nil, err
	}
	url := c.baseURL.ResolveReference(rel)
	return c.newV3Request(ctx, method, url, body)
}

// NewV3PERequest ...
func (c *Client) NewV3PERequest(ctx context.Context, method string, clusterUUID string, path string, body interface{}) (*http.Request, error) {
	cluster, err := c.Cluster.GetByUUID(ctx, clusterUUID)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(absolutePath + path)
	if err != nil {
		return nil, err
	}

	urlEndpoint, _ := url.Parse(fmt.Sprintf("%s://%s:%s", c.baseURL.Scheme, cluster.Spec.Resources.Network.ExternalIP, c.baseURL.Port()))

	url := urlEndpoint.ResolveReference(rel)
	return c.newV3Request(ctx, method, url, body)
}

func (c *Client) newV3Request(ctx context.Context, method string, url *url.URL, body interface{}) (*http.Request, error) {
	var contentBody io.Reader
	var contentType string

	switch b := body.(type) {
	case *schema.File:
		contentType = b.ContentType
		contentBody = b.Body
	default:
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		contentType = mediaTypeJSON
		contentBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, url.String(), contentBody)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", contentType)

	req = req.WithContext(ctx)

	return req, nil
}

// NewV2PERequest ...
func (c *Client) NewV2PERequest(ctx context.Context, method string, clusterUUID string, path string, body io.Reader) (*http.Request, error) {
	cluster, err := c.Cluster.GetByUUID(ctx, clusterUUID)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(defaultV2BaseURL + path)
	if err != nil {
		return nil, err
	}

	urlEndpoint, _ := url.Parse(fmt.Sprintf("%s://%s:%s", c.baseURL.Scheme, cluster.Spec.Resources.Network.ExternalIP, c.baseURL.Port()))

	url := urlEndpoint.ResolveReference(rel)
	return c.newV2Request(ctx, method, url, body)
}

// NewV2PCRequest ...
func (c *Client) NewV2PCRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	rel, err := url.Parse(defaultV2BaseURL + path)
	if err != nil {
		return nil, err
	}
	url := c.baseURL.ResolveReference(rel)
	return c.newV2Request(ctx, method, url, body)
}

// NewV2PERequest ...
func (c *Client) newV2Request(ctx context.Context, method string, url *url.URL, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	req.Header.Set("Content-Type", mediaTypeJSON)
	req.Header.Set("Accept", mediaTypeJSON)

	req = req.WithContext(ctx)

	return req, nil
}

func fillStruct(data map[string]interface{}, result interface{}) error {
	j, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(j, result)
}

func (c *Client) listHelper(ctx context.Context, path string, opts *schema.DSMetadata, i interface{}) error {
	err := c.requestHelper(ctx, path, http.MethodPost, opts, i)
	if err != nil {
		return err
	}
	switch v := i.(type) {
	case *schema.VMRecoveryPointListIntent:
		newList := new(schema.VMRecoveryPointListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.VMListIntent:
		newList := new(schema.VMListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.ImageListIntent:
		newList := new(schema.ImageListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.CategoryKeyList:
		newList := new(schema.CategoryKeyList)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.ProjectListIntent:
		newList := new(schema.ProjectListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.ClusterListIntent:
		newList := new(schema.ClusterListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.SubnetListIntent:
		newList := new(schema.SubnetListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.AvailabilityZoneListIntent:
		newList := new(schema.AvailabilityZoneListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}
	case *schema.VirtualNetworkListIntent:
		newList := new(schema.VirtualNetworkListIntent)
		totalEntities := v.Metadata.TotalMatches
		offset := v.Metadata.Offset
		remaining := totalEntities
		if totalEntities > itemsPerPage {
			for hasNext(&remaining) {
				opts.Offset = &offset
				err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
				if err != nil {
					return err
				}
				v.Entities = append(v.Entities, newList.Entities...)
				offset += itemsPerPage
			}
		}

	case *schema.TaskListIntent:
		return nil
		// No Pageination for now
		/*
			newList := new(schema.TaskListIntent)
			totalEntities := v.Metadata.TotalMatches
			offset := v.Metadata.Offset
			remaining := totalEntities
			 		if totalEntities > itemsPerPage {
				for hasNext(&remaining) {
					opts.Offset = &offset
					err := c.requestHelper(ctx, path, http.MethodPost, opts, newList)
					if err != nil {
						return err
					}
					v.Entities = append(v.Entities, newList.Entities...)
					offset += itemsPerPage
				}
			} */
	default:
		return fmt.Errorf("type not supported %v", reflect.ValueOf(v).Elem().Type())
	}
	return nil
}

func hasNext(ri *int64) bool {
	*ri -= itemsPerPage
	return *ri >= (0 - itemsPerPage)
}

func (c *Client) requestHelper(ctx context.Context, path, method string, request interface{}, output interface{}) error {
	req, err := c.NewV3PCRequest(ctx, method, path, request)
	if err != nil {
		return err
	}

	err = c.Do(req, &output)
	if err != nil {
		return err
	}

	return nil
}
