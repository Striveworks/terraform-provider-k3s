package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestK3sKubeconfigValidateDatasource(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)(.*)`),
			Config: providerConfig + `
			resource "k3s_kubeconfig" "main" {
				auth = {
				host	    = "192.168.1.2"
				user	    = "ubuntu"
				private_key = "abc123"
				password    = "abc123"
				}
 			}`,
		}},
	})

}
