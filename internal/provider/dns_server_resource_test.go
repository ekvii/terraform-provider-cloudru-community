package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDnsServerResource(t *testing.T) {
	projectID := os.Getenv("CLOUDRU_PROJECT_ID")
	keyID := os.Getenv("CLOUDRU_AUTH_KEY_ID")
	secret := os.Getenv("CLOUDRU_AUTH_SECRET")
	subnetID := os.Getenv("CLOUDRU_DNS_SUBNET_ID")

	if projectID == "" || keyID == "" || secret == "" || subnetID == "" {
		t.Skip("CLOUDRU_PROJECT_ID, CLOUDRU_AUTH_KEY_ID, CLOUDRU_AUTH_SECRET, CLOUDRU_DNS_SUBNET_ID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDnsServerResourceConfig(projectID, keyID, secret, subnetID, "tf-test-dns"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(
						"cloudru-community_dns_server.test",
						"id",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_dns_server.test",
						"name",
						"tf-test-dns",
					),
					resource.TestCheckResourceAttr(
						"cloudru-community_dns_server.test",
						"subnet_id",
						subnetID,
					),
				),
			},
		},
	})
}

func testAccDnsServerResourceConfig(projectID, keyID, secret, subnetID, name string) string {
	return fmt.Sprintf(`
provider "cloudru-community" {
  project_id  = "%s"
  auth_key_id = "%s"
  auth_secret = "%s"
}

resource "cloudru-community_dns_server" "test" {
  name        = "%s"
  subnet_id   = "%s"
  description = "created by terraform acceptance test"
}
`, projectID, keyID, secret, name, subnetID)
}
