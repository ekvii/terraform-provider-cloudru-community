resource "cloudru-community_vpc" "example" {
  name        = "example-vpc"
  description = "VPC created by Terraform"
}

resource "cloudru-community_subnet" "example" {
  name            = "example-subnet"
  vpc_id          = cloudru-community_vpc.example.id
  subnet_address  = "192.168.1.0/24"
  default_gateway = "192.168.1.1"
  description     = "Subnet created by Terraform"
  routed_network  = true
  dns_servers     = ["8.8.8.8", "8.8.4.4"]

  availability_zone {
    id = "ru-central1-a"
  }
}

output "subnet_id" {
  value = cloudru-community_subnet.example.id
}
