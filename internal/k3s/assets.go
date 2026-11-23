package k3s

import (
	"embed"
	"encoding/base64"
	"fmt"
)

//go:embed assets/*
var assets embed.FS

// The install script.
func ReadInstallScript() (string, error) {
	return readAssetBase64("assets/k3s-install.sh")
}

// The ECR credential provider helper script.
func ReadECRHelperScript() (string, error) {
	return readAssetBase64("assets/ecr-helper.sh")
}

// The ECR credential provider manifest template.
func ReadECRHelperManifest() (string, error) {
	return readAssetBase64("assets/ecr-helper.yaml")
}

func readAssetBase64(name string) (string, error) {
	script, err := assets.ReadFile(name)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(script), nil
}

// Generate a list of commands to write a base64 encoded string
// to a remote file and chown to root.
func WriteFileCommands(path string, b64Content string) []string {
	return []string{
		fmt.Sprintf("echo %q | sudo tee %s.tmp > /dev/null", b64Content, path),
		fmt.Sprintf("sudo base64 -d %[1]s.tmp | sudo tee %[1]s > /dev/null", path),
		fmt.Sprintf("sudo chown root:root %s", path),
		fmt.Sprintf("sudo rm %s.tmp", path),
	}
}
