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

var _ resource.Resource = &VmResource{}
var _ resource.ResourceWithImportState = &VmResource{}

func NewVmResource() resource.Resource {
	return &VmResource{}
}

// VmResource manages Cloud.ru virtual machines via POST /api/v1.1/vms.
type VmResource struct {
	client *client.CloudRuHttpClient
}

// ─── Terraform state models ──────────────────────────────────────────────────

// VmAvailabilityZoneModel mirrors the availability_zone block.
type VmAvailabilityZoneModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// VmBootDiskTypeModel mirrors the disk_type nested block inside boot_disk.
type VmBootDiskTypeModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// VmBootDiskModel mirrors the boot_disk block.
type VmBootDiskModel struct {
	ID       types.String        `tfsdk:"id"`
	Name     types.String        `tfsdk:"name"`
	Size     types.Int64         `tfsdk:"size"`
	DiskType VmBootDiskTypeModel `tfsdk:"disk_type"`
	State    types.String        `tfsdk:"state"`
}

// VmImageModel mirrors the image block.
type VmImageModel struct {
	Name      types.String `tfsdk:"name"`
	HostName  types.String `tfsdk:"host_name"`
	UserName  types.String `tfsdk:"user_name"`
	PublicKey types.String `tfsdk:"public_key"`
	Password  types.String `tfsdk:"password"`
}

// VmSecurityGroupModel mirrors the security_groups nested block inside network_interfaces.
type VmSecurityGroupModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	State types.String `tfsdk:"state"`
}

// VmFipModel mirrors the fip block inside network_interfaces.
type VmFipModel struct {
	ID        types.String `tfsdk:"id"`
	IPAddress types.String `tfsdk:"ip_address"`
	Name      types.String `tfsdk:"name"`
	State     types.String `tfsdk:"state"`
}

// VmSubnetModel mirrors the subnet block inside network_interfaces.
type VmSubnetModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	SubnetAddress types.String `tfsdk:"subnet_address"`
	RoutedNetwork types.Bool   `tfsdk:"routed_network"`
	State         types.String `tfsdk:"state"`
}

