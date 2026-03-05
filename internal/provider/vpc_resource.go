package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-cloudru-community/internal/client"
)

var _ resource.Resource = &CloudRuVpcResource{}
var _ resource.ResourceWithImportState = &CloudRuVpcResource{}

func NewCloudRuVpcResource() resource.Resource {
	return &CloudRuVpcResource{}
}

type CloudRuVpcResource struct {
	client *client.CloudRuHttpClient
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
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.CloudRuHttpClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.CloudRuHttpClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *CloudRuVpcResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VpcResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"projectId":   r.client.ProjectID,
		"name":        data.Name.ValueString(),
		"description": data.Description.ValueString(),
	}

	var op struct {
		Id         string `json:"id"`
		Done       bool   `json:"done"`
		ResourceId string `json:"resourceId"`
	}

	reqURL := r.client.VpcEndpoint + "/v1/vpcs"
	if err := r.client.PostJSON(ctx, reqURL, body, &op); err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	for !op.Done {
		time.Sleep(2 * time.Second)
		pollURL := fmt.Sprintf("%s/v1/vpcs/operations/%s", r.client.VpcEndpoint, op.Id)
		if err := r.client.GetJSON(ctx, pollURL, &op); err != nil {
			resp.Diagnostics.AddError("Operation Poll Error", err.Error())
			return
		}
	}

	data.Id = types.StringValue(op.ResourceId)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudRuVpcResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VpcResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getURL := fmt.Sprintf("%s/v1/vpcs/%s", r.client.VpcEndpoint, data.Id.ValueString())

	var vpcResp struct {
		Id          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	err := r.client.GetJSON(ctx, getURL, &vpcResp)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	data.Id = types.StringValue(vpcResp.Id)
	data.Name = types.StringValue(vpcResp.Name)
	data.Description = types.StringValue(vpcResp.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudRuVpcResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VpcResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"name":        data.Name.ValueString(),
		"description": data.Description.ValueString(),
	}

	putURL := fmt.Sprintf("%s/v1/vpcs/%s", r.client.VpcEndpoint, data.Id.ValueString())
	if err := r.client.PutJSON(ctx, putURL, body, nil); err != nil {
		resp.Diagnostics.AddError("Update Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudRuVpcResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VpcResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	delURL := fmt.Sprintf("%s/v1/vpcs/%s", r.client.VpcEndpoint, data.Id.ValueString())
	if err := r.client.Delete(ctx, delURL); err != nil {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *CloudRuVpcResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
