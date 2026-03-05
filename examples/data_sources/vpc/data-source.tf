data "cloudru-community_vpcs" "all" {}

output "vpc_names" {
  value = [for v in data.cloudru-community_vpcs.all.vpcs : v.name]
}
