package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-cloudru-community/internal/client"
)

var _ resource.Resource = &SubnetResource{}
var _ resource.ResourceWithImportState = &SubnetResource{}

func NewSubnetResource() resource.Resource {
	return &SubnetResource{}
}

type SubnetResource struct {
	client *client.CloudRuHttpClient
}

// SubnetAvailabilityZoneModel mirrors the availability_zone block.
type SubnetAvailabilityZoneModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// SubnetTagModel mirrors the tags block.
type SubnetTagModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Color types.String `tfsdk:"color"`
}

// SubnetResourceModel is the Terraform state model for the subnet resource.
type SubnetResourceModel struct {
	ID               types.String                  `tfsdk:"id"`
	Name             types.String                  `tfsdk:"name"`
	VpcID            types.String                  `tfsdk:"vpc_id"`
	ProjectID        types.String                  `tfsdk:"project_id"`
	SubnetAddress    types.String                  `tfsdk:"subnet_address"`
	DefaultGateway   types.String                  `tfsdk:"default_gateway"`
	Description      types.String                  `tfsdk:"description"`
	RoutedNetwork    types.Bool                    `tfsdk:"routed_network"`
	Default          types.Bool                    `tfsdk:"default"`
	DNSServers       types.List                    `tfsdk:"dns_servers"`
	AvailabilityZone []SubnetAvailabilityZoneModel `tfsdk:"availability_zone"`
	Tags             []SubnetTagModel              `tfsdk:"tags"`
	CreatedTime      types.String                  `tfsdk:"created_time"`
	ModifiedTime     types.String                  `tfsdk:"modified_time"`
	State            types.String                  `tfsdk:"state"`
	Type             types.String                  `tfsdk:"type"`
	CanDelete        types.Bool                    `tfsdk:"can_delete"`
	InterfaceCount   types.Int64                   `tfsdk:"interface_count"`
}

func (r *SubnetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subnet"
}

func (r *SubnetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cloud.ru subnet (managed via Compute API).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subnet ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Subnet name (1–64 chars, must start with a letter).",
			},
			"vpc_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the VPC this subnet belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Project ID. Defaults to the provider project_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnet_address": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "CIDR address of the subnet (e.g. 192.168.0.0/24).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_gateway": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Gateway IP address within the subnet.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Subnet description (0–255 chars).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"routed_network": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "If true (default), the subnet is routed; if false, it is isolated.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "If true, this subnet is the default subnet in its availability zone.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"dns_servers": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of DNS server IP addresses.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			// Computed-only attributes populated from the API response.
			"created_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"modified_time": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last modification timestamp (RFC 3339).",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current subnet state (e.g. created, creating, deleting).",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subnet type (e.g. regular, vpc, technical).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"can_delete": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the subnet can be deleted.",
			},
			"interface_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of network interfaces attached to this subnet.",
			},
		},
		Blocks: map[string]schema.Block{
			"availability_zone": schema.ListNestedBlock{
				MarkdownDescription: "Availability zone where the subnet is placed.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Availability zone ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Availability zone name.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"tags": schema.ListNestedBlock{
				MarkdownDescription: "Tags attached to this subnet.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Tag ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Tag name.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"color": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Tag color.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
		},
	}
}

func (r *SubnetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// apiSubnetResponse mirrors PublicSubnetResponse / PublicSubnetListItemResponse.
type apiSubnetResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	SubnetAddress    string   `json:"subnet_address"`
	RoutedNetwork    bool     `json:"routed_network"`
	DefaultGateway   string   `json:"default_gateway"`
	DNSServers       []string `json:"dns_servers"`
	AvailabilityZone struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"availability_zone"`
	Tags []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"tags"`
	Default        bool   `json:"default"`
	State          string `json:"state"`
	CreatedTime    string `json:"created_time"`
	ModifiedTime   string `json:"modified_time"`
	Type           string `json:"type"`
	CanDelete      bool   `json:"can_delete"`
	VpcID          string `json:"vpc_id"`
	InterfaceCount int64  `json:"interface_count"`
}

