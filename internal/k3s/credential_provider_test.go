package k3s

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

func TestECRHelperScript(t *testing.T) {
	script, err := assets.ReadFile("assets/ecr-helper.sh")
	if err != nil {
		t.Fatalf("reading ecr helper: %v", err)
	}

	for _, want := range []string{
		"curl -fsSL -o \"$BIN_PATH\" \"$ECR_CREDENTIAL_PROVIDER_URL\"",
		"${ECR_CREDENTIAL_PROVIDER_CHECKSUM}  ${BIN_PATH}",
		"sha256sum -c -",
		"restorecon -v \"$BIN_PATH\"",
		"CONFIG_PATH=/etc/rancher/k3s/credential-provider-config.yaml",
		"AWS_EC2_METADATA_DISABLED",
		"allow perm=any all : path=${BIN_PATH}",
		"fagenrules --load",
		"systemctl try-restart fapolicyd",
		"rm -f \"$BIN_PATH\" \"$CONFIG_PATH\"",
		"RULES_PATH=/etc/fapolicyd/rules.d/81-ecr-credential-provider.rules",
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("ecr helper missing %q", want)
		}
	}

	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(string(script))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s", err, out)
	}
}

func TestECRHelperCommands(t *testing.T) {
	install := newECRInstall(schemas.EcrCredentialProvider{
		RoleARN: types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
	})

	commands, err := ecrHelperCommands("/usr/local/bin", &install)
	if err != nil {
		t.Fatalf("ecrHelperCommands() error = %v", err)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"sudo mkdir -p /usr/local/bin",
		"sudo tee /usr/local/bin/ecr-helper.sh.tmp",
		"sudo base64 -d /usr/local/bin/ecr-helper.sh.tmp | sudo tee /usr/local/bin/ecr-helper.sh",
		"ECR_HELPER_ACTION=install",
		"ECR_CREDENTIAL_PROVIDER_URL=" + shellQuote(schemas.DefaultECRCredentialProviderURL),
		"ECR_CREDENTIAL_PROVIDER_CHECKSUM=" + shellQuote(schemas.DefaultECRCredentialProviderChecksum),
		"ECR_ROLE_ARN=" + shellQuote("arn:aws:iam::123456789012:role/ecr-reader"),
		"ECR_TOKEN_FILE=" + shellQuote(schemas.DefaultECRCredentialProviderToken),
		"bash /usr/local/bin/ecr-helper.sh",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("install commands missing %q\n%s", want, joined)
		}
	}

	cleanup, err := ecrHelperCommands("/opt/bin", nil)
	if err != nil {
		t.Fatalf("ecrHelperCommands() cleanup error = %v", err)
	}
	cleanupJoined := strings.Join(cleanup, "\n")
	if !strings.Contains(cleanupJoined, "sudo ECR_HELPER_ACTION=cleanup bash /opt/bin/ecr-helper.sh") {
		t.Errorf("cleanup commands = %q", cleanupJoined)
	}
}

func TestServerWithECRCredentialProvider(t *testing.T) {
	server := Server{
		Config: "kubelet-arg:\n  - \"eviction-hard=memory.available<100Mi\"\n",
	}
	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithECRCredentialProvider(schemas.EcrCredentialProvider{
		RoleARN:   types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
		TokenFile: types.StringValue("/var/run/ecr/token"),
	})

	args := kubeletArgs(server.config["kubelet-arg"])
	got := strings.Join(args, "\n")
	for _, want := range []string{
		"eviction-hard=memory.available<100Mi",
		"image-credential-provider-config=/etc/rancher/k3s/credential-provider-config.yaml",
		"image-credential-provider-bin-dir=/usr/local/libexec",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("kubelet-arg = %q, missing %q", got, want)
		}
	}

	server.WithECRCredentialProvider(schemas.EcrCredentialProvider{
		RoleARN: types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
	})
	args = kubeletArgs(server.config["kubelet-arg"])
	joined := strings.Join(args, "\n")
	if strings.Count(joined, "image-credential-provider-config=") != 1 {
		t.Errorf("kubelet-arg duplicated credential provider config: %q", joined)
	}
	if strings.Count(joined, "image-credential-provider-bin-dir=") != 1 {
		t.Errorf("kubelet-arg duplicated credential provider bin dir: %q", joined)
	}
}