// VmNetworkInterfaceModel mirrors the network_interfaces block.
type VmNetworkInterfaceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`

	// For direct_ip: allocate a new external IP.
	NewExternalIP types.Bool `tfsdk:"new_external_ip"`

	// Fixed internal IP (regular interfaces only).
	IPAddress types.String `tfsdk:"ip_address"`

	// Computed.
	InterfaceSecurityEnabled types.Bool   `tfsdk:"interface_security_enabled"`
	Type                     types.String `tfsdk:"type"`
	State                    types.String `tfsdk:"state"`
	Primary                  types.Bool   `tfsdk:"primary"`
	CreatedTime              types.String `tfsdk:"created_time"`
	ModifiedTime             types.String `tfsdk:"modified_time"`

	// Nested blocks.
	Subnet         *VmSubnetModel         `tfsdk:"subnet"`
	SecurityGroups []VmSecurityGroupModel `tfsdk:"security_groups"`
	Fip            *VmFipModel            `tfsdk:"fip"`
}

// VmTagModel mirrors a tag attached to a VM.
type VmTagModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Color types.String `tfsdk:"color"`
}

// VmPlacementGroupModel mirrors the placement_group block.
type VmPlacementGroupModel struct {
	Name types.String `tfsdk:"name"`
	ID   types.String `tfsdk:"id"`
}

// VmResourceModel is the full Terraform state model for the vm resource.
type VmResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	FlavorID    types.String `tfsdk:"flavor_id"`

	// Computed.
	VncURL       types.String `tfsdk:"vnc_url"`
	VncWS        types.String `tfsdk:"vnc_ws"`
	CreatedTime  types.String `tfsdk:"created_time"`
	ModifiedTime types.String `tfsdk:"modified_time"`
	State        types.String `tfsdk:"state"`
	Locked       types.Bool   `tfsdk:"locked"`

	// Blocks.
	AvailabilityZone  VmAvailabilityZoneModel   `tfsdk:"availability_zone"`
	PlacementGroup    *VmPlacementGroupModel    `tfsdk:"placement_group"`
	Image             VmImageModel              `tfsdk:"image"`
	BootDisk          VmBootDiskModel           `tfsdk:"boot_disk"`
	NetworkInterfaces []VmNetworkInterfaceModel `tfsdk:"network_interfaces"`
	Tags              []VmTagModel              `tfsdk:"tags"`
}

// ─── Resource metadata & schema ──────────────────────────────────────────────

func (r *VmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (r *VmResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Cloud.ru virtual machine (managed via Compute API v1.1).

Supports both **regular** interfaces (with an optional attached floating IP) and
**direct_ip** interfaces (a public IP assigned directly to the interface, no NAT).

> Floating IPs must be pre-created with the official ` + "`cloudru_evolution_fip`" + ` resource
> and passed to this resource via ` + "`network_interfaces[*].fip.id`" + `.`,

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
				MarkdownDescription: "VM description (0–255 chars).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"flavor_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Flavor ID. Changing this value triggers an in-place update (resize).",
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
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current VM state (`running`, `stopped`, `creating`, …).",
			},
			"locked": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the VM is locked for changes.",
			},
		},

		Blocks: map[string]schema.Block{
			// ── availability_zone ─────────────────────────────────────────
			"availability_zone": schema.SingleNestedBlock{
				MarkdownDescription: "Availability zone for the VM. Specify `id` or `name`.",
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

			// ── placement_group ───────────────────────────────────────────
			"placement_group": schema.SingleNestedBlock{
				MarkdownDescription: "Optional placement group. Specify `name` or `id`. Requires replacement.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Placement group name.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
							stringplanmodifier.RequiresReplace(),
						},
					},
					"id": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Placement group ID.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},

			// ── image ─────────────────────────────────────────────────────
			"image": schema.SingleNestedBlock{
				MarkdownDescription: "Boot image and initial credentials. All fields require replacement.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Image name (e.g. `ubuntu-22.04`).",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
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

			// ── boot_disk ─────────────────────────────────────────────────
			"boot_disk": schema.SingleNestedBlock{
				MarkdownDescription: "Boot disk configuration.",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Disk ID (computed after creation).",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Disk name.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"size": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "Disk size in GiB.",
					},
					"state": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Disk state.",
					},
				},
				Blocks: map[string]schema.Block{
					"disk_type": schema.SingleNestedBlock{
						MarkdownDescription: "Disk type. Specify `id`.",
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Disk type ID.",
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.RequiresReplace(),
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

			// ── network_interfaces ────────────────────────────────────────
			"network_interfaces": schema.ListNestedBlock{
				MarkdownDescription: `Network interfaces (1–8). Interface type is inferred:
- **subnet** block present → ` + "`regular`" + ` (private network, optional floating IP via ` + "`fip`" + ` block).
- **subnet** block absent + ` + "`new_external_ip = true`" + ` → ` + "`direct_ip`" + ` (public IP directly on interface, no NAT).`,
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
						"description": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Interface description.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"new_external_ip": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Allocate a new public IP for a `direct_ip` interface.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"ip_address": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Fixed internal IP address (regular interfaces only). Omit to auto-assign.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"interface_security_enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the interface can be added to a security group.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface type (`regular` or `direct_ip`).",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface state.",
						},
						"primary": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is the primary interface.",
						},
						"created_time": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface creation timestamp.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"modified_time": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface last modification timestamp.",
						},
					},
					Blocks: map[string]schema.Block{
						"subnet": schema.SingleNestedBlock{
							MarkdownDescription: "Subnet to attach to (for regular interfaces). Specify `id` or `name`.",
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Optional:            true,
									Computed:            true,
									MarkdownDescription: "Subnet ID.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Optional:            true,
									Computed:            true,
									MarkdownDescription: "Subnet name (e.g. `Default`).",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"subnet_address": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Subnet CIDR.",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
									},
								},
								"routed_network": schema.BoolAttribute{
									Computed:            true,
									MarkdownDescription: "Whether the subnet is routed.",
								},
								"state": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Subnet state.",
								},
							},
						},
						"security_groups": schema.ListNestedBlock{
							MarkdownDescription: "Security groups attached to this interface.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Security group ID.",
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
						"fip": schema.SingleNestedBlock{
							MarkdownDescription: "Floating IP to attach (must be pre-created via `cloudru_evolution_fip`).",
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Floating IP ID.",
								},
								"ip_address": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Floating IP address.",
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

func (r *VmResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ─── API structs ─────────────────────────────────────────────────────────────

type apiVmDiskType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiVmDisk struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Primary  bool          `json:"primary"`
	Size     int64         `json:"size"`
	State    string        `json:"state"`
	DiskType apiVmDiskType `json:"disk_type"`
}

type apiVmSecurityGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type apiVmFloatingIP struct {
	ID        string `json:"id"`
	IPAddress string `json:"ip_address"`
	Name      string `json:"name"`
	State     string `json:"state"`
}

type apiVmSubnet struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SubnetAddress string `json:"subnet_address"`
	RoutedNetwork bool   `json:"routed_network"`
	State         string `json:"state"`
}

