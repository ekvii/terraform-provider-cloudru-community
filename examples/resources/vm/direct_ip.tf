# Example: VM with a direct_ip interface (public IP assigned directly to the NIC, no NAT).
# No floating IP resource needed — set new_external_ip = true and Cloud.ru
# will allocate a public IP directly on the interface.

resource "cloudru-community_vm" "direct_ip" {
  name        = "my-vm-direct-ip"
  description = "VM with a direct public IP"
  flavor_id   = "flavor-uuid"

  availability_zone {
    name = "ru.AZ-1"
  }

  image {
    name       = "ubuntu-22.04"
    host_name  = "my-vm"
    user_name  = "ubuntu"
    public_key = "ssh-ed25519 AAAA..."
  }

  boot_disk {
    name = "my-vm-boot"
    size = 30
    disk_type {
      id = "disk-type-uuid"
    }
  }

  network_interfaces {
    new_external_ip = true
  }
}

output "vm_id" {
  value = cloudru-community_vm.direct_ip.id
}

output "vm_state" {
  value = cloudru-community_vm.direct_ip.state
}
