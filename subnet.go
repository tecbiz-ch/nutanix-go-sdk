package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/internal/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	subnetBasePath   = "/subnets"
	subnetListPath   = subnetBasePath + "/list"
	subnetSinglePath = subnetBasePath + "/%s"
)

// SubnetClient is a client for the subnet API.
type SubnetClient struct {
	client *Client
}

// Get retrieves an image by its ID if the input can be parsed as an integer, otherwise it
// retrieves an image by its name. If the image does not exist, nil is returned.
func (c *SubnetClient) Get(ctx context.Context, idOrName string) (*schema.SubnetIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an image by its UUID. If the image does not exist, nil is returned.
func (c *SubnetClient) GetByUUID(ctx context.Context, uuid string) (*schema.SubnetIntent, error) {
	response := new(schema.SubnetIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(subnetSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an subnet by its name. If the image does not exist, nil is returned.
func (c *SubnetClient) GetByName(ctx context.Context, name string) (*schema.SubnetIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("subnet not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of subnets for a specific page.
func (c *SubnetClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.SubnetListIntent, error) {
	response := new(schema.SubnetListIntent)
	err := c.client.listHelper(ctx, subnetListPath, opts, response)
	return response, err
}

// All returns all images.
func (c *SubnetClient) All(ctx context.Context) (*schema.SubnetListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Update a project
func (c *SubnetClient) Update(ctx context.Context, updateRequest *schema.SubnetIntent) (*schema.SubnetIntent, error) {
	updateRequest.Status = nil
	response := new(schema.SubnetIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(subnetSinglePath, updateRequest.Metadata.UUID), http.MethodPut, updateRequest, response)
	return response, err
}

// Create creates a subnet
func (c *SubnetClient) Create(ctx context.Context, createRequest *schema.SubnetIntent) (*schema.SubnetIntent, error) {
	response := new(schema.SubnetIntent)
	err := c.client.requestHelper(ctx, subnetBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Delete deletes a Subnet
func (c *SubnetClient) Delete(ctx context.Context, s *schema.SubnetIntent) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(subnetSinglePath, s.Metadata.UUID), http.MethodDelete, nil, nil)
}
