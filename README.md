# Unofficial cloud.ru provider for Terraform

This terraform cloud provider is complementary to the official [cloud.ru provider](https://registry.terraform.io/providers/cloudru/cloudru/latest) and is driven by the community. It provides additional resources and features that are not available in the official provider, allowing users to manage their cloud.ru infrastructure more effectively. When the resource is available in the official provider, it will be removed from this provider. Feature prioritization is based on user demand (via opened issues) and direct contribution.

## Development

The provider is built on top of the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework) and scaffolded from the [Terraform Provider Scaffolding Framework repository](https://github.com/hashicorp/terraform-provider-scaffolding-framework).
It uses the [cloud.ru API](https://cloud.ru/docs/api/) to interact with the cloud.ru services.

See GNUmakefile for the project SDLC details and development routine automation.

## Local testing

1. Run `make install`
2. Put `set_overrides` directive to the `~/.terraformrc`:

```
provider_installation {

  dev_overrides {
      "registry.terraform.io/ekvii/cloudru-community" = "<GOBIN:~/go/bin/>"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```


