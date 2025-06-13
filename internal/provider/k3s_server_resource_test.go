package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccK3sServerResource(t *testing.T) {
	inputs, err := LoadInputs(os.Getenv("TEST_JSON_PATH"))
	if err != nil {
		t.Fatalf("Could not load file from standing up acc infra: %s", err.Error())
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "k3s_server" "main" {
  host        = "%s"
  user        = "%s"
  private_key =<<EOT
%s
EOT
}
`, inputs.Nodes[0], inputs.User, inputs.SshKey),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectSensitiveValue(
					"k3s_server.server",
					tfjsonpath.New("token"),
				),
			},
		},
		},
	})

}
