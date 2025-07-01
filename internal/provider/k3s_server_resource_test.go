package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccK3sServerResource(t *testing.T) {

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			ConfigDirectory: config.StaticDirectory("../../tests/k3s_server"),
			Config:          providerConfig,
			ConfigVariables: map[string]config.Variable{
				"host":        config.StringVariable(inputs.Nodes[0]),
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
			Config:          providerConfig,
			ConfigDirectory: config.StaticDirectory("../../tests/k3s_server"),
			ConfigVariables: map[string]config.Variable{
				"host":        config.StringVariable(inputs.Nodes[0]),
				"user":        config.StringVariable(inputs.User),
				"private_key": config.StringVariable(inputs.SshKey),
			},
			Destroy: true,
		}},
	})
}

func TestAccK3sServerUpdateResource(t *testing.T) {
	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("../../tests/k3s_server"),
				Config:          providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[1]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("token"),
					),
				},
			},
			{
				ConfigDirectory: config.StaticDirectory("../../tests/k3s_server"),
				Config:          providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[1]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
				},

				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.main", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ConfigDirectory: config.StaticDirectory("../../tests/k3s_server"),
				Config:          providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[1]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
				},
				Destroy: true,
			}},
	})
}

func TestAccK3sServerImportResource(t *testing.T) {
	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	client, err := inputs.SshClient(t, 6)
	if err != nil {
		t.Errorf("%v", err.Error())
	}
	installLogs, err := client.Run("curl -sfL https://get.k3s.io | sh -")
	if err != nil {
		t.Errorf("%v", err.Error())
	}
	t.Log(installLogs)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				ImportState:        true,
				ConfigDirectory:    config.StaticDirectory("../../tests/k3s_server"),
				ResourceName:       "k3s_server.main",
				ImportStateId:      fmt.Sprintf("host=%s,user=%s,private_key=%s", inputs.Nodes[6], inputs.User, inputs.SshKey),
				Config:             providerConfig,
				ImportStatePersist: true,
			},
			{
				ConfigDirectory:    config.StaticDirectory("../../tests/k3s_server"),
				ResourceName:       "k3s_server.main",
				Config:             providerConfig,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[6]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
			}},
	})
}

func TestK3sServerValidateResource(t *testing.T) {
	t.Parallel()
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
