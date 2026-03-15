package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-cloudru-community/internal/client"
)

var _ resource.Resource = &FreeTierVmResource{}
var _ resource.ResourceWithImportState = &FreeTierVmResource{}

func NewFreeTierVmResource() resource.Resource {
	return &FreeTierVmResource{}
}

// FreeTierVmResource manages Cloud.ru free-tier virtual machines via
// POST /api/v1/free-tier. The flavor, boot disk size and type are chosen
// automatically by the platform — the user only provides a name, an image
// and optional metadata / networking preferences.
type FreeTierVmResource struct {
	client *client.CloudRuHttpClient
}

// ─── Terraform state models ──────────────────────────────────────────────────

type FreeTierVmImageModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	HostName        types.String `tfsdk:"host_name"`
	UserName        types.String `tfsdk:"user_name"`
	PublicKey       types.String `tfsdk:"public_key"`
	Password        types.String `tfsdk:"password"`
	FreeTierEnabled types.Bool   `tfsdk:"free_tier_enabled"`
}

type FreeTierVmFlavorModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	CPU              types.Int64  `tfsdk:"cpu"`
	RAM              types.Int64  `tfsdk:"ram"`
	GPU              types.Int64  `tfsdk:"gpu"`
	Oversubscription types.String `tfsdk:"oversubscription"`
}

type FreeTierVmDiskTypeModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type FreeTierVmDiskModel struct {
	ID       types.String            `tfsdk:"id"`
	Name     types.String            `tfsdk:"name"`
	Primary  types.Bool              `tfsdk:"primary"`
	Size     types.Int64             `tfsdk:"size"`
	State    types.String            `tfsdk:"state"`
	DiskType FreeTierVmDiskTypeModel `tfsdk:"disk_type"`
}

type FreeTierVmSecurityGroupModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	State types.String `tfsdk:"state"`
}

type FreeTierVmFloatingIPModel struct {
	ID        types.String `tfsdk:"id"`
	IPAddress types.String `tfsdk:"ip_address"`
	Name      types.String `tfsdk:"name"`
	State     types.String `tfsdk:"state"`
}

type FreeTierVmInterfaceModel struct {
	ID                       types.String                   `tfsdk:"id"`
	Name                     types.String                   `tfsdk:"name"`
	IPAddress                types.String                   `tfsdk:"ip_address"`
	Primary                  types.Bool                     `tfsdk:"primary"`
	Type                     types.String                   `tfsdk:"type"`
	State                    types.String                   `tfsdk:"state"`
	InterfaceSecurityEnabled types.Bool                     `tfsdk:"interface_security_enabled"`
	SecurityGroups           []FreeTierVmSecurityGroupModel `tfsdk:"security_groups"`
	FloatingIP               *FreeTierVmFloatingIPModel     `tfsdk:"floating_ip"`
}

type FreeTierVmAvailabilityZoneModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type FreeTierVmTagModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Color types.String `tfsdk:"color"`
}