type apiVmInterface struct {
	ID                       string               `json:"id"`
	Name                     string               `json:"name"`
	IPAddress                string               `json:"ip_address"`
	SecurityGroups           []apiVmSecurityGroup `json:"security_groups"`
	FloatingIP               *apiVmFloatingIP     `json:"floating_ip"`
	Primary                  bool                 `json:"primary"`
	Type                     string               `json:"type"`
	InterfaceSecurityEnabled bool                 `json:"interface_security_enabled"`
	State                    string               `json:"state"`
	Subnet                   *apiVmSubnet         `json:"subnet"`
}

type apiVmFlavor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiVmAvailabilityZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiVmProject struct {
	ID string `json:"id"`
}

type apiVmPlacementGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type apiVmTag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// apiVmResponse mirrors PublicVmResponse.
type apiVmResponse struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	State            string                `json:"state"`
	Locked           bool                  `json:"locked"`
	Flavor           apiVmFlavor           `json:"flavor"`
	AvailabilityZone apiVmAvailabilityZone `json:"availability_zone"`
	PlacementGroup   *apiVmPlacementGroup  `json:"placement_group"`
	Project          apiVmProject          `json:"project"`
	Disks            []apiVmDisk           `json:"disks"`
	Interfaces       []apiVmInterface      `json:"interfaces"`
	Tags             []apiVmTag            `json:"tags"`
	VncURL           string                `json:"vnc_url"`
	VncWS            string                `json:"vnc_ws"`
	CreatedTime      string                `json:"created_time"`
	ModifiedTime     string                `json:"modified_time"`
}

// ─── Flatten helper ───────────────────────────────────────────────────────────

