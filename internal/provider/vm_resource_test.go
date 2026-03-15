package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccVmResource_DirectIP tests a VM with a direct_ip network interface
// (a public IP assigned directly to the interface, no NAT / floating IP).
func TestAccVmResource_DirectIP(t *testing.T) {
	projectID := os.Getenv("CLOUDRU_PROJECT_ID")
	keyID := os.Getenv("CLOUDRU_AUTH_KEY_ID")
	secret := os.Getenv("CLOUDRU_AUTH_SECRET")
	azName := os.Getenv("CLOUDRU_AZ_NAME") // e.g. "ru.AZ-1"
	flavorID := os.Getenv("CLOUDRU_FLAVOR_ID")
	diskTypeID := os.Getenv("CLOUDRU_DISK_TYPE_ID")
	imageName := os.Getenv("CLOUDRU_IMAGE_NAME") // e.g. "ubuntu-22.04"
	sshPubKey := os.Getenv("CLOUDRU_SSH_PUB_KEY")

	if projectID == "" || keyID == "" || secret == "" || azName == "" ||
		flavorID == "" || diskTypeID == "" || imageName == "" {
		t.Skip("CLOUDRU_PROJECT_ID, CLOUDRU_AUTH_KEY_ID, CLOUDRU_AUTH_SECRET, " +
			"CLOUDRU_AZ_NAME, CLOUDRU_FLAVOR_ID, CLOUDRU_DISK_TYPE_ID, CLOUDRU_IMAGE_NAME " +
			"must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify
			{
				Config: testAccVmResourceConfigDirectIP(
					projectID, keyID, secret,
					"tf-test-vm-directip",
					azName, flavorID, diskTypeID, imageName, sshPubKey,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("cloudru-community_vm.test", "id"),
					resource.TestCheckResourceAttr("cloudru-community_vm.test", "name", "tf-test-vm-directip"),
					resource.TestCheckResourceAttr("cloudru-community_vm.test", "state", "running"),
					resource.TestCheckResourceAttrSet("cloudru-community_vm.test", "created_time"),
					resource.TestCheckResourceAttrSet("cloudru-community_vm.test", "boot_disk.id"),
				),
			},
			// Update name
			{
				Config: testAccVmResourceConfigDirectIP(
					projectID, keyID, secret,
					"tf-test-vm-directip-upd",
					azName, flavorID, diskTypeID, imageName, sshPubKey,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("cloudru-community_vm.test", "name", "tf-test-vm-directip-upd"),
				),
			},
			// Import
			{
				ResourceName:            "cloudru-community_vm.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"image"},
			},
		},
	})
}

// TestAccVmResource_RegularFIP tests a VM with a regular network interface
// and an externally-created floating IP (FIP attached via fip.id).
func TestAccVmResource_RegularFIP(t *testing.T) {
	projectID := os.Getenv("CLOUDRU_PROJECT_ID")
	keyID := os.Getenv("CLOUDRU_AUTH_KEY_ID")
	secret := os.Getenv("CLOUDRU_AUTH_SECRET")
	azName := os.Getenv("CLOUDRU_AZ_NAME")
	flavorID := os.Getenv("CLOUDRU_FLAVOR_ID")
	diskTypeID := os.Getenv("CLOUDRU_DISK_TYPE_ID")
	imageName := os.Getenv("CLOUDRU_IMAGE_NAME")
	subnetID := os.Getenv("CLOUDRU_SUBNET_ID")
	fipID := os.Getenv("CLOUDRU_FIP_ID") // pre-existing FIP
	sshPubKey := os.Getenv("CLOUDRU_SSH_PUB_KEY")

	if projectID == "" || keyID == "" || secret == "" || azName == "" ||
		flavorID == "" || diskTypeID == "" || imageName == "" || subnetID == "" || fipID == "" {
		t.Skip("CLOUDRU_PROJECT_ID, CLOUDRU_AUTH_KEY_ID, CLOUDRU_AUTH_SECRET, " +
			"CLOUDRU_AZ_NAME, CLOUDRU_FLAVOR_ID, CLOUDRU_DISK_TYPE_ID, CLOUDRU_IMAGE_NAME, " +
			"CLOUDRU_SUBNET_ID, CLOUDRU_FIP_ID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify
			{
				Config: testAccVmResourceConfigRegularFIP(
					projectID, keyID, secret,
					"tf-test-vm-fip",
					azName, flavorID, diskTypeID, imageName, subnetID, fipID, sshPubKey,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("cloudru-community_vm.test", "id"),
					resource.TestCheckResourceAttr("cloudru-community_vm.test", "name", "tf-test-vm-fip"),
					resource.TestCheckResourceAttr("cloudru-community_vm.test", "state", "running"),
					resource.TestCheckResourceAttrSet("cloudru-community_vm.test", "created_time"),
					resource.TestCheckResourceAttrSet("cloudru-community_vm.test", "boot_disk.id"),
				),
			},
			// Import
			{
				ResourceName:            "cloudru-community_vm.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"image"},
			},
		},
	})
}

// ── Config helpers ─────────────────────────────────────────────────────────────

func testAccVmResourceConfigDirectIP(
	projectID, keyID, secret, name,
	azName, flavorID, diskTypeID, imageName, sshPubKey string,
) string {
	sshBlock := ""
	if sshPubKey != "" {
		sshBlock = fmt.Sprintf(`    public_key = %q`, sshPubKey)
	}
	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id  = %q
  auth_key_id = %q
  auth_secret = %q
}

resource "cloudru-community_vm" "test" {
  name      = %q
  flavor_id = %q

  availability_zone {
    name = %q
  }

  image {
    name      = %q
    host_name = "tf-test-vm"
    user_name = "ubuntu"
    %s
  }

  boot_disk {
    name = "tf-test-boot-disk"
    size = 20
    disk_type {
      id = %q
    }
  }

  network_interfaces {
    new_external_ip = true
  }
}
`, projectID, keyID, secret, name, flavorID, azName, imageName, sshBlock, diskTypeID)
}

func testAccVmResourceConfigRegularFIP(
	projectID, keyID, secret, name,
	azName, flavorID, diskTypeID, imageName, subnetID, fipID, sshPubKey string,
) string {
	sshBlock := ""
	if sshPubKey != "" {
		sshBlock = fmt.Sprintf(`    public_key = %q`, sshPubKey)
	}
	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id  = %q
  auth_key_id = %q
  auth_secret = %q
}

resource "cloudru-community_vm" "test" {
  name      = %q
  flavor_id = %q

  availability_zone {
    name = %q
  }

  image {
    name      = %q
    host_name = "tf-test-vm"
    user_name = "ubuntu"
    %s
  }

  boot_disk {
    name = "tf-test-boot-disk"
    size = 20
    disk_type {
      id = %q
    }
  }

  network_interfaces {
    subnet {
      id = %q
    }
    fip {
      id = %q
    }
  }
}
`, projectID, keyID, secret, name, flavorID, azName, imageName, sshBlock, diskTypeID, subnetID, fipID)
}
