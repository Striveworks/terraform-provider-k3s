package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
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
				Config: providerConfig,
				PreConfig: func() {
					serverClient, err := inputs.SshClient(t, 0)
					if err != nil {
						t.Errorf("%v", err.Error())
					}
					server := k3s.NewK3sServerComponent(t.Context(), nil, nil, nil, "/usr/local/bin")
					if err := server.Resync(serverClient); err != nil {
						t.Errorf("%v", err.Error())
					}

					agentClient, err := inputs.SshClient(t, 2)
					if err != nil {
						t.Errorf("%v", err.Error())
					}

					if _, err = agentClient.Run(fmt.Sprintf("curl -sfL https://get.k3s.io | K3S_URL=https://%s:6443 K3S_TOKEN=%s sh -", inputs.Nodes[0], server.Token())); err != nil {
						t.Errorf("%v", err.Error())
					}

				},
				ImportState:        true,
				ImportStatePersist: true,
				ResourceName:       "k3s_agent.main[1]",
				ImportStateId:      fmt.Sprintf("host=%s,user=%s,private_key=%s", inputs.Nodes[2], inputs.User, inputs.SshKey),
				ConfigFile:         K3sAgentStaticFile,
				ConfigVariables: map[string]config.Variable{
					"server_host": config.StringVariable(inputs.Nodes[0]),
					"agent_hosts": config.ListVariable(config.StringVariable(inputs.Nodes[1]), config.StringVariable(inputs.Nodes[2])),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"secondary":   config.BoolVariable(true),
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
					statecheck.ExpectKnownValue(
						"k3s_agent.main[1]",
						tfjsonpath.New("active"),
						knownvalue.NotNull(),
					),
				},
			}, {
				Config:             providerConfig,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
				ConfigFile:         K3sAgentStaticFile,
				ConfigVariables: map[string]config.Variable{
					"server_host": config.StringVariable(inputs.Nodes[0]),
					"agent_hosts": config.ListVariable(config.StringVariable(inputs.Nodes[1]), config.StringVariable(inputs.Nodes[2])),
					"user":        config.StringVariable(inputs.User),
					"private_key": config.StringVariable(inputs.SshKey),
					"secondary":   config.BoolVariable(true),
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
