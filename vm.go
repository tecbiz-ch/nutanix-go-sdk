package nutanix

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/tecbiz-ch/nutanix-go-sdk/internal/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
	v2 "github.com/tecbiz-ch/nutanix-go-sdk/schema/v2"
)

const (
	vmBasePath       = "/vms"
	vmListPath       = vmBasePath + "/list"
	vmSinglePath     = vmBasePath + "/%s"
	vmClonePath      = vmSinglePath + "/clone"
	vmRevertPath     = vmSinglePath + "/revert"
	vmPowerStatePath = vmSinglePath + "/set_power_state"
	vmSnapshotPath   = vmSinglePath + "/snapshot"
)

type MetaData struct {
	SSHAuthorizedKeyMap map[string]string `json:"public_keys,omitempty"`
	Hostname            string            `json:"hostname"`
	UUID                string            `json:"uuid"`
	AvailabilityZone    string            `json:"availability_zone,omitempty"`
	Project             string            `json:"project_id,omitempty"`
}

func (m *MetaData) ToBase64() (string, error) {
	if m.UUID == "" {
		uuid, _ := uuid.NewRandom()
		m.UUID = uuid.String()
	}
	j, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(j), nil
}

// VMClient is a client for the vm API.
type VMClient struct {
	client *Client
}

// VMRevertRequest ...
type VMRevertRequest struct {
	VMRecoveryPointUUID *string `json:"vm_recovery_point_uuid"`
}

// Get ...
func (c *VMClient) Get(ctx context.Context, idOrName string) (*schema.VMIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an vm by its UUID. If the vm does not exist, nil is returned.
func (c *VMClient) GetByUUID(ctx context.Context, uuid string) (*schema.VMIntent, error) {
	response := new(schema.VMIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(vmSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an vm by its name. If the vm does not exist, nil is returned.
func (c *VMClient) GetByName(ctx context.Context, name string) (*schema.VMIntent, error) {
	vms, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("vm_name==%s", name)})
	if len(vms.Entities) == 0 {
		return nil, fmt.Errorf("VM not found: %s", name)
	}
	return vms.Entities[0], err
}

// List returns a list of vms for a specific page.
func (c *VMClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.VMListIntent, error) {
	response := new(schema.VMListIntent)
	err := c.client.listHelper(ctx, vmListPath, opts, response)
	return response, err
}

// All returns all vms
func (c *VMClient) All(ctx context.Context) (*schema.VMListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Create creates a vm
func (c *VMClient) Create(ctx context.Context, createRequest *schema.VMIntent) (*schema.VMIntent, error) {
	response := new(schema.VMIntent)
	err := c.client.requestHelper(ctx, vmBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a vm
func (c *VMClient) Update(ctx context.Context, updateRequest *schema.VMIntent) (*schema.VMIntent, error) {
	updateRequest.Status = nil
	response := new(schema.VMIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(vmSinglePath, updateRequest.Metadata.UUID), http.MethodPut, updateRequest, response)
	return response, err
}

// Delete deletes a vm
func (c *VMClient) Delete(ctx context.Context, s *schema.VMIntent) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(vmSinglePath, s.Metadata.UUID), http.MethodDelete, nil, nil)
}

// RevertToRecoveryPoint ...
func (c *VMClient) RevertToRecoveryPoint(ctx context.Context, vm *schema.VMIntent, vmRevertRequest *VMRevertRequest) (*v2.Task, error) {
	reqBodyData, err := json.Marshal(&vmRevertRequest)
	if err != nil {
		return nil, err
	}

	req, err := c.client.NewV3PERequest(ctx, http.MethodPost, vm.Spec.ClusterReference.UUID, fmt.Sprintf(vmRevertPath, vm.Metadata.UUID), bytes.NewReader(reqBodyData))

	if err != nil {
		return nil, err
	}
	task := new(v2.Task)
	err = c.client.Do(req, &task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// CreateRecoveryPoint ...
func (c *VMClient) CreateRecoveryPoint(ctx context.Context, vm *schema.VMIntent) (*schema.ExecutionContext, error) {
	// Returns a Task UUID
	response := new(schema.ExecutionContext)
	err := c.client.requestHelper(ctx, fmt.Sprintf(vmSnapshotPath, vm.Metadata.UUID), http.MethodPost, bytes.NewReader([]byte("{}")), response)
	return response, err
}

// CreateV3Snapshot ...
func (c *VMClient) CreateV3Snapshot(ctx context.Context) (*schema.ExecutionContext, error) {
	p := &schema.VMIntent{
		Metadata: &schema.Metadata{
			Kind: "vm_snapshot",
		},
	}

	reqBodyData, err := json.Marshal(&p)
	if err != nil {
		return nil, err
	}

	req, err := c.client.NewV3PCRequest(
		ctx,
		http.MethodPost,
		"/vm_snapshots",
		bytes.NewReader(reqBodyData),
	)
	if err != nil {
		return nil, err
	}

	task := new(schema.ExecutionContext)
	err = c.client.Do(req, &task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// SetPowerState ...
func (c *VMClient) SetPowerState(ctx context.Context, powerState v2.PowerState, vm *schema.VMIntent) (*v2.Task, error) {
	path := fmt.Sprintf(vmPowerStatePath, vm.Metadata.UUID)
	powerStateSpec := &v2.VMPowerStateCreate{
		Transition: &powerState,
	}

	reqBodyData, err := json.Marshal(powerStateSpec)
	if err != nil {
		return nil, err
	}

	req, err := c.client.NewV2PERequest(ctx, http.MethodPost, vm.Spec.ClusterReference.UUID, path, bytes.NewReader(reqBodyData))
	if err != nil {
		return nil, err
	}

	response := new(v2.Task)

	err = c.client.Do(req, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *VMClient) Clone(ctx context.Context, sourcevm *schema.VMIntent) (*v2.Task, error) {

	// TODO:
	// POST to https://<prism>:9440/api/nutanix/v3/idempotence_identifiers to get a NEW UUID
	// POST Body IdempotenceIdentifiersInput
	// Response is a IdempotenceIdentifiersResponse
	// get first UUID in Array UUIDList
	// POST NEW VMCLoneInput with UUID retrieved

	req, err := c.client.NewV3PCRequest(ctx, http.MethodPost, fmt.Sprintf(vmClonePath, sourcevm.Metadata.UUID), bytes.NewReader([]byte("{}")))

	if err != nil {
		return nil, err
	}
	task := new(v2.Task)
	err = c.client.Do(req, &task)
	if err != nil {
		return nil, err
	}
	return task, nil
}