// flattenSubnetResponse copies an API response into the Terraform state model.
func flattenSubnetResponse(api *apiSubnetResponse, data *SubnetResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.VpcID = types.StringValue(api.VpcID)
	data.SubnetAddress = types.StringValue(api.SubnetAddress)
	data.DefaultGateway = types.StringValue(api.DefaultGateway)
	data.Description = types.StringValue(api.Description)
	data.RoutedNetwork = types.BoolValue(api.RoutedNetwork)
	data.Default = types.BoolValue(api.Default)
	data.State = types.StringValue(api.State)
	data.CreatedTime = types.StringValue(api.CreatedTime)
	data.ModifiedTime = types.StringValue(api.ModifiedTime)
	data.Type = types.StringValue(api.Type)
	data.CanDelete = types.BoolValue(api.CanDelete)
	data.InterfaceCount = types.Int64Value(api.InterfaceCount)

	// dns_servers
	listVal, diags := types.ListValueFrom(context.Background(), types.StringType, api.DNSServers)
	_ = diags
	data.DNSServers = listVal

	// availability_zone
	data.AvailabilityZone = []SubnetAvailabilityZoneModel{
		{
			ID:   types.StringValue(api.AvailabilityZone.ID),
			Name: types.StringValue(api.AvailabilityZone.Name),
		},
	}

	// tags
	tags := make([]SubnetTagModel, len(api.Tags))
	for i, t := range api.Tags {
		tags[i] = SubnetTagModel{
			ID:    types.StringValue(t.ID),
			Name:  types.StringValue(t.Name),
			Color: types.StringValue(t.Color),
		}
	}
	data.Tags = tags
}

