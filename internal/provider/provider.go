package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-cloudru-community/internal/client"
)

var _ provider.Provider = &CloudRuCommunityProvider{}

type CloudRuCommunityProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type CloudRuCommunityProviderModel struct {
	ProjectID   types.String `tfsdk:"project_id"`
	AuthKeyID   types.String `tfsdk:"auth_key_id"`
	AuthSecret  types.String `tfsdk:"auth_secret"`
	VpcEndpoint types.String `tfsdk:"vpc_endpoint"`
	DnsEndpoint types.String `tfsdk:"dns_endpoint"`
}

func (p *CloudRuCommunityProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cloudru-community"
	resp.Version = p.version
}

func (p *CloudRuCommunityProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Cloud.ru project ID",
				Required:            true,
			},
			"auth_key_id": schema.StringAttribute{
				MarkdownDescription: "Cloud.ru API key ID",
				Required:            true,
			},
			"auth_secret": schema.StringAttribute{
				MarkdownDescription: "Cloud.ru API key secret",
				Required:            true,
				Sensitive:           true,
			},
			"vpc_endpoint": schema.StringAttribute{
				MarkdownDescription: "Cloud.ru VPC API endpoint",
				Optional:            true,
			},
			"dns_endpoint": schema.StringAttribute{
				MarkdownDescription: "Cloud.ru DNS API endpoint",
				Optional:            true,
			},
		},
	}
}

func (p *CloudRuCommunityProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data CloudRuCommunityProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vpcEndpoint := data.VpcEndpoint.ValueString()
	if vpcEndpoint == "" {
		vpcEndpoint = "https://vpc.api.cloud.ru"
	}

	dnsEndpoint := data.DnsEndpoint.ValueString()
	if dnsEndpoint == "" {
		dnsEndpoint = "https://dns.api.cloud.ru"
	}

	c, err := client.NewCloudRuHttpClient(
		ctx,
		data.AuthKeyID.ValueString(),
		data.AuthSecret.ValueString(),
		data.ProjectID.ValueString(),
		vpcEndpoint,
		dnsEndpoint,
	)
	if err != nil {
		resp.Diagnostics.AddError("Authentication Error", err.Error())
		return
	}

	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *CloudRuCommunityProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCloudRuVpcResource,
		NewDnsServerResource,
	}
}

func (p *CloudRuCommunityProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVpcsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CloudRuCommunityProvider{
			version: version,
		}
	}
}