// flattenVmResponse copies an API response into the Terraform state model,
// preserving user-supplied values that are not returned by the API (image credentials).
func flattenVmResponse(api *apiVmResponse, data *VmResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	data.State = types.StringValue(api.State)
	data.Locked = types.BoolValue(api.Locked)
	data.FlavorID = types.StringValue(api.Flavor.ID)
	data.ProjectID = types.StringValue(api.Project.ID)
	data.VncURL = types.StringValue(api.VncURL)
	data.VncWS = types.StringValue(api.VncWS)
	data.CreatedTime = types.StringValue(api.CreatedTime)
	data.ModifiedTime = types.StringValue(api.ModifiedTime)

	data.AvailabilityZone = VmAvailabilityZoneModel{
		ID:   types.StringValue(api.AvailabilityZone.ID),
		Name: types.StringValue(api.AvailabilityZone.Name),
	}

	if api.PlacementGroup != nil {
		data.PlacementGroup = &VmPlacementGroupModel{
			ID:   types.StringValue(api.PlacementGroup.ID),
			Name: types.StringValue(api.PlacementGroup.Name),
		}
	} else {
		data.PlacementGroup = nil
	}

	// Boot disk: find the primary disk in the response.
	for _, d := range api.Disks {
		if d.Primary {
			data.BootDisk.ID = types.StringValue(d.ID)
			data.BootDisk.State = types.StringValue(d.State)
			if d.Name != "" {
				data.BootDisk.Name = types.StringValue(d.Name)
			}
			if d.Size > 0 {
				data.BootDisk.Size = types.Int64Value(d.Size)
			}
			data.BootDisk.DiskType.ID = types.StringValue(d.DiskType.ID)
			data.BootDisk.DiskType.Name = types.StringValue(d.DiskType.Name)
			break
		}
	}

	// Network interfaces: only expose regular and direct_ip interfaces.
	// We match by position with the existing state to preserve user config.
	prevIfaces := data.NetworkInterfaces
	ifaces := make([]VmNetworkInterfaceModel, 0, len(api.Interfaces))
	apiIdx := 0
	for _, iface := range api.Interfaces {
		if iface.Type != "regular" && iface.Type != "direct_ip" && iface.Type != "fip" {
			continue
		}

		m := VmNetworkInterfaceModel{
			ID:                       types.StringValue(iface.ID),
			Name:                     types.StringValue(iface.Name),
			IPAddress:                types.StringValue(iface.IPAddress),
			InterfaceSecurityEnabled: types.BoolValue(iface.InterfaceSecurityEnabled),
			Type:                     types.StringValue(iface.Type),
			State:                    types.StringValue(iface.State),
			Primary:                  types.BoolValue(iface.Primary),
		}

		// Preserve user-supplied fields from prior state.
		if apiIdx < len(prevIfaces) {
			m.Description = prevIfaces[apiIdx].Description
			m.NewExternalIP = prevIfaces[apiIdx].NewExternalIP
			m.CreatedTime = prevIfaces[apiIdx].CreatedTime
			m.ModifiedTime = prevIfaces[apiIdx].ModifiedTime
		} else {
			m.Description = types.StringNull()
			m.NewExternalIP = types.BoolValue(false)
			m.CreatedTime = types.StringValue("")
			m.ModifiedTime = types.StringValue("")
		}

		// Subnet.
		if iface.Subnet != nil {
			m.Subnet = &VmSubnetModel{
				ID:            types.StringValue(iface.Subnet.ID),
				Name:          types.StringValue(iface.Subnet.Name),
				SubnetAddress: types.StringValue(iface.Subnet.SubnetAddress),
				RoutedNetwork: types.BoolValue(iface.Subnet.RoutedNetwork),
				State:         types.StringValue(iface.Subnet.State),
			}
		}

		// Security groups.
		sgs := make([]VmSecurityGroupModel, len(iface.SecurityGroups))
		for j, sg := range iface.SecurityGroups {
			sgs[j] = VmSecurityGroupModel{
				ID:    types.StringValue(sg.ID),
				Name:  types.StringValue(sg.Name),
				State: types.StringValue(sg.State),
			}
		}
		m.SecurityGroups = sgs

		// Floating IP.
		if iface.FloatingIP != nil {
			m.Fip = &VmFipModel{
				ID:        types.StringValue(iface.FloatingIP.ID),
				IPAddress: types.StringValue(iface.FloatingIP.IPAddress),
				Name:      types.StringValue(iface.FloatingIP.Name),
				State:     types.StringValue(iface.FloatingIP.State),
			}
		} else if apiIdx < len(prevIfaces) {
			// Preserve user-specified fip block even when the API doesn't echo it.
			m.Fip = prevIfaces[apiIdx].Fip
		}

		ifaces = append(ifaces, m)
		apiIdx++
	}
	data.NetworkInterfaces = ifaces

	// Tags.
	tags := make([]VmTagModel, len(api.Tags))
	for i, t := range api.Tags {
		tags[i] = VmTagModel{
			ID:    types.StringValue(t.ID),
			Name:  types.StringValue(t.Name),
			Color: types.StringValue(t.Color),
		}
	}
	data.Tags = tags
}

