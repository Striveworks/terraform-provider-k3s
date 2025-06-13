package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var (
	k3s_agent_tpl = `
resource "k3s_agent" "main" {
	host        = "{{ .agent_host }}"
	user        = "{{ .user }}"
	server      = "{{ .server_host }}"
	token		= k3s_server.main.token
	private_key =<<EOT
{{ .private_key }}
EOT
}`
)

func TestAccK3sAgentResource(t *testing.T) {
	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Fatalf("Could not load file from standing up acc infra: %s", err.Error())
	}

	contents, err := Render(k3s_server_tpl+k3s_agent_tpl, map[string]string{
		"server_host": inputs.Nodes[1],
		"agent_host":  inputs.Nodes[2],
		"user":        inputs.User,
		"private_key": inputs.SshKey,
	})

	if err != nil {
		t.Fatalf("Could not load file from standing up acc infra: %s", err.Error())
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + contents,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectSensitiveValue(
					"k3s_server.main",
					tfjsonpath.New("token"),
				),
				statecheck.ExpectKnownValue(
					"k3s_server.main",
					tfjsonpath.New("active"),
					knownvalue.Bool(true),
				),
				statecheck.ExpectKnownValue(
					"k3s_agent.main",
					tfjsonpath.New("active"),
					knownvalue.Bool(true),
				),
			},
		},
		},
	})
}
