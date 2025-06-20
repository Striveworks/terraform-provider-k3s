// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"text/template"
)

//go:embed assets/*
var assets embed.FS

func ReadInstallScript() (string, error) {
	script, err := assets.ReadFile("assets/k3s-install.sh")
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(script), nil
}

func tplSystemD(path string, filename string, binDir string) (string, error) {
	raw, err := assets.ReadFile(fmt.Sprintf("assets/%s.tpl", filename))
	if err != nil {
		return "", err
	}
	tpl, err := template.New(filename).Parse(string(raw))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, struct {
		ConfigPath string
		BinDir     string
	}{
		ConfigPath: path,
		BinDir:     binDir,
	}); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Returns the systemd file templated correctly as a b64 encoded string.
func ReadSystemDSingleServer(path string, binDir string) (string, error) {
	return tplSystemD(path, "k3s-single.service", binDir)
}

// Returns the systemd file templated correctly as a b64 encoded string.
func ReadSystemDSingleAgent(path string, binDir string) (string, error) {
	return tplSystemD(path, "k3s-agent.service", binDir)
}
