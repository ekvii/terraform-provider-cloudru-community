terraform {
  required_providers {
    cloudru-community = {
      source = "registry.terraform.io/ekvii/cloudru-community"
    }
  }
}

variable "project_id" {
  description = "Cloud.ru Project ID"
  type        = string
}

variable "auth_key_id" {
  description = "Cloud.ru API Key ID"
  type        = string
  sensitive   = true
}

variable "auth_secret" {
  description = "Cloud.ru API Key Secret"
  type        = string
  sensitive   = true
}

provider "cloudru-community" {
  project_id  = var.project_id
  auth_key_id = var.auth_key_id
  auth_secret = var.auth_secret
}

data "cloudru-community_vpcs" "all" {}

output "vpc_names" {
  value = [for v in data.cloudru-community_vpcs.all.vpcs : v.name]
}
