package provider

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

func TestAccK3sAgentResource(t *testing.T) {

	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Errorf("%v", err.Error())
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigDirectory: config.StaticDirectory("../../tests/k3s_agent"),
				Config:          providerConfig,
				ConfigVariables: map[string]config.Variable{
					"server_host":  config.StringVariable(inputs.Nodes[2]),
					"agent_host_1": config.StringVariable(inputs.Nodes[3]),
					"user":         config.StringVariable(inputs.User),
					"private_key":  config.StringVariable(inputs.SshKey),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("token"),
					),
					statecheck.ExpectKnownValue(
						"k3s_agent.main",
						tfjsonpath.New("active"),
						knownvalue.NotNull(),
					),
				},
			},
			{
				Config: providerConfig,
				PreConfig: func() {
					serverClient, err := inputs.SshClient(t, 2)
					if err != nil {
						t.Errorf("%v", err.Error())
					}
					server := k3s.NewK3sServerComponent(t.Context(), nil, nil, nil, "/usr/local/bin")
					if err := server.Resync(serverClient); err != nil {
						t.Errorf("%v", err.Error())
					}

					agentClient, err := inputs.SshClient(t, 4)
					if err != nil {
						t.Errorf("%v", err.Error())
					}

					if _, err = agentClient.Run(fmt.Sprintf("curl -sfL https://get.k3s.io | K3S_URL=https://%s:6443 K3S_TOKEN=%s sh -", inputs.Nodes[2], server.Token())); err != nil {
						t.Errorf("%v", err.Error())
					}

				},
				ImportState:        true,
				ImportStatePersist: true,
				ResourceName:       "k3s_agent.secondary[0]",
				ImportStateId:      fmt.Sprintf("host=%s,user=%s,private_key=%s", inputs.Nodes[4], inputs.User, inputs.SshKey),
				ConfigDirectory:    config.StaticDirectory("../../tests/k3s_agent"),
				ConfigVariables: map[string]config.Variable{
					"server_host":  config.StringVariable(inputs.Nodes[2]),
					"agent_host_1": config.StringVariable(inputs.Nodes[3]),
					"agent_host_2": config.StringVariable(inputs.Nodes[4]),
					"user":         config.StringVariable(inputs.User),
					"private_key":  config.StringVariable(inputs.SshKey),
					"secondary":    config.BoolVariable(true),
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("token"),
					),
					statecheck.ExpectKnownValue(
						"k3s_agent.main",
						tfjsonpath.New("active"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"k3s_agent.secondary[0]",
						tfjsonpath.New("active"),
						knownvalue.NotNull(),
					),
				},
			}, {
				Config:             providerConfig,
				ExpectNonEmptyPlan: false,
				PlanOnly:           true,
				ConfigDirectory:    config.StaticDirectory("../../tests/k3s_agent"),
				ConfigVariables: map[string]config.Variable{
					"server_host":  config.StringVariable(inputs.Nodes[2]),
					"agent_host_1": config.StringVariable(inputs.Nodes[3]),
					"agent_host_2": config.StringVariable(inputs.Nodes[4]),
					"user":         config.StringVariable(inputs.User),
					"private_key":  config.StringVariable(inputs.SshKey),
					"secondary":    config.BoolVariable(true),
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
				host	 = "192.168.1.2"
				user	 = "ubuntu"
				password = "abc123"
				server	 = "192.168.1.1"
				token 	 = "abc123"
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
				host	    = "192.168.1.2"
				user	    = "ubuntu"
				private_key = "abc123"
				server      = "192.168.1.1"
				token 	    = "abc123"
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
				host	    = "192.168.1.2"
				user	    = "ubuntu"
				private_key = "abc123"
				server      = "192.168.1.1"
				token 	    = "abc123"
				password    = "abc123"
			}`,
		}},
	})

}