// ─── CRUD ─────────────────────────────────────────────────────────────────────

func (r *VmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID
	}

	vmReq := r.buildCreateRequest(&data, projectID)

	// POST /api/v1.1/vms expects an array and returns an array (201 Created).
	createURL := r.client.ComputeEndpoint + "/api/v1.1/vms"
	var apiRespArr []apiVmResponse
	if err := r.client.PostJSONCreated(ctx, createURL, []interface{}{vmReq}, &apiRespArr); err != nil {
		resp.Diagnostics.AddError("Create VM Error", err.Error())
		return
	}
	if len(apiRespArr) == 0 {
		resp.Diagnostics.AddError("Create VM Error", "API returned empty response array")
		return
	}
	apiResp := apiRespArr[0]

	// Poll until running/stopped or terminal error.
	apiResp, err := r.waitForVmState(ctx, apiResp.ID, []string{"running", "stopped"})
	if err != nil {
		resp.Diagnostics.AddError("Create VM Wait Error", err.Error())
		return
	}

	flattenVmResponse(&apiResp, &data)
	data.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	var apiResp apiVmResponse
	if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read VM Error", err.Error())
		return
	}

	flattenVmResponse(&apiResp, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state VmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	body := map[string]interface{}{
		"name":        plan.Name.ValueString(),
		"description": plan.Description.ValueString(),
		"flavor_id":   plan.FlavorID.ValueString(),
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
	var apiResp apiVmResponse
	if err := r.client.PutJSON(ctx, putURL, body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Update VM Error", err.Error())
		return
	}

	// Wait for the VM to settle (e.g. after flavor resize).
	apiResp, err := r.waitForVmState(ctx, apiResp.ID, []string{"running", "stopped"})
	if err != nil {
		resp.Diagnostics.AddError("Update VM Wait Error", err.Error())
		return
	}

	flattenVmResponse(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Co-delete the boot disk along with the VM.
	var bootDiskIDs []string
	if !data.BootDisk.ID.IsNull() && !data.BootDisk.ID.IsUnknown() && data.BootDisk.ID.ValueString() != "" {
		bootDiskIDs = append(bootDiskIDs, data.BootDisk.ID.ValueString())
	}

	deleteBody := map[string]interface{}{
		"delete_attachments": map[string]interface{}{
			"disk_ids":     bootDiskIDs,
			"external_ips": []string{},
		},
	}

	delURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	if err := r.deleteWithBody(ctx, delURL, deleteBody); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Delete VM Error", err.Error())
		return
	}

	// Poll until 404.
	getURL := fmt.Sprintf("%s/api/v1/vms/%s", r.client.ComputeEndpoint, data.ID.ValueString())
	for {
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Delete VM Timeout", ctx.Err().Error())
			return
		case <-time.After(3 * time.Second):
		}
		var apiResp apiVmResponse
		err := r.client.GetJSON(ctx, getURL, &apiResp)
		if err != nil {
			if client.IsNotFound(err) {
				return
			}
			resp.Diagnostics.AddError("Delete VM Poll Error", err.Error())
			return
		}
		if apiResp.State == "error_deleting" || apiResp.State == "error" {
			resp.Diagnostics.AddError("Delete VM Error", fmt.Sprintf("VM entered %s state", apiResp.State))
			return
		}
	}
}