// FreeTierVmResourceModel is the full Terraform state for cloudru-community_free_tier_vm.
type FreeTierVmResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`

	// Input: request a new public IP on the interface.
	NewFloatingIP types.Bool `tfsdk:"new_floating_ip"`

	// Computed.
	State        types.String `tfsdk:"state"`
	Locked       types.Bool   `tfsdk:"locked"`
	VncURL       types.String `tfsdk:"vnc_url"`
	VncWS        types.String `tfsdk:"vnc_ws"`
	CreatedTime  types.String `tfsdk:"created_time"`
	ModifiedTime types.String `tfsdk:"modified_time"`

	// Blocks.
	AvailabilityZone FreeTierVmAvailabilityZoneModel `tfsdk:"availability_zone"`
	Image            FreeTierVmImageModel            `tfsdk:"image"`
	Flavor           FreeTierVmFlavorModel           `tfsdk:"flavor"`
	Disks            []FreeTierVmDiskModel           `tfsdk:"disks"`
	Interfaces       []FreeTierVmInterfaceModel      `tfsdk:"interfaces"`
	Tags             []FreeTierVmTagModel            `tfsdk:"tags"`
}

// ─── Metadata & Schema ───────────────────────────────────────────────────────

func (r *FreeTierVmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_free_tier_vm"
}

func (r *FreeTierVmResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Cloud.ru free-tier virtual machine (managed via Compute API ` + "`POST /api/v1/free-tier`" + `).

The platform automatically selects the flavor and boot disk — you only provide
an image, a name and optional credentials/networking.

> Each Cloud.ru organisation may have at most one active free-tier VM.
> Use ` + "`GET /api/v1/free-tier?project_id=…`" + ` to check eligibility before creating.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VM ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Project ID. Defaults to the provider `project_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "VM name (1–64 chars, must start with a letter).",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "VM description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"new_floating_ip": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Assign a new public IP to the VM's network interface.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VM state (`running`, `stopped`, `creating`, …).",
			},
			"locked": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the VM is locked for changes.",
			},
			"vnc_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VNC console URL.",
			},
			"vnc_ws": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "VNC WebSocket URL.",
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
			// ── availability_zone ─────────────────────────────────────────
			"availability_zone": schema.SingleNestedBlock{
				MarkdownDescription: "Availability zone. If omitted, the platform picks the default free-tier zone.",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Availability zone ID.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
							stringplanmodifier.RequiresReplace(),
						},
					},
					"name": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Availability zone name (e.g. `ru.AZ-1`).",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},

			// ── image ─────────────────────────────────────────────────────
			"image": schema.SingleNestedBlock{
				MarkdownDescription: "Boot image and initial credentials. The image must have `free_tier_enabled = true`.",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Image ID. Must be a free-tier-enabled image.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Image name (computed).",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"free_tier_enabled": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether this image supports free tier (always true for valid free-tier VMs).",
					},
					"host_name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Hostname to set inside the VM.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"user_name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Initial OS user name.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"public_key": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "SSH public key for the initial user.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"password": schema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						MarkdownDescription: "Initial user password.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},

			// ── flavor (fully computed) ────────────────────────────────────
			"flavor": schema.SingleNestedBlock{
				MarkdownDescription: "Flavor assigned by the platform (read-only).",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Flavor ID.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Flavor name.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"cpu": schema.Int64Attribute{
						Computed:            true,
						MarkdownDescription: "Number of vCPUs.",
					},
					"ram": schema.Int64Attribute{
						Computed:            true,
						MarkdownDescription: "RAM in GiB.",
					},
					"gpu": schema.Int64Attribute{
						Computed:            true,
						MarkdownDescription: "Number of GPUs.",
					},
					"oversubscription": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "vCPU oversubscription ratio (e.g. `1:1`).",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},

			// ── disks (fully computed) ─────────────────────────────────────
			"disks": schema.ListNestedBlock{
				MarkdownDescription: "Disks attached to the VM (assigned by the platform, read-only).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Disk ID.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Disk name.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"primary": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is the boot disk.",
						},
						"size": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Disk size in GiB.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Disk state.",
						},
					},
					Blocks: map[string]schema.Block{
						"disk_type": schema.SingleNestedBlock{
							MarkdownDescription: "Disk type (assigned by the platform).",
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Disk type ID.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Disk type name.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
							},
						},
					},
				},
			},

			// ── interfaces (fully computed) ────────────────────────────────
			"interfaces": schema.ListNestedBlock{
				MarkdownDescription: "Network interfaces (assigned by the platform, read-only).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface ID.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface name.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"ip_address": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Private IP address.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"primary": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is the primary interface.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface type.",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface state.",
						},
						"interface_security_enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the interface can be added to a security group.",
						},
					},
					Blocks: map[string]schema.Block{
						"security_groups": schema.ListNestedBlock{
							MarkdownDescription: "Security groups attached to this interface.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Security group ID.",
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
									"name": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Security group name.",
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
									"state": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Security group state.",
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseStateForUnknown(),
										},
									},
								},
							},
						},
						"floating_ip": schema.SingleNestedBlock{
							MarkdownDescription: "Floating IP attached to this interface (present when `new_floating_ip = true`).",
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Floating IP ID.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"ip_address": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Public IP address.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Floating IP name.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"state": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Floating IP state.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
							},
						},
					},
				},
			},

			// ── tags ──────────────────────────────────────────────────────
			"tags": schema.ListNestedBlock{
				MarkdownDescription: "Tags attached to this VM.",
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

// ─── Configure ───────────────────────────────────────────────────────────────

func (r *FreeTierVmResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.CloudRuHttpClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.CloudRuHttpClient, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = c
}

// ─── API structs ─────────────────────────────────────────────────────────────

type apiFreeTierVmDiskType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiFreeTierVmDisk struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Primary  bool                  `json:"primary"`
	Size     int64                 `json:"size"`
	State    string                `json:"state"`
	DiskType apiFreeTierVmDiskType `json:"disk_type"`
}

type apiFreeTierVmSecurityGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type apiFreeTierVmFloatingIP struct {
	ID        string `json:"id"`
	IPAddress string `json:"ip_address"`
	Name      string `json:"name"`
	State     string `json:"state"`
}

type apiFreeTierVmInterface struct {
	ID                       string                       `json:"id"`
	Name                     string                       `json:"name"`
	IPAddress                string                       `json:"ip_address"`
	Primary                  bool                         `json:"primary"`
	Type                     string                       `json:"type"`
	State                    string                       `json:"state"`
	InterfaceSecurityEnabled bool                         `json:"interface_security_enabled"`
	SecurityGroups           []apiFreeTierVmSecurityGroup `json:"security_groups"`
	FloatingIP               *apiFreeTierVmFloatingIP     `json:"floating_ip"`
}

type apiFreeTierVmFlavor struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	CPU              int64  `json:"cpu"`
	RAM              int64  `json:"ram"`
	GPU              int64  `json:"gpu"`
	Oversubscription string `json:"oversubscription"`
}

type apiFreeTierVmImage struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	FreeTierEnabled bool   `json:"free_tier_enabled"`
}

type apiFreeTierVmAvailabilityZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiFreeTierVmProject struct {
	ID string `json:"id"`
}

type apiFreeTierVmTag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// apiFreeTierVmState mirrors VmState — same enum values as regular VMs.
// apiFreeTierVmResponse mirrors FreeTierVmResponse from the OpenAPI spec.
type apiFreeTierVmResponse struct {
	ID               string                        `json:"id"`
	Name             string                        `json:"name"`
	Description      string                        `json:"description"`
	State            string                        `json:"state"`
	Locked           bool                          `json:"locked"`
	Flavor           apiFreeTierVmFlavor           `json:"flavor"`
	Image            apiFreeTierVmImage            `json:"image"`
	AvailabilityZone apiFreeTierVmAvailabilityZone `json:"availability_zone"`
	Project          apiFreeTierVmProject          `json:"project"`
	Disks            []apiFreeTierVmDisk           `json:"disks"`
	Interfaces       []apiFreeTierVmInterface      `json:"interfaces"`
	Tags             []apiFreeTierVmTag            `json:"tags"`
	VncURL           string                        `json:"vnc_url"`
	VncWS            string                        `json:"vnc_ws"`
	CreatedTime      string                        `json:"created_time"`
	ModifiedTime     string                        `json:"modified_time"`
}

// ─── Flatten ─────────────────────────────────────────────────────────────────

