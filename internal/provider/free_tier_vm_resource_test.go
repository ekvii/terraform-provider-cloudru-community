package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFreeTierVmResource tests the full lifecycle of a free-tier VM:
// create → read → update name → import.
//
// Required env vars:
//   - CLOUDRU_PROJECT_ID
//   - CLOUDRU_AUTH_KEY_ID
//   - CLOUDRU_AUTH_SECRET
//   - CLOUDRU_FREE_TIER_IMAGE_ID  — image with free_tier_enabled=true
//   - CLOUDRU_SSH_PUB_KEY         — optional, but recommended
//
// Optional:
//   - CLOUDRU_AZ_ID               — availability zone ID; if empty the platform picks the default free-tier AZ
func TestAccFreeTierVmResource(t *testing.T) {
	projectID := os.Getenv("CLOUDRU_PROJECT_ID")
	keyID := os.Getenv("CLOUDRU_AUTH_KEY_ID")
	secret := os.Getenv("CLOUDRU_AUTH_SECRET")
	imageID := os.Getenv("CLOUDRU_FREE_TIER_IMAGE_ID")
	sshPubKey := os.Getenv("CLOUDRU_SSH_PUB_KEY")
	azID := os.Getenv("CLOUDRU_AZ_ID") // optional

	if projectID == "" || keyID == "" || secret == "" || imageID == "" {
		t.Skip("CLOUDRU_PROJECT_ID, CLOUDRU_AUTH_KEY_ID, CLOUDRU_AUTH_SECRET, " +
			"CLOUDRU_FREE_TIER_IMAGE_ID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify.
			{
				Config: testAccFreeTierVmConfig(projectID, keyID, secret, "tf-test-free-vm", imageID, azID, sshPubKey, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("cloudru-community_free_tier_vm.test", "id"),
					resource.TestCheckResourceAttr("cloudru-community_free_tier_vm.test", "name", "tf-test-free-vm"),
					resource.TestCheckResourceAttr("cloudru-community_free_tier_vm.test", "state", "running"),
					resource.TestCheckResourceAttrSet("cloudru-community_free_tier_vm.test", "created_time"),
					resource.TestCheckResourceAttrSet("cloudru-community_free_tier_vm.test", "flavor.id"),
					resource.TestCheckResourceAttrSet("cloudru-community_free_tier_vm.test", "flavor.cpu"),
				),
			},
			// Update name.
			{
				Config: testAccFreeTierVmConfig(projectID, keyID, secret, "tf-test-free-vm-upd", imageID, azID, sshPubKey, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("cloudru-community_free_tier_vm.test", "name", "tf-test-free-vm-upd"),
				),
			},
			// Import.
			{
				ResourceName:            "cloudru-community_free_tier_vm.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"image.host_name", "image.user_name", "image.public_key", "image.password"},
			},
		},
	})
}

// TestAccFreeTierVmResource_WithFloatingIP tests free-tier VM creation with
// new_floating_ip = true.
func TestAccFreeTierVmResource_WithFloatingIP(t *testing.T) {
	projectID := os.Getenv("CLOUDRU_PROJECT_ID")
	keyID := os.Getenv("CLOUDRU_AUTH_KEY_ID")
	secret := os.Getenv("CLOUDRU_AUTH_SECRET")
	imageID := os.Getenv("CLOUDRU_FREE_TIER_IMAGE_ID")
	sshPubKey := os.Getenv("CLOUDRU_SSH_PUB_KEY")
	azID := os.Getenv("CLOUDRU_AZ_ID")

	if projectID == "" || keyID == "" || secret == "" || imageID == "" {
		t.Skip("CLOUDRU_PROJECT_ID, CLOUDRU_AUTH_KEY_ID, CLOUDRU_AUTH_SECRET, " +
			"CLOUDRU_FREE_TIER_IMAGE_ID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFreeTierVmConfig(projectID, keyID, secret, "tf-test-free-vm-fip", imageID, azID, sshPubKey, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("cloudru-community_free_tier_vm.test", "id"),
					resource.TestCheckResourceAttr("cloudru-community_free_tier_vm.test", "new_floating_ip", "true"),
					resource.TestCheckResourceAttr("cloudru-community_free_tier_vm.test", "state", "running"),
				),
			},
		},
	})
}

// ── Config helpers ─────────────────────────────────────────────────────────────

func testAccFreeTierVmConfig(projectID, keyID, secret, name, imageID, azID, sshPubKey string, newFIP bool) string {
	azBlock := ""
	if azID != "" {
		azBlock = fmt.Sprintf(`
  availability_zone {
    id = %q
  }`, azID)
	}

	sshLine := ""
	if sshPubKey != "" {
		sshLine = fmt.Sprintf(`    public_key = %q`, sshPubKey)
	}

	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id  = %q
  auth_key_id = %q
  auth_secret = %q
}

resource "cloudru-community_free_tier_vm" "test" {
  name           = %q
  new_floating_ip = %t
%s

  image {
    id        = %q
    host_name = "tf-test-free-vm"
    user_name = "ubuntu"
    %s
  }
}
`, projectID, keyID, secret, name, newFIP, azBlock, imageID, sshLine)
}
