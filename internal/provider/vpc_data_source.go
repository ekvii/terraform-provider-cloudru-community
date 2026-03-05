package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-cloudru-community/internal/client"
)

var _ datasource.DataSource = &VpcsDataSource{}

func NewVpcsDataSource() datasource.DataSource {
	return &VpcsDataSource{}
}

type VpcsDataSource struct {
	client *client.CloudRuHttpClient
}

type VpcsDataSourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Vpcs      types.List   `tfsdk:"vpcs"`
}

type VpcModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	ProjectID         types.String `tfsdk:"project_id"`
	CustomerID        types.String `tfsdk:"customer_id"`
	ProductInstanceID types.String `tfsdk:"product_instance_id"`
	Type              types.String `tfsdk:"type"`
	Default           types.Bool   `tfsdk:"default"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

var vpcObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":                  types.StringType,
		"name":                types.StringType,
		"description":         types.StringType,
		"project_id":          types.StringType,
		"customer_id":         types.StringType,
		"product_instance_id": types.StringType,
		"type":                types.StringType,
		"default":             types.BoolType,
		"created_at":          types.StringType,
		"updated_at":          types.StringType,
	},
}

func (d *VpcsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpcs"
}

func (d *VpcsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns the list of VPCs available in the configured project.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Project ID to list VPCs for. Defaults to the provider project_id.",
				Optional:            true,
				Computed:            true,
			},
			"vpcs": schema.ListNestedAttribute{
				MarkdownDescription: "List of VPCs.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Unique VPC identifier.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "VPC name.",
							Computed:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "VPC description.",
							Computed:            true,
						},
						"project_id": schema.StringAttribute{
							MarkdownDescription: "Project ID the VPC belongs to.",
							Computed:            true,
						},
						"customer_id": schema.StringAttribute{
							MarkdownDescription: "Customer (owner) identifier.",
							Computed:            true,
						},
						"product_instance_id": schema.StringAttribute{
							MarkdownDescription: "Product instance identifier.",
							Computed:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "VPC type (VPC_TYPE_CLIENT or VPC_TYPE_SERVICE).",
							Computed:            true,
						},
						"default": schema.BoolAttribute{
							MarkdownDescription: "Whether this is the default VPC.",
							Computed:            true,
						},
						"created_at": schema.StringAttribute{
							MarkdownDescription: "Creation timestamp (RFC3339).",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "Last update timestamp (RFC3339).",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *VpcsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.CloudRuHttpClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected *client.CloudRuHttpClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = c
}

// apiVpc is the JSON shape returned by the VPC listing API.
type apiVpc struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	ProjectID         string `json:"projectId"`
	CustomerID        string `json:"customerId"`
	ProductInstanceID string `json:"productInstanceId"`
	Type              string `json:"type"`
	Default           bool   `json:"default"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

// listVPCResponse implements client.PagedResponse[apiVpc].
type listVPCResponse struct {
	Vpcs          []apiVpc `json:"vpcs"`
	NextPageToken string   `json:"nextPageToken"`
}

func (r *listVPCResponse) Items() []apiVpc   { return r.Vpcs }
func (r *listVPCResponse) NextToken() string { return r.NextPageToken }

func (d *VpcsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VpcsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = d.client.ProjectID
	}

	baseURL := fmt.Sprintf("%s/v1/vpcs?projectId=%s", d.client.VpcEndpoint, projectID)

	allVpcs, err := client.GetPaged(
		ctx,
		d.client,
		baseURL,
		func() *listVPCResponse { return &listVPCResponse{} },
		func(v apiVpc) string { return v.ID },
	)
	if err != nil {
		resp.Diagnostics.AddError("VPCs Read Error", err.Error())
		return
	}

	vpcObjects := make([]attr.Value, 0, len(allVpcs))
	for _, v := range allVpcs {
		obj, diags := types.ObjectValue(vpcObjectType.AttrTypes, map[string]attr.Value{
			"id":                  types.StringValue(v.ID),
			"name":                types.StringValue(v.Name),
			"description":         types.StringValue(v.Description),
			"project_id":          types.StringValue(v.ProjectID),
			"customer_id":         types.StringValue(v.CustomerID),
			"product_instance_id": types.StringValue(v.ProductInstanceID),
			"type":                types.StringValue(v.Type),
			"default":             types.BoolValue(v.Default),
			"created_at":          types.StringValue(v.CreatedAt),
			"updated_at":          types.StringValue(v.UpdatedAt),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		vpcObjects = append(vpcObjects, obj)
	}

	vpcList, diags := types.ListValue(vpcObjectType, vpcObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ProjectID = types.StringValue(projectID)
	data.Vpcs = vpcList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
