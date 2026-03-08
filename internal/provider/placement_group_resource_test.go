package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPlacementGroupResource(t *testing.T) {
	projectID := os.Getenv("CLOUDRU_PROJECT_ID")
	keyID := os.Getenv("CLOUDRU_AUTH_KEY_ID")
	secret := os.Getenv("CLOUDRU_AUTH_SECRET")

	if projectID == "" || keyID == "" || secret == "" {
		t.Skip("CLOUDRU_PROJECT_ID, CLOUDRU_AUTH_KEY_ID, CLOUDRU_AUTH_SECRET must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify
			{
				Config: testAccPlacementGroupResourceConfig(projectID, keyID, secret, "tf-test-pg", "soft-anti-affinity"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(
						"cloudru-community_placement_group.test",
						"id",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_placement_group.test",
						"name",
						"tf-test-pg",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_placement_group.test",
						"policy",
						"soft-anti-affinity",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_placement_group.test",
						"description",
						"created by terraform acceptance test",
					),
					resource.TestCheckResourceAttrSet(
						"cloudru-community_placement_group.test",
						"created_time",
					),
				),
			},
			// Update name and verify
			{
				Config: testAccPlacementGroupResourceConfigUpdated(projectID, keyID, secret, "tf-test-pg-updated", "soft-anti-affinity"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"cloudru-community_placement_group.test",
						"name",
						"tf-test-pg-updated",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_placement_group.test",
						"policy",
						"soft-anti-affinity",
					),
				),
			},
			// Import
			{
				ResourceName:      "cloudru-community_placement_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPlacementGroupResourceConfig(projectID, keyID, secret, name, policy string) string {
	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id  = "%s"
  auth_key_id = "%s"
  auth_secret = "%s"
}

resource "cloudru-community_placement_group" "test" {
  name        = "%s"
  policy      = "%s"
  description = "created by terraform acceptance test"
}
`, projectID, keyID, secret, name, policy)
}

func testAccPlacementGroupResourceConfigUpdated(projectID, keyID, secret, name, policy string) string {
	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id  = "%s"
  auth_key_id = "%s"
  auth_secret = "%s"
}

resource "cloudru-community_placement_group" "test" {
  name        = "%s"
  policy      = "%s"
  description = "created by terraform acceptance test"
}
`, projectID, keyID, secret, name, policy)
}
