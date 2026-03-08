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

var _ resource.Resource = &PlacementGroupResource{}
var _ resource.ResourceWithImportState = &PlacementGroupResource{}

func NewPlacementGroupResource() resource.Resource {
	return &PlacementGroupResource{}
}

type PlacementGroupResource struct {
	client *client.CloudRuHttpClient
}

// PlacementGroupAvailabilityZoneModel mirrors PublicAvailabilityZonePlacementGroupResponse.
type PlacementGroupAvailabilityZoneModel struct {
	AvailabilityZoneID   types.String `tfsdk:"availability_zone_id"`
	AvailabilityZoneName types.String `tfsdk:"availability_zone_name"`
	VmCount              types.Int64  `tfsdk:"vm_count"`
	MaxVmCount           types.Int64  `tfsdk:"max_vm_count"`
	State                types.String `tfsdk:"state"`
}

// PlacementGroupTagModel mirrors TagMiniResponse.
type PlacementGroupTagModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// PlacementGroupResourceModel is the Terraform state model for the placement_group resource.
type PlacementGroupResourceModel struct {
	ID                types.String                          `tfsdk:"id"`
	Name              types.String                          `tfsdk:"name"`
	Description       types.String                          `tfsdk:"description"`
	ProjectID         types.String                          `tfsdk:"project_id"`
	Policy            types.String                          `tfsdk:"policy"`
	CreatedTime       types.String                          `tfsdk:"created_time"`
	ModifiedTime      types.String                          `tfsdk:"modified_time"`
	AvailabilityZones []PlacementGroupAvailabilityZoneModel `tfsdk:"availability_zones"`
	Tags              []PlacementGroupTagModel              `tfsdk:"tags"`
}

func (r *PlacementGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_placement_group"
}

func (r *PlacementGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cloud.ru placement group (managed via Compute API).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Placement group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Placement group name (1–64 chars, must start with a letter).",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Placement group description (0–255 chars).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"policy": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Placement policy: `soft-anti-affinity` or `anti-affinity`. Cannot be changed after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
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
		},
		Blocks: map[string]schema.Block{
			"availability_zones": schema.ListNestedBlock{
				MarkdownDescription: "Availability zones where the placement group is active.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"availability_zone_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Availability zone ID.",
						},
						"availability_zone_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Availability zone name.",
						},
						"vm_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Current number of VMs in this availability zone.",
						},
						"max_vm_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Maximum number of VMs allowed in this availability zone.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "State of the placement group in this availability zone.",
						},
					},
				},
			},
			"tags": schema.ListNestedBlock{
				MarkdownDescription: "Tags attached to this placement group.",
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
					},
				},
			},
		},
	}
}

func (r *PlacementGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// apiPlacementGroupAvailabilityZone mirrors PublicAvailabilityZonePlacementGroupResponse.
type apiPlacementGroupAvailabilityZone struct {
	AvailabilityZoneID   string `json:"availability_zone_id"`
	AvailabilityZoneName string `json:"availability_zone_name"`
	VmCount              int64  `json:"vm_count"`
	MaxVmCount           int64  `json:"max_vm_count"`
	State                string `json:"state"`
}

// apiPlacementGroupTag mirrors TagMiniResponse.
type apiPlacementGroupTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// apiPlacementGroupResponse mirrors PublicPlacementGroupResponse.
type apiPlacementGroupResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Project     struct {
		ID string `json:"id"`
	} `json:"project"`
	Policy            string                              `json:"policy"`
	CreatedTime       string                              `json:"created_time"`
	ModifiedTime      string                              `json:"modified_time"`
	AvailabilityZones []apiPlacementGroupAvailabilityZone `json:"availability_zones"`
	Tags              []apiPlacementGroupTag              `json:"tags"`
}

// flattenPlacementGroupResponse copies an API response into the Terraform state model.
func flattenPlacementGroupResponse(api *apiPlacementGroupResponse, data *PlacementGroupResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	data.ProjectID = types.StringValue(api.Project.ID)
	data.Policy = types.StringValue(api.Policy)
	data.CreatedTime = types.StringValue(api.CreatedTime)
	data.ModifiedTime = types.StringValue(api.ModifiedTime)

	zones := make([]PlacementGroupAvailabilityZoneModel, len(api.AvailabilityZones))
	for i, z := range api.AvailabilityZones {
		zones[i] = PlacementGroupAvailabilityZoneModel{
			AvailabilityZoneID:   types.StringValue(z.AvailabilityZoneID),
			AvailabilityZoneName: types.StringValue(z.AvailabilityZoneName),
			VmCount:              types.Int64Value(z.VmCount),
			MaxVmCount:           types.Int64Value(z.MaxVmCount),
			State:                types.StringValue(z.State),
		}
	}
	data.AvailabilityZones = zones

	tags := make([]PlacementGroupTagModel, len(api.Tags))
	for i, t := range api.Tags {
		tags[i] = PlacementGroupTagModel{
			ID:   types.StringValue(t.ID),
			Name: types.StringValue(t.Name),
		}
	}
	data.Tags = tags
}

func (r *PlacementGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PlacementGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID
	}

	body := map[string]interface{}{
		"project_id": projectID,
		"name":       data.Name.ValueString(),
		"policy":     data.Policy.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		body["description"] = data.Description.ValueString()
	}

	if len(data.Tags) > 0 {
		tagIDs := make([]string, len(data.Tags))
		for i, t := range data.Tags {
			tagIDs[i] = t.ID.ValueString()
		}
		body["tag_ids"] = tagIDs
	}

	var apiResp apiPlacementGroupResponse
	createURL := r.client.ComputeEndpoint + "/api/v1/placement-groups"
	if err := r.client.PostJSONCreated(ctx, createURL, body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Create Placement Group Error", err.Error())
		return
	}

	flattenPlacementGroupResponse(&apiResp, &data)
	data.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlacementGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PlacementGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getURL := fmt.Sprintf("%s/api/v1/placement-groups/%s", r.client.ComputeEndpoint, data.ID.ValueString())

	var apiResp apiPlacementGroupResponse
	if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Placement Group Error", err.Error())
		return
	}

	flattenPlacementGroupResponse(&apiResp, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlacementGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PlacementGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve ID from state (not present in plan).
	var state PlacementGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID = state.ID

	body := map[string]interface{}{
		"name":        data.Name.ValueString(),
		"description": data.Description.ValueString(),
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

	putURL := fmt.Sprintf("%s/api/v1/placement-groups/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	var apiResp apiPlacementGroupResponse
	if err := r.client.PutJSON(ctx, putURL, body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Update Placement Group Error", err.Error())
		return
	}

	flattenPlacementGroupResponse(&apiResp, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PlacementGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PlacementGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	delURL := fmt.Sprintf("%s/api/v1/placement-groups/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	if err := r.client.DeleteNoContent(ctx, delURL); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete Placement Group Error", err.Error())
		return
	}

	// Defensively poll GET until the placement group is gone (404).
	getURL := fmt.Sprintf("%s/api/v1/placement-groups/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	for {
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Delete Placement Group Timeout", ctx.Err().Error())
			return
		case <-time.After(2 * time.Second):
		}

		var apiResp apiPlacementGroupResponse
		err := r.client.GetJSON(ctx, getURL, &apiResp)
		if err != nil {
			if client.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError("Delete Placement Group Poll Error", err.Error())
			return
		}
	}
}

func (r *PlacementGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
