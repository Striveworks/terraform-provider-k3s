package provider_test

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var K3sAgentStaticFile = config.StaticFile("../../examples/resources/k3s_agent/resource.tf")

func TestAccK3sAgentResource(t *testing.T) {

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}
	inputs = inputs.AgentTests()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigFile: K3sAgentStaticFile,
				Config:     providerConfig,
				ConfigVariables: map[string]config.Variable{
					"server_host": config.StringVariable(inputs.Nodes[0]),
					"agent_hosts": config.ListVariable(config.StringVariable(inputs.Nodes[1])),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("token"),
					),
					statecheck.ExpectKnownValue(
						"k3s_agent.main[0]",
						tfjsonpath.New("active"),
						knownvalue.NotNull(),
					),
				},
			},
			{
				PreConfig: func() {

					client, err := inputs.SshClient(t, 0)
					if err != nil {
						t.Errorf("Could not create ssh client: %v", err.Error())
					}

					jres, err := client.Run("sudo k3s kubectl get nodes -ojson")
					raw, _ := client.Run("sudo k3s kubectl get nodes")
					if err != nil {
						t.Errorf("Could not run kubectl command: %v", err.Error())
					}

					var nodes map[string]any
					if err := json.Unmarshal([]byte(jres[0]), &nodes); err != nil {
						t.Fatal(err.Error())
					}
					nc, _ := nodes["items"].([]any)
					if len(nc) != 2 {
						t.Errorf("Wrong count of nodes expected. Expected 2, got %s", raw[0])
					}

				},
				Config:             providerConfig,
				ExpectNonEmptyPlan: true,
				PlanOnly:           true,
				ConfigFile:         K3sAgentStaticFile,
				ConfigVariables: map[string]config.Variable{
					"server_host": config.StringVariable(inputs.Nodes[0]),
					"agent_hosts": config.ListVariable(config.StringVariable(inputs.Nodes[1]), config.StringVariable(inputs.Nodes[2])),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
				},
			},
		},
	})
}

func TestK3sAgentValidateResource(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `
			resource "k3s_agent" "main" {
				auth = {
				host	   = "192.168.1.2"
				user	   = "ubuntu"
				password   = "abc123"
				}
				kubeconfig = "1asdsad"
				server	   = "192.168.1.1"
				token      = "abc123"
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
			resource "k3s_agent" "main" {
				auth = {
				host	    = "192.168.1.2"
				user	    = "ubuntu"
				private_key = "abc123"
		}
				server      = "192.168.1.1"
				token 	    = "abc123"
				kubeconfig  = "1asdsad"
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
			resource "k3s_agent" "main" {
				auth = {
				host	    = "192.168.1.2"
				user	    = "ubuntu"
				private_key = "abc123"
				password    = "abc123"
				}
				server      = "192.168.1.1"
				token 	    = "abc123"

				kubeconfig  = "1asdsad"
			}`,
		}},
	})

}