func (r *SubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SubnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID
	}

	body := map[string]interface{}{
		"name":            data.Name.ValueString(),
		"vpc_id":          data.VpcID.ValueString(),
		"project_id":      projectID,
		"subnet_address":  data.SubnetAddress.ValueString(),
		"default_gateway": data.DefaultGateway.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		body["description"] = data.Description.ValueString()
	}
	if !data.RoutedNetwork.IsNull() && !data.RoutedNetwork.IsUnknown() {
		body["routed_network"] = data.RoutedNetwork.ValueBool()
	}
	if !data.Default.IsNull() && !data.Default.IsUnknown() {
		body["default"] = data.Default.ValueBool()
	}
	if !data.DNSServers.IsNull() && !data.DNSServers.IsUnknown() {
		var dns []string
		resp.Diagnostics.Append(data.DNSServers.ElementsAs(ctx, &dns, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body["dns_servers"] = dns
	}

	if len(data.AvailabilityZone) > 0 {
		body["availability_zone_id"] = data.AvailabilityZone[0].ID.ValueString()
	}

	if len(data.Tags) > 0 {
		tagIDs := make([]string, len(data.Tags))
		for i, t := range data.Tags {
			tagIDs[i] = t.ID.ValueString()
		}
		body["tag_ids"] = tagIDs
	}

	var apiResp apiSubnetResponse
	createURL := r.client.ComputeEndpoint + "/api/v1/subnets"
	if err := r.client.PostJSONCreated(ctx, createURL, body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Create Subnet Error", err.Error())
		return
	}

	// The Compute API creates subnets asynchronously. Poll GET until state is
	// "created" or a terminal error state is reached.
	getURL := fmt.Sprintf("%s/api/v1/subnets/%s", r.client.ComputeEndpoint, apiResp.ID)
	for apiResp.State != "created" {
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Create Subnet Timeout", ctx.Err().Error())
			return
		case <-time.After(2 * time.Second):
		}
		if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
			resp.Diagnostics.AddError("Create Subnet Poll Error", err.Error())
			return
		}
		if apiResp.State == "error_creating" || apiResp.State == "error" {
			resp.Diagnostics.AddError("Create Subnet Error", fmt.Sprintf("subnet entered %s state", apiResp.State))
			return
		}
	}

	flattenSubnetResponse(&apiResp, &data)
	data.ProjectID = types.StringValue(projectID)

	// Fetch interface_count from list endpoint (not returned by create/get-by-id).
	ifCount, err := r.fetchInterfaceCount(ctx, apiResp.ID, projectID, apiResp.VpcID)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Could not retrieve interface_count",
			fmt.Sprintf("GET /api/v1/subnets list call failed: %s. The interface_count attribute may be stale.", err.Error()),
		)
	} else {
		data.InterfaceCount = types.Int64Value(ifCount)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getURL := fmt.Sprintf("%s/api/v1/subnets/%s", r.client.ComputeEndpoint, data.ID.ValueString())

	var apiResp apiSubnetResponse
	if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Subnet Error", err.Error())
		return
	}

	flattenSubnetResponse(&apiResp, &data)

	// interface_count is only available from the list endpoint, not get-by-id.
	// Fetch it separately so the state stays accurate.
	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID
	}
	ifCount, err := r.fetchInterfaceCount(ctx, data.ID.ValueString(), projectID, apiResp.VpcID)
	if err != nil {
		// Non-fatal: surface as a warning and keep the previously stored value.
		resp.Diagnostics.AddWarning(
			"Could not retrieve interface_count",
			fmt.Sprintf("GET /api/v1/subnets list call failed: %s. The interface_count attribute may be stale.", err.Error()),
		)
	} else {
		data.InterfaceCount = types.Int64Value(ifCount)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// apiSubnetListResponse mirrors PaginationResponse[PublicSubnetListItemResponse].
type apiSubnetListResponse struct {
	Items  []apiSubnetResponse `json:"items"`
	Offset int                 `json:"offset"`
	Limit  int                 `json:"limit"`
	Total  int                 `json:"total"`
}

// fetchInterfaceCount calls GET /api/v1/subnets with project_id and vpc_id
// filters and pages through results (offset/limit) until it finds the subnet
// with the given id, then returns its interface_count.
func (r *SubnetResource) fetchInterfaceCount(ctx context.Context, subnetID, projectID, vpcID string) (int64, error) {
	const pageSize = 100
	baseURL := fmt.Sprintf("%s/api/v1/subnets?project_id=%s&vpc_id=%s&limit=%d",
		r.client.ComputeEndpoint, projectID, vpcID, pageSize)

	for offset := 0; ; offset += pageSize {
		listURL := fmt.Sprintf("%s&offset=%d", baseURL, offset)
		var page apiSubnetListResponse
		if err := r.client.GetJSON(ctx, listURL, &page); err != nil {
			return 0, err
		}
		for _, item := range page.Items {
			if item.ID == subnetID {
				return item.InterfaceCount, nil
			}
		}
		// Stop when we have seen all items.
		if offset+len(page.Items) >= page.Total || len(page.Items) == 0 {
			break
		}
	}
	// Subnet not found in the list (unusual, but not an error — could be a
	// race with deletion). Return 0 so the caller can decide what to do.
	return 0, nil
}

func (r *SubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SubnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve ID from state (not present in plan).
	var state SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID = state.ID

	body := map[string]interface{}{
		"name":           data.Name.ValueString(),
		"description":    data.Description.ValueString(),
		"routed_network": data.RoutedNetwork.ValueBool(),
		"default":        data.Default.ValueBool(),
	}

	if !data.DNSServers.IsNull() && !data.DNSServers.IsUnknown() {
		var dns []string
		resp.Diagnostics.Append(data.DNSServers.ElementsAs(ctx, &dns, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body["dns_servers"] = dns
	}

	if len(data.Tags) > 0 {
		tagIDs := make([]string, len(data.Tags))
		for i, t := range data.Tags {
			tagIDs[i] = t.ID.ValueString()
		}
		body["tag_ids"] = tagIDs
	} else {
		body["tag_ids"] = []string{}
	}

	putURL := fmt.Sprintf("%s/api/v1/subnets/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	var apiResp apiSubnetResponse
	if err := r.client.PutJSON(ctx, putURL, body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Update Subnet Error", err.Error())
		return
	}

	flattenSubnetResponse(&apiResp, &data)

	// Refresh interface_count from list endpoint after update.
	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID
	}
	ifCount, err := r.fetchInterfaceCount(ctx, data.ID.ValueString(), projectID, apiResp.VpcID)
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Could not retrieve interface_count",
			fmt.Sprintf("GET /api/v1/subnets list call failed: %s. The interface_count attribute may be stale.", err.Error()),
		)
	} else {
		data.InterfaceCount = types.Int64Value(ifCount)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	delURL := fmt.Sprintf("%s/api/v1/subnets/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	if err := r.client.DeleteNoContent(ctx, delURL); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete Subnet Error", err.Error())
		return
	}

	// The Compute API returns 204 immediately but deletes asynchronously.
	// Poll GET until the subnet is gone (404) or enters error_deleting state.
	getURL := fmt.Sprintf("%s/api/v1/subnets/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	for {
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Delete Subnet Timeout", ctx.Err().Error())
			return
		case <-time.After(2 * time.Second):
		}

		var apiResp apiSubnetResponse
		err := r.client.GetJSON(ctx, getURL, &apiResp)
		if err != nil {
			if client.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError("Delete Subnet Poll Error", err.Error())
			return
		}
		if apiResp.State == "error_deleting" {
			resp.Diagnostics.AddError("Delete Subnet Error", fmt.Sprintf("subnet entered error_deleting state"))
			return
		}
	}
}

func (r *SubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