func (r *VmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildCreateRequest constructs the single-VM object to be sent inside the
// POST /api/v1.1/vms array body.
func (r *VmResource) buildCreateRequest(data *VmResourceModel, projectID string) map[string]interface{} {
	body := map[string]interface{}{
		"project_id": projectID,
		"name":       data.Name.ValueString(),
		"flavor_id":  data.FlavorID.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		body["description"] = data.Description.ValueString()
	}

	// Availability zone.
	if v := data.AvailabilityZone.ID.ValueString(); v != "" {
		body["availability_zone_id"] = v
	} else if v := data.AvailabilityZone.Name.ValueString(); v != "" {
		body["availability_zone_name"] = v
	}

	// Placement group.
	if data.PlacementGroup != nil {
		if v := data.PlacementGroup.ID.ValueString(); v != "" {
			body["placement_group_id"] = v
		} else if v := data.PlacementGroup.Name.ValueString(); v != "" {
			body["placement_group_name"] = v
		}
	}

	// Image: name + metadata map with credentials.
	body["image_name"] = data.Image.Name.ValueString()
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

	// Boot disk.
	disk := map[string]interface{}{
		"name":         data.BootDisk.Name.ValueString(),
		"size":         data.BootDisk.Size.ValueInt64(),
		"disk_type_id": data.BootDisk.DiskType.ID.ValueString(),
		"bootable":     true,
	}
	body["disks"] = []interface{}{disk}

	// Network interfaces.
	ifaces := make([]interface{}, 0, len(data.NetworkInterfaces))
	for _, ni := range data.NetworkInterfaces {
		ifaces = append(ifaces, r.buildInterfaceSection(&ni))
	}
	if len(ifaces) > 0 {
		body["interfaces"] = ifaces
	}

	// Tags.
	if len(data.Tags) > 0 {
		ids := make([]string, len(data.Tags))
		for i, t := range data.Tags {
			ids[i] = t.ID.ValueString()
		}
		body["tag_ids"] = ids
	}

	return body
}

// buildInterfaceSection maps a VmNetworkInterfaceModel to the
// VmCreateInterfaceSection API object.
func (r *VmResource) buildInterfaceSection(ni *VmNetworkInterfaceModel) map[string]interface{} {
	iface := map[string]interface{}{}

	if ni.Subnet != nil {
		// regular interface.
		iface["type"] = "regular"
		if v := ni.Subnet.ID.ValueString(); v != "" {
			iface["subnet_id"] = v
		} else if v := ni.Subnet.Name.ValueString(); v != "" {
			iface["subnet_name"] = v
		}
		if ni.Fip != nil {
			iface["attach_floating_ip_id"] = ni.Fip.ID.ValueString()
		}
		if v := ni.IPAddress.ValueString(); v != "" {
			iface["ip_address"] = v
		}
	} else {
		// direct_ip interface.
		iface["type"] = "direct_ip"
		newIP := true
		if !ni.NewExternalIP.IsNull() && !ni.NewExternalIP.IsUnknown() {
			newIP = ni.NewExternalIP.ValueBool()
		}
		iface["new_external_ip"] = newIP
	}

	if len(ni.SecurityGroups) > 0 {
		ids := make([]string, len(ni.SecurityGroups))
		for i, sg := range ni.SecurityGroups {
			ids[i] = sg.ID.ValueString()
		}
		iface["security_groups"] = ids
	}

	return iface
}

// waitForVmState polls GET /api/v1/vms/{id} until one of targetStates is reached.
func (r *VmResource) waitForVmState(ctx context.Context, vmID string, targetStates []string) (apiVmResponse, error) {
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

	var apiResp apiVmResponse
	if err := r.client.GetJSON(ctx, getURL, &apiResp); err != nil {
		return apiResp, err
	}
	for !isTarget(apiResp.State) {
		if isError(apiResp.State) {
			return apiResp, fmt.Errorf("VM %s entered error state: %s", vmID, apiResp.State)
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

// deleteWithBody issues a DELETE request with a JSON body and asserts HTTP 204.
// Delegates to the client's DeleteWithBodyNoContent method.
func (r *VmResource) deleteWithBody(ctx context.Context, url string, body interface{}) error {
	return r.client.DeleteWithBodyNoContent(ctx, url, body)
}
