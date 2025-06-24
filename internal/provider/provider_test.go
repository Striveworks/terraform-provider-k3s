// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
	"text/template"

	"github.com/caarlos0/env"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (

	// providerConfig is a shared configuration to combine with the actual
	// test configuration so the Inventory client is properly configured.
	providerConfig = `provider "k3s" {}`
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"k3s": providerserver.NewProtocol6WithError(New("")()),
}

func testAccPreCheck(t *testing.T) {}

type AccTestInputs struct {
	Nodes  []string `json:"nodes,omitempty" env:"TF_TEST_NODES"`
	SshKey string   `json:"ssh_key,omitempty" env:"TF_TEST_SSH_KEY"`
	User   string   `json:"user,omitempty" env:"TF_TEST_USER"`
}

func NewAccTestInputs() (AccTestInputs, error) {
	output := AccTestInputs{}
	if json_file, ok := os.LookupEnv("TEST_JSON_PATH"); ok {
		file, err := os.Open(json_file)
		if err != nil {
			return output, err
		}

		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			return output, err
		}
		if err = json.Unmarshal(fileBytes, &output); err != nil {
			return output, err
		}
	}

	err := env.Parse(&output)

	return output, err
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
