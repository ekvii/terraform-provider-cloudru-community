package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &CloudRuCommunityProvider{}

type CloudRuCommunityProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type CloudRuProviderClient struct {
	HTTPClient  *http.Client
	Token       string
	ProjectID   string
	VpcEndpoint string
	DnsEndpoint string
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

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", data.AuthKeyID.ValueString())
	form.Set("client_secret", data.AuthSecret.ValueString())

	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://id.cloud.ru/auth/system/openid/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Token Request Error", err.Error())
		return
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := http.DefaultClient
	tokenResp, err := httpClient.Do(tokenReq)
	if err != nil {
		resp.Diagnostics.AddError("Token HTTP Error", err.Error())
		return
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Token Error", fmt.Sprintf("unexpected status: %d", tokenResp.StatusCode))
		return
	}

	var tokenBody struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenBody); err != nil {
		resp.Diagnostics.AddError("Token Decode Error", err.Error())
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

	client := &CloudRuProviderClient{
		HTTPClient:  httpClient,
		Token:       tokenBody.AccessToken,
		ProjectID:   data.ProjectID.ValueString(),
		VpcEndpoint: vpcEndpoint,
		DnsEndpoint: dnsEndpoint,
	}

	resp.ResourceData = client
}

func (p *CloudRuCommunityProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCloudRuVpcResource,
		NewDnsServerResource,
	}
}

func (p *CloudRuCommunityProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CloudRuCommunityProvider{
			version: version,
		}
	}
}
