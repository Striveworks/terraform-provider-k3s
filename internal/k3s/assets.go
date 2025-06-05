// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import (
	"bytes"
	"embed"
	"encoding/base64"
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

func ReadSystemDSingle(path string) (string, error) {
	// Default ExtraServerArgs to empty slice if nil

	raw, err := assets.ReadFile("assets/k3s-single.service.tpl")
	if err != nil {
		return "", err
	}
	tpl, err := template.New("k3s-single.service").Parse(string(raw))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, struct {
		ConfigPath string
	}{
		ConfigPath: path,
	}); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