func TestAgentWithECRCredentialProvider(t *testing.T) {
	agent := Agent{Config: "{}\n"}
	if err := agent.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	agent.WithECRCredentialProvider(schemas.EcrCredentialProvider{
		RoleARN: types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
	})

	args := kubeletArgs(agent.config["kubelet-arg"])
	if len(args) != 2 {
		t.Fatalf("kubelet-arg = %#v, want 2 arguments", args)
	}
}

func TestECRServerManifest(t *testing.T) {
	install := newECRInstall(schemas.EcrCredentialProvider{
		RoleARN:   types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
		TokenFile: types.StringValue("/var/run/ecr/my-token"),
		Image:     types.StringValue("example.com/token-sync:1"),
	})

	commands, err := ecrServerManifestCommands("/usr/local/bin", "/var/lib/rancher/k3s", &install)
	if err != nil {
		t.Fatalf("ecrServerManifestCommands() error = %v", err)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"sudo mkdir -p /usr/local/bin /var/lib/rancher/k3s/server/manifests",
		"sudo tee /usr/local/bin/ecr-helper.yaml.tmp",
		"sudo cat /usr/local/bin/ecr-helper.yaml",
		"ECR_READER_ROLE_ARN=" + shellQuote("arn:aws:iam::123456789012:role/ecr-reader"),
		"ECR_READER_IMAGE=" + shellQuote("example.com/token-sync:1"),
		"ECR_TOKEN_DIR=" + shellQuote("/var/run/ecr"),
		"ECR_TOKEN_NAME=" + shellQuote("my-token"),
		"envsubst " + shellQuote(ecrManifestEnvsubst),
		"sudo tee /var/lib/rancher/k3s/server/manifests/ecr-helper.yaml",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("manifest commands missing %q\n%s", want, joined)
		}
	}

	cleanup := ecrRemoveManifestCommands("/usr/local/bin", "/var/lib/rancher/k3s", true)
	cleanupJoined := strings.Join(cleanup, "\n")
	for _, want := range []string{
		"sudo test ! -f /var/lib/rancher/k3s/server/manifests/ecr-helper.yaml || sudo /usr/local/bin/k3s kubectl delete -f /var/lib/rancher/k3s/server/manifests/ecr-helper.yaml --ignore-not-found",
		"sudo rm -f /var/lib/rancher/k3s/server/manifests/ecr-helper.yaml /usr/local/bin/ecr-helper.yaml",
	} {
		if !strings.Contains(cleanupJoined, want) {
			t.Errorf("cleanup commands missing %q\n%s", want, cleanupJoined)
		}
	}
}

func TestECRManifestEnvsubst(t *testing.T) {
	if _, err := exec.LookPath("envsubst"); err != nil {
		t.Skip("envsubst not installed")
	}

	raw, err := assets.ReadFile("assets/ecr-helper.yaml")
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	cmd := exec.Command("envsubst", ecrManifestEnvsubst)
	cmd.Env = append(os.Environ(),
		"ECR_READER_ROLE_ARN=arn:aws:iam::123456789012:role/ecr-reader",
		"ECR_READER_IMAGE=example.com/token-sync:1",
		"ECR_TOKEN_DIR=/var/run/ecr",
		"ECR_TOKEN_NAME=my-token",
		"tmp=should-not-replace",
	)
	cmd.Stdin = bytes.NewReader(raw)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("envsubst: %v\n%s", err, out)
	}

	rendered := string(out)
	for _, want := range []string{
		"arn:aws:iam::123456789012:role/ecr-reader",
		"example.com/token-sync:1",
		`path: "/var/run/ecr"`,
		"/host/my-token",
		`"$tmp"`,
		"runAsUser: 0",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered manifest missing %q\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "${ECR_") || strings.Contains(rendered, "should-not-replace") {
		t.Errorf("rendered manifest substituted the wrong variables:\n%s", rendered)
	}
}
