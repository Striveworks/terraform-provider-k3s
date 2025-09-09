package provider_test

import (
	"encoding/json"
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
	var K3sServerStaticFile = config.StaticFile("../../examples/resources/k3s_server/examples/basic/resource.tf")

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
					"host":        config.StringVariable(inputs.Nodes.Server[0]),
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
				PreConfig: func() {

					client, err := inputs.ServerSshClient(t, 0)
					if err != nil {
						t.Errorf("Could not create ssh client: %v", err.Error())
					}

					jres, err := client.Run("sudo k3s kubectl get nodes -l unit-test=basic -ojson")
					raw, _ := client.Run("sudo k3s kubectl get nodes -l unit-test=basic")
					if err != nil {
						t.Errorf("Could not run kubectl command: %v", err.Error())
					}

					var nodes map[string]any
					if err := json.Unmarshal([]byte(jres[0]), &nodes); err != nil {
						t.Fatal(err.Error())
					}
					nc, _ := nodes["items"].([]any)
					if len(nc) != 0 {
						t.Errorf("Wrong count of nodes expected. Expected 0, got %s", raw[0])
					}

				},
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes.Server[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("node-label: [\"unit-test=basic\"]"),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.main", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				PreConfig: func() {
					client, err := inputs.ServerSshClient(t, 0)
					if err != nil {
						t.Errorf("Could not create ssh client: %v", err.Error())
					}

					res, _ := client.Run("sudo k3s kubectl get nodes -l unit-test=basic -ojson")
					raw, _ := client.Run("sudo k3s kubectl get nodes -oyaml")

					var nodes map[string]any
					if err := json.Unmarshal([]byte(res[0]), &nodes); err != nil {
						t.Fatal(err.Error())
					}

					nc, _ := nodes["items"].([]any)
					if len(nc) != 1 {
						t.Errorf("Wrong count of nodes expected. Expected 1, got %s", raw[0])
					}
				},
				PlanOnly: true,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes.Server[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("node-label: [\"unit-test=basic\"]"),
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes.Server[0]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("node-label: [\"unit-test=basic\"]"),
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
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"hosts": config.ListVariable(
						config.StringVariable(inputs.Nodes.Server[1]),
						config.StringVariable(inputs.Nodes.Server[2]),
						config.StringVariable(inputs.Nodes.Server[3]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
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
						config.StringVariable(inputs.Nodes.Server[1]),
						config.StringVariable(inputs.Nodes.Server[2]),
						config.StringVariable(inputs.Nodes.Server[3]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("node-label: [\"unit-test=basic\"]"),
				},

				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("k3s_server.init", plancheck.ResourceActionUpdate)},
				},
			},
			{
				PreConfig: func() {

					client, err := inputs.ServerSshClient(t, 1)
					if err != nil {
						t.Errorf("Could not create ssh client: %v", err.Error())
					}

					jres, err := client.Run("sudo k3s kubectl get nodes -l unit-test=basic -ojson")
					raw, _ := client.Run("sudo k3s kubectl get nodes -l unit-test=basic")
					if err != nil {
						t.Errorf("Could not run kubectl command: %v", err.Error())
					}

					var nodes map[string]any
					if err := json.Unmarshal([]byte(jres[0]), &nodes); err != nil {
						t.Fatal(err.Error())
					}
					nc, _ := nodes["items"].([]any)
					if len(nc) != 3 {
						t.Errorf("Wrong count of nodes expected. Expected 3, got %s", raw[0])
					}

				},
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				PlanOnly:   true,
				ConfigVariables: map[string]config.Variable{
					"hosts": config.ListVariable(
						config.StringVariable(inputs.Nodes.Server[1]),
						config.StringVariable(inputs.Nodes.Server[2]),
						config.StringVariable(inputs.Nodes.Server[3]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("node-label: [\"unit-test=basic\"]"),
				},
			},
			{
				ConfigFile: K3sServerStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"hosts": config.ListVariable(
						config.StringVariable(inputs.Nodes.Server[1]),
						config.StringVariable(inputs.Nodes.Server[2]),
						config.StringVariable(inputs.Nodes.Server[3]),
					),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"config":      config.StringVariable("node-label: [\"unit-test=basic\"]"),
				},
				Destroy: true,
			},
		},
	})
}

func TestAccK3sServerSetVersion(t *testing.T) {
	// Read text to inject provider version
	raw, _ := os.ReadFile("../../examples/resources/k3s_server/examples/basic/resource.tf")

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}
	providerConfig := `
provider "k3s" {
	k3s_version = "v1.32.7+k3s1"
}
	` + string(raw)
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			Config: providerConfig,
			ConfigVariables: map[string]config.Variable{
				"host":        config.StringVariable(inputs.Nodes.Server[4]),
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
				PreConfig: func() {

					client, err := inputs.ServerSshClient(t, 4)
					if err != nil {
						t.Errorf("Could not create ssh client: %v", err.Error())
					}

					jres, err := client.Run("sudo k3s kubectl version -o json")
					if err != nil {
						t.Errorf("Could not run kubectl command: %v", err.Error())
					}

					var kversion map[string]any
					if err := json.Unmarshal([]byte(jres[0]), &kversion); err != nil {
						t.Fatal(err.Error())
					}
					serverVersion, ok := kversion["serverVersion"].(map[string]interface{})
					if !ok {
						t.Fatalf("serverVersion is not a map[string]interface{}")
					}
					gitVersion, ok := serverVersion["gitVersion"].(string)
					if !ok {
						t.Fatalf("gitVersion is not a string")
					}
					if gitVersion != "v1.32.7+k3s1" {
						t.Errorf("Wrong k3s version, expected 'v1.32.7+k3s1', got '%s'", gitVersion)
					}

				},
				Config:   providerConfig,
				PlanOnly: true,
				ConfigVariables: map[string]config.Variable{
					"host":        config.StringVariable(inputs.Nodes.Server[4]),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
				ExpectNonEmptyPlan: false,
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
