package provider

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var (
	k3s_server_tpl = `
resource "k3s_server" "main" {
	host        = var.host
	user        = var.user
	private_key = var.private_key
}`
)

func TestAccK3sServerResource(t *testing.T) {

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{

		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + k3s_server_tpl,
			ConfigVariables: map[string]config.Variable{
				"server_host": config.StringVariable(inputs.Nodes[0]),
				"user":        config.StringVariable(inputs.User),
				"private_key": config.StringVariable(inputs.SshKey),
			},
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectSensitiveValue(
					"k3s_server.main",
					tfjsonpath.New("token"),
				),
			},
		}, {
			Config:  providerConfig + k3s_server_tpl,
			Destroy: true,
		}},
	})

}

func TestK3sServerValidateResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{

			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host		= "192.168.1.1"
				user		= "ubuntu"
				password	= "abc123"
			}`,
		}},
	})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host		= "192.168.1.1"
				user		= "ubuntu"
				private_key	= "somelongkey"
			}`,
		}},
	})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host        = "192.168.1.1"
				user        = "ubuntu"
				private_key = "somelongkey"
				password    = "abc123"
			}`,
		}},
	})
}
