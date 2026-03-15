# Example: VM with a regular (private subnet) interface and a pre-existing floating IP.
# The floating IP must be created separately (e.g. via the official cloudru provider)
# and its ID passed to the fip block.

resource "cloudru-community_vm" "floating_ip" {
  name        = "my-vm-fip"
  description = "VM with a floating IP attached"
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
    name = "my-vm-fip-boot"
    size = 30
    disk_type {
      id = "disk-type-uuid"
    }
  }

  network_interfaces {
    subnet {
      id = "subnet-uuid"
    }

    # Attach a pre-existing floating IP (created by cloudru_evolution_fip or similar).
    fip {
      id = "fip-uuid"
    }
  }
}

output "vm_id" {
  value = cloudru-community_vm.floating_ip.id
}

output "vm_public_ip" {
  value = cloudru-community_vm.floating_ip.network_interfaces[0].fip[0].ip_address
}