func flattenFreeTierVmResponse(api *apiFreeTierVmResponse, data *FreeTierVmResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	data.State = types.StringValue(api.State)
	data.Locked = types.BoolValue(api.Locked)
	data.ProjectID = types.StringValue(api.Project.ID)
	data.VncURL = types.StringValue(api.VncURL)
	data.VncWS = types.StringValue(api.VncWS)
	data.CreatedTime = types.StringValue(api.CreatedTime)
	data.ModifiedTime = types.StringValue(api.ModifiedTime)

	data.AvailabilityZone = FreeTierVmAvailabilityZoneModel{
		ID:   types.StringValue(api.AvailabilityZone.ID),
		Name: types.StringValue(api.AvailabilityZone.Name),
	}

	// Image: preserve user-supplied credentials (not returned by API).
	data.Image.ID = types.StringValue(api.Image.ID)
	data.Image.Name = types.StringValue(api.Image.Name)
	data.Image.FreeTierEnabled = types.BoolValue(api.Image.FreeTierEnabled)

	data.Flavor = FreeTierVmFlavorModel{
		ID:               types.StringValue(api.Flavor.ID),
		Name:             types.StringValue(api.Flavor.Name),
		CPU:              types.Int64Value(api.Flavor.CPU),
		RAM:              types.Int64Value(api.Flavor.RAM),
		GPU:              types.Int64Value(api.Flavor.GPU),
		Oversubscription: types.StringValue(api.Flavor.Oversubscription),
	}

	disks := make([]FreeTierVmDiskModel, len(api.Disks))
	for i, d := range api.Disks {
		disks[i] = FreeTierVmDiskModel{
			ID:      types.StringValue(d.ID),
			Name:    types.StringValue(d.Name),
			Primary: types.BoolValue(d.Primary),
			Size:    types.Int64Value(d.Size),
			State:   types.StringValue(d.State),
			DiskType: FreeTierVmDiskTypeModel{
				ID:   types.StringValue(d.DiskType.ID),
				Name: types.StringValue(d.DiskType.Name),
			},
		}
	}
	data.Disks = disks

	ifaces := make([]FreeTierVmInterfaceModel, len(api.Interfaces))
	for i, iface := range api.Interfaces {
		m := FreeTierVmInterfaceModel{
			ID:                       types.StringValue(iface.ID),
			Name:                     types.StringValue(iface.Name),
			IPAddress:                types.StringValue(iface.IPAddress),
			Primary:                  types.BoolValue(iface.Primary),
			Type:                     types.StringValue(iface.Type),
			State:                    types.StringValue(iface.State),
			InterfaceSecurityEnabled: types.BoolValue(iface.InterfaceSecurityEnabled),
		}
		sgs := make([]FreeTierVmSecurityGroupModel, len(iface.SecurityGroups))
		for j, sg := range iface.SecurityGroups {
			sgs[j] = FreeTierVmSecurityGroupModel{
				ID:    types.StringValue(sg.ID),
				Name:  types.StringValue(sg.Name),
				State: types.StringValue(sg.State),
			}
		}
		m.SecurityGroups = sgs
		if iface.FloatingIP != nil {
			m.FloatingIP = &FreeTierVmFloatingIPModel{
				ID:        types.StringValue(iface.FloatingIP.ID),
				IPAddress: types.StringValue(iface.FloatingIP.IPAddress),
				Name:      types.StringValue(iface.FloatingIP.Name),
				State:     types.StringValue(iface.FloatingIP.State),
			}
		}
		ifaces[i] = m
	}
	data.Interfaces = ifaces

	tags := make([]FreeTierVmTagModel, len(api.Tags))
	for i, t := range api.Tags {
		tags[i] = FreeTierVmTagModel{
			ID:    types.StringValue(t.ID),
			Name:  types.StringValue(t.Name),
			Color: types.StringValue(t.Color),
		}
	}
	data.Tags = tags
}

// ─── CRUD ─────────────────────────────────────────────────────────────────────

