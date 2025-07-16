package handlers_test

import "striveworks.us/terraform-provider-k3s/internal/ssh_client"

type mockSSH struct {
	host        string
	hostname    string
	hostnameErr error
	file        string
	fileErr     error
	run         []string
	runErr      error
	streamErr   error
	waitErr     error

	ranCommands    []string
	streamCommands []string
}

// Host implements ssh_client.SSHClient.
func (m *mockSSH) Host() string {
	return m.host
}

// Hostname implements ssh_client.SSHClient.
func (m *mockSSH) Hostname() (string, error) {
	return m.hostname, m.hostnameErr
}

// HostnameOrIpAddress implements ssh_client.SSHClient.
func (m *mockSSH) HostnameOrIpAddress() string {
	return m.hostname
}

// ReadFile implements ssh_client.SSHClient.
func (m *mockSSH) ReadFile(path string, missingOk bool, sudo bool) (string, error) {
	return m.file, m.fileErr
}

// Run implements ssh_client.SSHClient.
func (m *mockSSH) Run(commands ...string) ([]string, error) {
	m.ranCommands = append(m.ranCommands, commands...)
	return m.run, m.runErr
}

// RunStream implements ssh_client.SSHClient.
func (m *mockSSH) RunStream(commands []string) error {
	m.streamCommands = append(m.streamCommands, commands...)
	return m.streamErr
}

// WaitForReady implements ssh_client.SSHClient.
func (m *mockSSH) WaitForReady() error {
	return m.waitErr
}

var _ ssh_client.SSHClient = &mockSSH{}
