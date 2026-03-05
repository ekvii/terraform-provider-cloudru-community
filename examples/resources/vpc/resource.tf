resource "cloudru-community_vpc" "example" {
  name        = "example-vpc"
  description = "VPC created by Terraform"
}

output "vpc_id" {
  value = cloudru-community_vpc.example.id
}
