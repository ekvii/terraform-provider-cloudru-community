# Free-tier VM — the platform automatically selects the flavor and boot disk.
# Each organisation may have at most one active free-tier VM.
#
# Find a free-tier-enabled image ID with:
#   GET /api/v1/images?free_tier_enabled=true

resource "cloudru-community_free_tier_vm" "example" {
  name            = "my-free-vm"
  new_floating_ip = true # assign a public IP; set false to keep the VM private

  image {
    id         = "image-uuid" # must have free_tier_enabled = true
    host_name  = "my-free-vm"
    user_name  = "ubuntu"
    public_key = "ssh-ed25519 AAAA..."
  }
}

output "vm_id" {
  value = cloudru-community_free_tier_vm.example.id
}

output "vm_state" {
  value = cloudru-community_free_tier_vm.example.state
}

output "vm_flavor" {
  description = "Flavor assigned by the platform."
  value = {
    id   = cloudru-community_free_tier_vm.example.flavor.id
    name = cloudru-community_free_tier_vm.example.flavor.name
    cpu  = cloudru-community_free_tier_vm.example.flavor.cpu
    ram  = cloudru-community_free_tier_vm.example.flavor.ram
  }
}

output "vm_public_ip" {
  description = "Public IP (only set when new_floating_ip = true)."
  value = try(
    cloudru-community_free_tier_vm.example.interfaces[0].floating_ip[0].ip_address,
    null
  )
}
