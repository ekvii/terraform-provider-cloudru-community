package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVpcResource(t *testing.T) {
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
			{
				Config: testAccVpcResourceConfig(projectID, keyID, secret, "tf-test-vpc"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(
						"cloudru-community_vpc.test",
						"id",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_vpc.test",
						"name",
						"tf-test-vpc",
					),
				),
			},
		},
	})
}

func testAccVpcResourceConfig(projectID, keyID, secret, name string) string {
	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id   = "%s"
  auth_key_id  = "%s"
  auth_secret  = "%s"
}

resource "cloudru-community_vpc" "test" {
  name        = "%s"
  description = "created by terraform acceptance test"
}
`, projectID, keyID, secret, name)
}
