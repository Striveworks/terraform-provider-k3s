package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var (
	k3s_server_tpl = `
resource "k3s_server" "main" {
	host        = "{{ .server_host }}"
	user        = "{{ .user }}"
	private_key =<<EOT
{{ .private_key }}
EOT
}`
)

func TestAccK3sServerResource(t *testing.T) {
	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Fatalf("Could not load file from standing up acc infra: %s", err.Error())
	}
	contents, err := Render(k3s_server_tpl, map[string]string{
		"server_host": inputs.Nodes[0],
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
			},
		}, {
			Config:  providerConfig + contents,
			Destroy: true,
		}},
	})
}
