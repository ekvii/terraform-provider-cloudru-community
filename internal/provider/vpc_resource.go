package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &CloudRuVpcResource{}
var _ resource.ResourceWithImportState = &CloudRuVpcResource{}

func NewCloudRuVpcResource() resource.Resource {
	return &CloudRuVpcResource{}
}

type CloudRuVpcResource struct {
	client *CloudRuProviderClient
}

type VpcResourceModel struct {
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Id          types.String `tfsdk:"id"`
}

func (r *CloudRuVpcResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc"
}

func (r *CloudRuVpcResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "VPC",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "VPC name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "VPC description",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VPC id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *CloudRuVpcResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*CloudRuProviderClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *CloudRuVpcResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VpcResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	body := map[string]interface{}{
		"projectId":   r.client.ProjectID,
		"name":        data.Name.ValueString(),
		"description": data.Description.ValueString(),
	}

	payload, _ := json.Marshal(body)

	reqURL := r.client.VpcEndpoint + "/v1/vpcs"
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	httpReq.Header.Set("Authorization", "Bearer "+r.client.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := r.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}
	defer httpResp.Body.Close()

	var op struct {
		Id       string          `json:"id"`
		Done     bool            `json:"done"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&op); err != nil {
		resp.Diagnostics.AddError("Create Decode Error", err.Error())
		return
	}

	for !op.Done {
		time.Sleep(2 * time.Second)
		pollURL := fmt.Sprintf("%s/v1/vpcs/operations/%s", r.client.VpcEndpoint, op.Id)
		pollReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		pollReq.Header.Set("Authorization", "Bearer "+r.client.Token)

		pollResp, err := r.client.HTTPClient.Do(pollReq)
		if err != nil {
			resp.Diagnostics.AddError("Operation Poll Error", err.Error())
			return
		}
		json.NewDecoder(pollResp.Body).Decode(&op)
		pollResp.Body.Close()
	}

	var vpc struct {
		Id string `json:"id"`
	}
	json.Unmarshal(op.Response, &vpc)
	data.Id = types.StringValue(vpc.Id)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudRuVpcResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VpcResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	getURL := fmt.Sprintf("%s/v1/vpcs/%s", r.client.VpcEndpoint, data.Id.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	httpReq.Header.Set("Authorization", "Bearer "+r.client.Token)

	httpResp, err := r.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	var vpcResp struct {
		Id          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(httpResp.Body).Decode(&vpcResp)

	data.Name = types.StringValue(vpcResp.Name)
	data.Description = types.StringValue(vpcResp.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudRuVpcResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VpcResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"name":        data.Name.ValueString(),
		"description": data.Description.ValueString(),
	}
	payload, _ := json.Marshal(body)

	putURL := fmt.Sprintf("%s/v1/vpcs/%s", r.client.VpcEndpoint, data.Id.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(payload))
	httpReq.Header.Set("Authorization", "Bearer "+r.client.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	_, err := r.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Update Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudRuVpcResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VpcResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	delURL := fmt.Sprintf("%s/v1/vpcs/%s", r.client.VpcEndpoint, data.Id.ValueString())
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
	httpReq.Header.Set("Authorization", "Bearer "+r.client.Token)

	_, err := r.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *CloudRuVpcResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
