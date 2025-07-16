package provider_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	provider "striveworks.us/terraform-provider-k3s/internal/provider"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const (

	// providerConfig is a shared configuration to combine with the actual
	// test configuration so the Inventory client is properly configured.
	providerConfig = `
provider "k3s" {}
`
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"k3s": providerserver.NewProtocol6WithError(provider.NewDebugMode("")()),
}

type StandupInputs struct {
	Nodes  []string `json:"nodes"`
	SshKey string   `json:"ssh_key"`
	User   string   `json:"user"`
}

func (s StandupInputs) AgentTests() StandupInputs {
	return StandupInputs{
		s.Nodes[0:3],
		s.SshKey,
		s.User,
	}
}

func (s StandupInputs) ServerTests() StandupInputs {
	return StandupInputs{
		s.Nodes[3:len(s.Nodes)],
		s.SshKey,
		s.User,
	}
}

func LoadInputs(f string) (StandupInputs, error) {
	var output StandupInputs
	file, err := os.Open(f)
	if err != nil {
		return output, err
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return output, err
	}
	err = json.Unmarshal(fileBytes, &output)

	return output, err
}

func (s *StandupInputs) SshClient(t *testing.T, index uint) (ssh_client.SSHClient, error) {
	return ssh_client.NewSSHClient(t.Context(), s.Nodes[index], 22, s.User, s.SshKey, "")
}

func Render(raw string, args map[string]string) (string, error) {
	tpl, err := template.New("tpl").Parse(raw)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, args); err != nil {
		return "", err
	}
	return buf.String(), nil
}
