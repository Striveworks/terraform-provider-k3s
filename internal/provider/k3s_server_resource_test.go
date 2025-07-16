package provider_test

import (
	"maps"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"gopkg.in/yaml.v2"
)

func TestAccK3sServerResource(t *testing.T) {
	var K3sServerStaticFile = config.StaticFile("../../examples/resources/k3s_server/examples/basic/resource.tf")

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}
	inputs = inputs.ServerTests()
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
						"k3s_server.main",
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
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.main", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				PreConfig: func() {
					client, err := inputs.SshClient(t, 0)
					if err != nil {
						t.Errorf("Could not create ssh client: %v", err.Error())
					}
					res, err := client.Run("sudo cat /etc/rancher/k3s/config.yaml")
					if err != nil {
						t.Errorf("%v", err.Error())
					}

					contents := res[0]
					var expected map[string]bool
					if err := yaml.Unmarshal([]byte(res[0]), &expected); err != nil {
						t.Errorf("%v", err.Error())
					}

					if !maps.Equal(map[string]bool{
						"embedded-registry": true,
					}, expected) {
						t.Errorf("Expected config to be 'embedded-registry: true' but found %v", contents)
					}
				},
				PlanOnly: true,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("embedded-registry: true"),
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

func TestAccK3sHAServerResource(t *testing.T) {
	var K3sServerStaticFile = config.StaticFile("../../examples/resources/k3s_server/examples/ha/resource.tf")

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}
	inputs = inputs.ServerTests()
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
						"k3s_server.init",
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
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.init", plancheck.ResourceActionUpdate)},
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
				auth = {
				   host		= "192.168.1.1"
				   user		= "ubuntu"
				   password	= "abc123"
		        }
			}`,
		}, {
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `
			resource "k3s_server" "main" {
			    auth = {
					host		= "192.168.1.1"
					user		= "ubuntu"
					private_key	= "somelongkey"
				}
			}`,
		}, {
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)both password and private key were passed, only pass one(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				auth = {
					host        = "192.168.1.1"
					user        = "ubuntu"
					private_key = "somelongkey"
					password    = "abc123"
				}
			}`,
		}, {
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)when not in cluster-init, token and server must be passed(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				auth = {
					host        = "192.168.1.1"
					user        = "ubuntu"
					private_key = "somelongkey"
		        }
				highly_available = {}
			}`,
		}, {
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`(.*)when in cluster-init, token and server must not be passed(.*)`),
			Config: providerConfig + `
			resource "k3s_server" "main" {
				auth = {
					host        = "192.168.1.1"
					user        = "ubuntu"
					private_key = "somelongkey"
				}
				highly_available = {
					cluster_init = true
					token = "absdad"
				}
			}`,
		}},
	})
}
