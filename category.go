package nutanix

import (
	"context"
	"fmt"

	"github.com/tecbiz-ch/nutanix-go-sdk/internal/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	categoryBasePath     = "/categories"
	categoryListPath     = categoryBasePath + "/list"
	categorySinglePath   = categoryBasePath + "/%s"
	categoryNameListPath = categorySinglePath + "/list"
)

// CategoryClient is a client for the image API.
type CategoryClient struct {
	client *Client
}

// Get retrieves an image by its ID if the input can be parsed as an integer, otherwise it
// retrieves an image by its name. If the image does not exist, nil is returned.
func (c *CategoryClient) Get(ctx context.Context, idOrName string) (*schema.CategoryKeyStatus, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an image by its UUID. If the image does not exist, nil is returned.
func (c *CategoryClient) GetByUUID(ctx context.Context, uuid string) (*schema.CategoryKeyStatus, error) {
	response := new(schema.CategoryKeyStatus)
	err := c.client.requestHelper(ctx, fmt.Sprintf(categorySinglePath, uuid), "GET", nil, response)
	return response, err
}

// GetByName retrieves an image by its name. If the image does not exist, nil is returned.
func (c *CategoryClient) GetByName(ctx context.Context, name string) (*schema.CategoryKeyStatus, error) {
	categories, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(categories.Entities) == 0 {
		return nil, fmt.Errorf("category not found: %s", name)
	}
	return categories.Entities[0], err
}

// List returns a list of images for a specific page.
func (c *CategoryClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.CategoryKeyList, error) {
	response := new(schema.CategoryKeyList)
	err := c.client.listHelper(ctx, categoryListPath, opts, response)
	return response, err
}

// ListValues returns a list of images for a specific page.
func (c *CategoryClient) ListValues(ctx context.Context, name string) (*schema.CategoryValueList, error) {
	response := new(schema.CategoryValueList)
	err := c.client.requestHelper(ctx, fmt.Sprintf(categoryNameListPath, name), "POST", &schema.DSMetadata{}, response)
	return response, err
}

// Create creates a CategoryKeyStatus
func (c *CategoryClient) Create(ctx context.Context, createRequest *schema.CategoryKey) (*schema.CategoryKeyStatus, error) {
	response := new(schema.CategoryKeyStatus)
	err := c.client.requestHelper(ctx, fmt.Sprintf(categorySinglePath, createRequest.Name), "PUT", createRequest, response)
	return response, err
}

// All returns all CategoryKeyStatus.
func (c *CategoryClient) All(ctx context.Context) (*schema.CategoryKeyList, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Delete deletes a CategoryKeyStatus.
func (c *CategoryClient) Delete(ctx context.Context, name string) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(categorySinglePath, name), "DELETE", nil, nil)
}