func (r *FreeTierVmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FreeTierVmResourceModel
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
		"image_id":   data.Image.ID.ValueString(),
	}

	if v := data.AvailabilityZone.ID.ValueString(); v != "" {
		body["availability_zone_id"] = v
	}

	if !data.NewFloatingIP.IsNull() && !data.NewFloatingIP.IsUnknown() {
		body["new_floating_ip"] = data.NewFloatingIP.ValueBool()
	}

	// image_metadata carries credentials (hostname, username, ssh key, password).
	imgMeta := map[string]string{
		"hostname": data.Image.HostName.ValueString(),
		"username": data.Image.UserName.ValueString(),
	}
	if v := data.Image.PublicKey.ValueString(); v != "" {
		imgMeta["ssh_pub_key"] = v
	}
	if v := data.Image.Password.ValueString(); v != "" {
		imgMeta["password"] = v
	}
	body["image_metadata"] = imgMeta

	if len(data.Tags) > 0 {
		ids := make([]string, len(data.Tags))
		for i, t := range data.Tags {
			ids[i] = t.ID.ValueString()
		}
		body["tag_ids"] = ids
	}

	createURL := r.client.ComputeEndpoint + "/api/v1/free-tier"
	var apiRespArr []apiFreeTierVmResponse
	if err := r.client.PostJSONCreated(ctx, createURL, body, &apiRespArr); err != nil {
		resp.Diagnostics.AddError("Create Free Tier VM Error", err.Error())
		return
	}
	if len(apiRespArr) == 0 {
		resp.Diagnostics.AddError("Create Free Tier VM Error", "API returned empty response array")
		return
	}

	apiResp, err := r.waitForState(ctx, apiRespArr[0].ID, []string{"running", "stopped"})
	if err != nil {
		resp.Diagnostics.AddError("Create Free Tier VM Wait Error", err.Error())
		return
	}

	// Preserve user-supplied credentials before flattening.
	savedImage := data.Image
	flattenFreeTierVmResponse(&apiResp, &data)
	data.Image.HostName = savedImage.HostName
	data.Image.UserName = savedImage.UserName
	data.Image.PublicKey = savedImage.PublicKey
	data.Image.Password = savedImage.Password
	data.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FreeTierVmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FreeTierVmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Free tier VMs are accessible via the standard GET /api/v1/vms/{id} endpoint.
	getURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	var apiResp apiFreeTierVmResponse
	if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Free Tier VM Error", err.Error())
		return
	}

	savedImage := data.Image
	flattenFreeTierVmResponse(&apiResp, &data)
	data.Image.HostName = savedImage.HostName
	data.Image.UserName = savedImage.UserName
	data.Image.PublicKey = savedImage.PublicKey
	data.Image.Password = savedImage.Password

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FreeTierVmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FreeTierVmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state FreeTierVmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	body := map[string]interface{}{
		"name":        plan.Name.ValueString(),
		"description": plan.Description.ValueString(),
	}
	if len(plan.Tags) > 0 {
		ids := make([]string, len(plan.Tags))
		for i, t := range plan.Tags {
			ids[i] = t.ID.ValueString()
		}
		body["tag_ids"] = ids
	} else {
		body["tag_ids"] = []string{}
	}

	putURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, plan.ID.ValueString())
	var apiResp apiFreeTierVmResponse
	if err := r.client.PutJSON(ctx, putURL, body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Update Free Tier VM Error", err.Error())
		return
	}

	apiResp, err := r.waitForState(ctx, apiResp.ID, []string{"running", "stopped"})
	if err != nil {
		resp.Diagnostics.AddError("Update Free Tier VM Wait Error", err.Error())
		return
	}

	savedImage := plan.Image
	flattenFreeTierVmResponse(&apiResp, &plan)
	plan.Image.HostName = savedImage.HostName
	plan.Image.UserName = savedImage.UserName
	plan.Image.PublicKey = savedImage.PublicKey
	plan.Image.Password = savedImage.Password

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FreeTierVmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FreeTierVmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Collect boot disk IDs for co-deletion.
	var bootDiskIDs []string
	for _, d := range data.Disks {
		if d.Primary.ValueBool() && !d.ID.IsNull() && !d.ID.IsUnknown() {
			bootDiskIDs = append(bootDiskIDs, d.ID.ValueString())
		}
	}

	deleteBody := map[string]interface{}{
		"delete_attachments": map[string]interface{}{
			"disk_ids":     bootDiskIDs,
			"external_ips": []string{},
		},
	}

	delURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	if err := r.client.DeleteWithBodyNoContent(ctx, delURL, deleteBody); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete Free Tier VM Error", err.Error())
		return
	}

	// Poll until 404.
	getURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	for {
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Delete Free Tier VM Timeout", ctx.Err().Error())
			return
		case <-time.After(3 * time.Second):
		}
		var apiResp apiFreeTierVmResponse
		err := r.client.GetJSON(ctx, getURL, &apiResp)
		if err != nil {
			if client.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError("Delete Free Tier VM Poll Error", err.Error())
			return
		}
		if apiResp.State == "error_deleting" || apiResp.State == "error" {
			resp.Diagnostics.AddError("Delete Free Tier VM Error",
				fmt.Sprintf("VM entered %s state", apiResp.State))
			return
		}
	}
}

func (r *FreeTierVmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// waitForState polls GET /api/v1/vms/{id} until one of targetStates is reached.
func (r *FreeTierVmResource) waitForState(ctx context.Context, vmID string, targetStates []string) (apiFreeTierVmResponse, error) {
	getURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, vmID)

	isTarget := func(s string) bool {
		for _, t := range targetStates {
			if s == t {
				return true
			}
		}
		return false
	}
	isError := func(s string) bool {
		return s == "error" || s == "error_creating" || s == "error_deleting"
	}

	var apiResp apiFreeTierVmResponse
	if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
		return apiResp, err
	}
	for !isTarget(apiResp.State) {
		if isError(apiResp.State) {
			return apiResp, fmt.Errorf("free tier VM %s entered error state: %s", vmID, apiResp.State)
		}
		select {
		case <-ctx.Done():
			return apiResp, ctx.Err()
		case <-time.After(3 * time.Second):
		}
		if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
			return apiResp, err
		}
	}
	return apiResp, nil
}
