resource "cloudru-community_placement_group" "example" {
  name        = "example-placement-group"
  description = "Placement group created by Terraform"
  policy      = "soft-anti-affinity"
}

output "placement_group_id" {
  value = cloudru-community_placement_group.example.id
}
