package k3s

import (
	"fmt"
	"path"
	"strings"

	"striveworks.us/terraform-provider-k3s/internal/schemas"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const (
	ecrCredentialProviderBinDir = "/usr/local/libexec"
	ecrCredentialProviderConfig = "/etc/rancher/k3s/credential-provider-config.yaml"
	ecrKubeletConfigArg         = "image-credential-provider-config=" + ecrCredentialProviderConfig
	ecrKubeletBinArg            = "image-credential-provider-bin-dir=" + ecrCredentialProviderBinDir
	ecrHelperScript             = "ecr-helper.sh"
	ecrHelperManifest           = "ecr-helper.yaml"
	ecrManifestDir              = "/server/manifests"
	ecrManifestEnvsubst         = "${ECR_READER_ROLE_ARN}${ECR_READER_IMAGE}${ECR_TOKEN_DIR}${ECR_TOKEN_NAME}"
)

type ecrInstall struct {
	RoleARN   string
	URL       string
	Checksum  string
	TokenFile string
	Image     string
}

func newECRInstall(cfg schemas.EcrCredentialProvider) ecrInstall {
	install := ecrInstall{
		RoleARN:   strings.TrimSpace(cfg.RoleARN.ValueString()),
		URL:       strings.TrimSpace(cfg.URL.ValueString()),
		Checksum:  strings.TrimSpace(cfg.Checksum.ValueString()),
		TokenFile: strings.TrimSpace(cfg.TokenFile.ValueString()),
		Image:     strings.TrimSpace(cfg.Image.ValueString()),
	}
	if install.URL == "" {
		install.URL = schemas.DefaultECRCredentialProviderURL
	}
	if install.Checksum == "" {
		install.Checksum = schemas.DefaultECRCredentialProviderChecksum
	}
	if install.TokenFile == "" {
		install.TokenFile = schemas.DefaultECRCredentialProviderToken
	}
	if install.Image == "" {
		install.Image = schemas.DefaultECRCredenialIRSAImage
	}
	return install
}

func ecrHelperCommands(binDir string, install *ecrInstall) ([]string, error) {
	contents, err := ReadECRHelperScript()
	if err != nil {
		return nil, err
	}
	if binDir == "" {
		binDir = BIN_DIR
	}

	commands := []string{fmt.Sprintf("sudo mkdir -p %s", binDir)}
	commands = append(commands, WriteFileCommands(binDir+"/"+ecrHelperScript, contents)...)
	commands = append(commands, ecrCommand(binDir, install))
	return commands, nil
}

func (e ecrInstall) command(binDir string) string {
	flags := []string{
		"ECR_HELPER_ACTION=install",
		"ECR_CREDENTIAL_PROVIDER_URL=" + shellQuote(e.URL),
		"ECR_CREDENTIAL_PROVIDER_CHECKSUM=" + shellQuote(e.Checksum),
		"ECR_ROLE_ARN=" + shellQuote(e.RoleARN),
		"ECR_TOKEN_FILE=" + shellQuote(e.TokenFile),
	}
	return fmt.Sprintf("sudo %s bash %s/%s", strings.Join(flags, " "), binDir, ecrHelperScript)
}

func ecrCommand(binDir string, install *ecrInstall) string {
	if install == nil {
		return ecrCleanupCommand(binDir)
	}
	return install.command(binDir)
}

func ecrCleanupCommand(binDir string) string {
	if binDir == "" {
		binDir = BIN_DIR
	}
	return fmt.Sprintf("sudo ECR_HELPER_ACTION=cleanup bash %s/%s", binDir, ecrHelperScript)
}

func ecrServerManifestCommands(binDir, dataDir string, install *ecrInstall) ([]string, error) {
	if install == nil {
		return ecrRemoveManifestCommands(binDir, dataDir, true), nil
	}
	return ecrInstallManifestCommands(binDir, dataDir, install)
}

func ecrInstallManifestCommands(binDir, dataDir string, install *ecrInstall) ([]string, error) {
	contents, err := ReadECRHelperManifest()
	if err != nil {
		return nil, err
	}
	binDir, dataDir = ecrManifestPaths(binDir, dataDir)
	templatePath := binDir + "/" + ecrHelperManifest
	manifestPath := dataDir + ecrManifestDir + "/" + ecrHelperManifest

	commands := []string{fmt.Sprintf("sudo mkdir -p %s %s", binDir, dataDir+ecrManifestDir)}
	commands = append(commands, WriteFileCommands(templatePath, contents)...)
	commands = append(commands, install.envsubstCommand(templatePath, manifestPath))
	commands = append(commands, fmt.Sprintf("sudo chown root:root %s", manifestPath))
	return commands, nil
}

func (e ecrInstall) envsubstCommand(templatePath, manifestPath string) string {
	env := []string{
		"ECR_READER_ROLE_ARN=" + shellQuote(e.RoleARN),
		"ECR_READER_IMAGE=" + shellQuote(e.Image),
		"ECR_TOKEN_DIR=" + shellQuote(path.Dir(e.TokenFile)),
		"ECR_TOKEN_NAME=" + shellQuote(path.Base(e.TokenFile)),
	}
	// Limit envsubst to these variables so "$tmp" in the pod script is preserved.
	return fmt.Sprintf(
		"sudo cat %s | sudo %s envsubst %s | sudo tee %s > /dev/null",
		templatePath,
		strings.Join(env, " "),
		shellQuote(ecrManifestEnvsubst),
		manifestPath,
	)
}

func ecrRemoveManifestCommands(binDir, dataDir string, deleteResources bool) []string {
	binDir, dataDir = ecrManifestPaths(binDir, dataDir)
	manifestPath := dataDir + ecrManifestDir + "/" + ecrHelperManifest
	templatePath := binDir + "/" + ecrHelperManifest

	var commands []string
	if deleteResources {
		// Removing the file does not delete the AddOn resources. Delete them while k3s is still up.
		commands = append(commands, fmt.Sprintf(
			"sudo test ! -f %[1]s || sudo %[2]s/k3s kubectl delete -f %[1]s --ignore-not-found",
			manifestPath,
			binDir,
		))
	}
	commands = append(commands, fmt.Sprintf("sudo rm -f %s %s", manifestPath, templatePath))
	return commands
}

func ecrManifestPaths(binDir, dataDir string) (string, string) {
	if binDir == "" {
		binDir = BIN_DIR
	}
	if dataDir == "" {
		dataDir = DATA_DIR
	}
	return binDir, dataDir
}

func runECRHelper(client ssh_client.SSHClient, binDir string, install *ecrInstall) error {
	commands, err := ecrHelperCommands(binDir, install)
	if err != nil {
		return err
	}
	return client.RunStream(commands)
}

func appendECRKubeletArgs(config map[any]any) {
	args := kubeletArgs(config["kubelet-arg"])
	args = upsertKubeletArg(args, "image-credential-provider-config=", ecrKubeletConfigArg)
	args = upsertKubeletArg(args, "image-credential-provider-bin-dir=", ecrKubeletBinArg)
	config["kubelet-arg"] = args
}

func kubeletArgs(raw any) []string {
	switch value := raw.(type) {
	case nil:
		return nil
	case []string:
		return append([]string{}, value...)
	case string:
		if value == "" {
			return nil
		}
		return []string{value}
	case []any:
		args := make([]string, 0, len(value))
		for _, item := range value {
			args = append(args, fmt.Sprint(item))
		}
		return args
	default:
		return []string{fmt.Sprint(value)}
	}
}

func upsertKubeletArg(args []string, prefix string, value string) []string {
	filtered := make([]string, 0, len(args)+1)
	for _, arg := range args {
		if arg == value || strings.HasPrefix(arg, prefix) {
			continue
		}
		filtered = append(filtered, arg)
	}
	return append(filtered, value)
}
