package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-cloudru-community/internal/client"
)

var _ resource.Resource = &DnsServerResource{}
var _ resource.ResourceWithImportState = &DnsServerResource{}

func NewDnsServerResource() resource.Resource {
	return &DnsServerResource{}
}

type DnsServerResource struct {
	client *client.CloudRuHttpClient
}

type DnsServerModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	SubnetId    types.String `tfsdk:"subnet_id"`
	IpAddress   types.String `tfsdk:"ip_address"`
	Description types.String `tfsdk:"description"`
}

func (r *DnsServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_server"
}

func (r *DnsServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cloud.ru DNS Server",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name":        schema.StringAttribute{Required: true},
			"subnet_id":   schema.StringAttribute{Required: true},
			"ip_address":  schema.StringAttribute{Optional: true},
			"description": schema.StringAttribute{Optional: true},
		},
	}
}

func (r *DnsServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DnsServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DnsServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"name":     data.Name.ValueString(),
		"subnetId": data.SubnetId.ValueString(),
	}
	if !data.IpAddress.IsNull() {
		body["ipAddress"] = data.IpAddress.ValueString()
	}
	if !data.Description.IsNull() {
		body["description"] = data.Description.ValueString()
	}

	var op struct {
		Id string `json:"id"`
	}

	url := r.client.DnsEndpoint + "/v1/dnsServers"
	if err := r.client.PostJSON(ctx, url, body, &op); err != nil {
		resp.Diagnostics.AddError("Create Error", err.Error())
		return
	}

	data.Id = types.StringValue(op.Id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DnsServerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/v1/dnsServers/%s", r.client.DnsEndpoint, data.Id.ValueString())

	var server struct {
		Id          string `json:"id"`
		Name        string `json:"name"`
		SubnetId    string `json:"subnetId"`
		IpAddress   string `json:"ipAddress"`
		Description string `json:"description"`
	}

	err := r.client.GetJSON(ctx, url, &server)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Error", err.Error())
		return
	}

	data.Name = types.StringValue(server.Name)
	data.SubnetId = types.StringValue(server.SubnetId)
	data.IpAddress = types.StringValue(server.IpAddress)
	data.Description = types.StringValue(server.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DnsServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"name": data.Name.ValueString(),
	}

	url := fmt.Sprintf("%s/v1/dnsServers/%s", r.client.DnsEndpoint, data.Id.ValueString())
	if err := r.client.PutJSON(ctx, url, body, nil); err != nil {
		resp.Diagnostics.AddError("Update Error", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DnsServerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/v1/dnsServers/%s", r.client.DnsEndpoint, data.Id.ValueString())
	if err := r.client.Delete(ctx, url); err != nil {
		resp.Diagnostics.AddError("Delete Error", err.Error())
		return
	}
}

func (r *DnsServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
