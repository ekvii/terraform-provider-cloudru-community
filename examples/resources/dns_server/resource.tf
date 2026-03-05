variable "dns_subnet_id" {
  type        = string
  description = "Subnet ID where DNS server will be created"
}

resource "cloudru-community_dns_server" "example" {
  name        = "example-dns-server"
  subnet_id   = var.dns_subnet_id
  description = "DNS server created by Terraform"
}

output "dns_server_id" {
  value = cloudru-community_dns_server.example.id
}
