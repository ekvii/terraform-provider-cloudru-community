# Unofficial cloud.ru provider for Terraform

[![Tests](https://github.com/ekvii/terraform-provider-cloudru-community/actions/workflows/test.yml/badge.svg)](https://github.com/ekvii/terraform-provider-cloudru-community/actions/workflows/test.yml)
![Version](https://img.shields.io/github/v/release/ekvii/terraform-provider-cloudru-community?label=Version)

> **Disclaimer**: This provider is not officially supported by cloud.ru and is maintained by the community. Use it at your own risk and always review the code and documentation before using it in production environments.

This Terraform provider is complementary to the official [cloud.ru provider](https://github.com/cloud-ru/evo-terraform) and is community-driven. It provides additional resources and features not yet available in the official provider, allowing users to manage their cloud.ru infrastructure more effectively. Once a resource becomes available in the official provider, it will be removed from this one. Feature prioritization is based on user demand (via open issues) and direct contributions.

## Supported entities

- `cloudru-community_vpc`: (resource) Manage VPCs in cloud.ru.
- `cloudru-community_subnet`: (resource) Manage subnets in the custom VPCs.
- `cloudru-community_dns_server`: (resource) Manage Evolution DNS servers in cloud.ru.
- `cloudru-community_vpcs` (data source): Retrieve a list of VPCs in cloud.ru.

## Usage

Recommented to use in pair with the official provider.

```hcl
terraform {
  required_version = ">= 1.14.6"

  required_providers {
    cloudru = {
      source  = "cloud.ru/cloudru/cloud"
      version = "1.6.0"
    }
    cloudru-community = {
      source = "registry.terraform.io/ekvii/cloudru-community"
    }
  }
}

provider "cloudru" {
  project_id         = var.project_id
  auth_key_id        = var.auth_key_id
  auth_secret        = var.auth_secret
  iam_endpoint       = "iam.api.cloud.ru:443"
  evolution_endpoint = "https://compute.api.cloud.ru"
}

provider "cloudru-community" {
  project_id  = var.project_id
  auth_key_id = var.auth_key_id
  auth_secret = var.auth_secret
}

```

## Development

The provider is built on top of the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework), using the [Terraform Provider Scaffolding Framework](https://github.com/hashicorp/terraform-provider-scaffolding-framework) as a template.
It uses the [cloud.ru API](https://cloud.ru/docs/api/) to interact with cloud.ru services.

See `GNUmakefile` for SDLC details and development automation.

## Local testing

1. Run `make install`
2. Add a `dev_overrides` block to `~/.terraformrc`:

```
provider_installation {

  dev_overrides {
    "registry.terraform.io/ekvii/cloudru-community" = "/your/GOBIN/path"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

3. Use the provider in your Terraform configuration (see the `examples` directory for more details):

```hcl
terraform {
  required_providers {
    cloudru-community = {
      source = "registry.terraform.io/ekvii/cloudru-community"
    }
  }
}
```
