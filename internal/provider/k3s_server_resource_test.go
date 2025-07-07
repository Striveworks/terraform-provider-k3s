package provider_test

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

var K3sServerStaticFile = config.StaticFile("../../examples/resources/k3s_server/resource.tf")

func TestAccK3sServerResource(t *testing.T) {

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main[0]",
						tfjsonpath.New("token"),
					),
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
				},

				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.main[0]", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
				},
				Destroy: true,
			},
		},
	})
}

func TestAccK3sServerImportResource(t *testing.T) {
	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	client, err := inputs.SshClient(t, 1)
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	if _, err := client.Run("curl -sfL https://get.k3s.io | sh -"); err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				ImportState:   true,
				ConfigFile:    K3sServerStaticFile,
				ResourceName:  "k3s_server.main",
				ImportStateId: fmt.Sprintf("host=%s,user=%s,private_key=%s", inputs.Nodes[1], inputs.User, inputs.SshKey),
				Config:        providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[1]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
				ImportStatePersist: true,
			},
			{
				ConfigFile:         K3sServerStaticFile,
				ResourceName:       "k3s_server.main",
				Config:             providerConfig,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[1]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
			}},
	})
}

func TestAccK3sHAServerResource(t *testing.T) {

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"hosts": config.ListVariable(
						config.StringVariable(inputs.Nodes[2]),
						config.StringVariable(inputs.Nodes[3]),
						config.StringVariable(inputs.Nodes[4]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"highly_available": config.ObjectVariable(map[string]config.Variable{
						"cluster_init": config.BoolVariable(true),
					}),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.init[0]",
						tfjsonpath.New("token"),
					),
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"hosts": config.ListVariable(
						config.StringVariable(inputs.Nodes[2]),
						config.StringVariable(inputs.Nodes[3]),
						config.StringVariable(inputs.Nodes[4]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
				},

				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.init[0]", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"hosts": config.ListVariable(
						config.StringVariable(inputs.Nodes[2]),
						config.StringVariable(inputs.Nodes[3]),
						config.StringVariable(inputs.Nodes[4]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
				},
				Destroy: true,
			},
		},
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
		}, {
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host		= "192.168.1.1"
				user		= "ubuntu"
				private_key	= "somelongkey"
			}`,
		}, {
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)Both password and private key were passed, only pass one(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host        = "192.168.1.1"
				user        = "ubuntu"
				private_key = "somelongkey"
				password    = "abc123"
			}`,
		}, {
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)When not in cluster-init, token and server must be passed(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host        = "192.168.1.1"
				user        = "ubuntu"
				private_key = "somelongkey"
				highly_available = {
				}
			}`,
		}, {
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)When in cluster-init, token and server must not be passed(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				host        = "192.168.1.1"
				user        = "ubuntu"
				private_key = "somelongkey"
				highly_available = {
					cluster_init = true
					token = "absdad"
				}
			}`,
		}},
	})

}
