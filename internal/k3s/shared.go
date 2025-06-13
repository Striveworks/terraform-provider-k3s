package k3s

import (
	"fmt"
	"regexp"

	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const DATA_DIR string = "/var/lib/rancher/k3s"
const CONFIG_DIR string = "/etc/rancher/k3s"

type K3sComponent interface {
	RunPreReqs(ssh_client.SSHClient) error
	RunInstall(ssh_client.SSHClient) error
	RunUninstall(ssh_client.SSHClient) error
	Status(ssh_client.SSHClient) (bool, error)
	StatusLog(ssh_client.SSHClient) (string, error)
	Journal(ssh_client.SSHClient) (string, error)
}

func systemdStatus(unit string, client ssh_client.SSHClient) (bool, error) {
	res, err := client.Run(fmt.Sprintf("sudo systemctl is-active %s", unit))
	if err != nil {
		return false, err
	}

	if len(res) != 1 {
		return false, fmt.Errorf("wrong number of results from server status check")
	}

	active := regexp.MustCompile(`\s+`).ReplaceAllString(res[0], "")
	return (active == "active"), nil

}
