package ssh_client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/crypto/ssh"
)

func NewSSHClient(ctx context.Context, config SSHConfig) (SSHClient, error) {
	auths := make([]ssh.AuthMethod, 0)
	password := config.Password.ValueString()
	if password != "" {
		ctx = tflog.MaskLogStrings(ctx, password)
		auths = append(auths, ssh.Password(password))
	}

	privateKey := config.PrivateKey.ValueString()
	if privateKey != "" {
		ctx = tflog.MaskLogStrings(ctx, privateKey)
		signer, err := signerFromPem([]byte(privateKey))
		if err != nil {
			return SSHClient{}, err
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	if config.PrivateKeyFile.ValueString() != "" {
		key, err := os.ReadFile(config.PrivateKeyFile.ValueString())
		if err != nil {
			return SSHClient{}, fmt.Errorf("cannot read private key file: %w", err)
		}
		signer, err := signerFromPem(key)
		if err != nil {
			return SSHClient{}, fmt.Errorf("cannot parse private key file: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	Config := ssh.ClientConfig{
		User: config.User.ValueString(),
		Auth: auths,
	}
	if config.IgnoreHostKeyVerification.ValueBool() {
		Config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	tflog.Info(ctx, fmt.Sprintf("Using auth against %s", config.Host))
	return SSHClient{
		ctx:                 ctx,
		HostnameOrIPAddress: config.Host.ValueString(),
		Port:                int(config.Port.ValueInt32()),
		Config:              Config,
	}, nil
}

type SSHClient struct {
	HostnameOrIPAddress string
	Config              ssh.ClientConfig
	Port                int

	ctx context.Context
}

func (s *SSHClient) Hostname() (hostname string, err error) {
	hostname, err = s.runSingle("sudo hostname")
	if err != nil {
		return
	}

	hostname = regexp.MustCompile(`\s+`).ReplaceAllString(hostname, "")
	return
}

func (s *SSHClient) Host() string {
	return fmt.Sprintf("%s:%d", s.HostnameOrIPAddress, s.Port)
}

// Runs a set of commands, gathering their output into
// a list of outputs.
func (s *SSHClient) Run(commands ...string) (results []string, err error) {
	// Start the command
	for _, cmd := range commands {
		result, err := s.runSingle(cmd)
		if err != nil {
			return results, fmt.Errorf("cannot start cmd '%s': %s", cmd, err)
		}
		tflog.Debug(s.ctx, fmt.Sprintf("Running bash command: %v with result: %v", cmd, result))
		results = append(results, result)
	}

	return
}

func (s *SSHClient) runSingle(command string) (result string, err error) {
	client, err := ssh.Dial("tcp", s.Host(), &s.Config)
	if err != nil {
		return result, fmt.Errorf("create client failed %v", err)
	}
	defer client.Close()

	// open session
	session, err := client.NewSession()
	if err != nil {
		return result, fmt.Errorf("create session failed %v", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(command)
	if err != nil {
		return result, fmt.Errorf("cannot start cmd '%s': %s", command, err)
	}
	result = string(out)

	return
}

// Runs a set of commands, streaming their output to a callbacks
// Callbacks will be (stdout, stderr) or (stdout + stderr,).
func (s *SSHClient) RunStream(commands []string) (err error) {
	for _, cmd := range commands {
		if err = s.streamSingle(cmd); err != nil {
			return
		}
	}
	return
}

func (s *SSHClient) streamSingle(command string) error {
	client, err := ssh.Dial("tcp", s.Host(), &s.Config)
	if err != nil {
		return fmt.Errorf("create client failed %v", err)
	}
	defer client.Close()

	// open session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create session failed %v", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot open stdout pipe for cmd '%s': %s", command, err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot open stderr pipe for cmd '%s': %s", command, err)
	}

	// Start the commands
	tflog.Debug(s.ctx, fmt.Sprintf("Running ssh command %s", command))
	if err := session.Start(command); err != nil {
		return fmt.Errorf("cannot start cmd '%s': %s", command, err)
	}

	errChan := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go s.logPipe(stdout, "[STDOUT]", &wg, errChan)
	go s.logPipe(stderr, "[STDERR]", &wg, errChan)

	// Wait for both output streams to finish
	wg.Wait()

	// Wait for the command to finish
	if err := session.Wait(); err != nil {
		tflog.Error(s.ctx, fmt.Sprintf("cannot run cmd '%s': %s", command, err))
		return fmt.Errorf("cannot run cmd, see error logs") // Mask error command to prevent secret leakage
	}

	return nil
}

func (s *SSHClient) logPipe(pipe io.Reader, prefix string, wg *sync.WaitGroup, errChan chan<- error) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		tflog.Debug(s.ctx, fmt.Sprintf("%s %s", prefix, line))
	}

	// Send the error to the channel (could be nil, which is fine)
	if err := scanner.Err(); err != nil {
		errChan <- fmt.Errorf("%s: %w", prefix, err)
	}
}

// Waits for the server to be ready.
func (s *SSHClient) WaitForReady() error {
	maxRetries := 10
	for i := range maxRetries {
		client, err := ssh.Dial("tcp", s.Host(), &s.Config)
		if err == nil {
			client.Close()
			break
		} else {
			tflog.Warn(s.ctx, fmt.Sprintf("While waiting for ssh to be ready %s", err.Error()))
		}
		if i == maxRetries-1 {
			return fmt.Errorf("SSH not ready after %d attempts: %v", maxRetries, err)
		}
		tflog.Info(s.ctx, fmt.Sprintf("Waiting for SSH to be ready... (%d/%d)", i+1, maxRetries))
		time.Sleep(5 * time.Second)
	}

	return nil
}

func (s *SSHClient) ReadFile(path string, missingOk bool, sudo bool) (string, error) {
	command := fmt.Sprintf("cat %s", path)
	if sudo {
		command = fmt.Sprintf("sudo %s", command)
	}
	if missingOk {
		command = fmt.Sprintf("sudo [ -f %s ] && %s || echo ''", path, command)
	}

	result, err := s.Run(command)
	if err != nil {
		return "", err
	}

	return result[0], nil
}

func (s *SSHClient) ReadOptionalFile(path string, sudo ...bool) (string, error) {
	return s.ReadFile(path, true, len(sudo) > 0 && sudo[0])
}

func signerFromPem(pemBytes []byte) (ssh.Signer, error) {
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		// Check if the error is due to a password-protected key, which is not supported yet.
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			return nil, fmt.Errorf("parsing private key failed: password-protected keys are not supported: %w", err)
		}
		return nil, fmt.Errorf("parsing private key failed: %w", err)
	}
	return signer, nil
}
