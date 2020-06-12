package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/internal/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	projectBasePath   = "/projects"
	projectListPath   = projectBasePath + "/list"
	projectSinglePath = projectBasePath + "/%s"
)

// ProjectClient is a client for the project API.
type ProjectClient struct {
	client *Client
}

// Get retrieves an project by its ID if the input can be parsed as an integer, otherwise it
// retrieves an image by its name. If the image does not exist, nil is returned.
func (c *ProjectClient) Get(ctx context.Context, idOrName string) (*schema.ProjectIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an image by its UUID. If the image does not exist, nil is returned.
func (c *ProjectClient) GetByUUID(ctx context.Context, uuid string) (*schema.ProjectIntent, error) {
	response := new(schema.ProjectIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(projectSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an project by its name. If the project does not exist, nil is returned.
func (c *ProjectClient) GetByName(ctx context.Context, name string) (*schema.ProjectIntent, error) {
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	return list.Entities[0], err
}

// List returns a list of projects for a specific page.
func (c *ProjectClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.ProjectListIntent, error) {
	response := new(schema.ProjectListIntent)
	err := c.client.listHelper(ctx, projectListPath, opts, response)
	return response, err
}

// All returns all images.
func (c *ProjectClient) All(ctx context.Context) (*schema.ProjectListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a project
func (c *ProjectClient) Create(ctx context.Context, createRequest *schema.ProjectIntent) (*schema.ProjectIntent, error) {
	response := new(schema.ProjectIntent)
	err := c.client.requestHelper(ctx, projectBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a project
func (c *ProjectClient) Update(ctx context.Context, project *schema.ProjectIntent) (*schema.ProjectIntent, error) {
	project.Status = nil
	response := new(schema.ProjectIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(projectSinglePath, project.Metadata.UUID), http.MethodPut, project, response)
	return response, err
}

// Delete deletes a Project
func (c *ProjectClient) Delete(ctx context.Context, s *schema.ProjectIntent) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(projectSinglePath, s.Metadata.UUID), http.MethodDelete, nil, nil)
}
