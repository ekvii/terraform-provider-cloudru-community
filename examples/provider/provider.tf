# Copyright (c) HashiCorp, Inc.

provider "cloudru-community" {
  project_id  = var.project_id
  auth_key_id = var.auth_key_id
  auth_secret = var.auth_secret

  # Optional overrides
  # vpc_endpoint = "https://vpc.api.cloud.ru"
  # dns_endpoint = "https://dns.api.cloud.ru"
}

variable "project_id" {
  type        = string
  description = "Cloud.ru project ID"
}

variable "auth_key_id" {
  type        = string
  description = "Cloud.ru API key ID"
}

variable "auth_secret" {
  type        = string
  description = "Cloud.ru API key secret"
  sensitive   = true
}
