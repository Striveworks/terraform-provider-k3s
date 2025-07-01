package ssh_client

import (
	"bufio"
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	sftp "github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"
)

func NewSSHClient(ctx context.Context, addr string, user string, pem string, password string) (SSHClient, error) {

	var auth ssh.AuthMethod
	if pem != "" {
		tflog.Debug(ctx, "Using pem key auth")
		signer, err := signerFromPem([]byte(pem))
		if err != nil {
			return nil, err
		}
		auth = ssh.PublicKeys(signer)
	} else {
		tflog.Debug(ctx, "Using password auth")
		auth = ssh.Password(password)
	}

	tflog.Info(ctx, fmt.Sprintf("Using auth against %s", addr))
	return &sshClient{
		ctx:  ctx,
		host: addr,
		config: ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				auth,
			},
			// In production, implement proper host key verification
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			// Timeout:         60,
		}}, nil
}

type SSHClient interface {
	// Runs a set of commands, gathering their output into
	// a list of outputs
	Run(commands ...string) ([]string, error)
	// Runs a set of commands, streaming their output to a callbacks
	// Callbacks will be (stdout, stderr) or (stdout + stderr,)
	RunStream(commands []string) error
	// Waits for the server to be ready
	WaitForReady() error
	// Host name/address
	Host() string
	// Reads file from remote path
	ReadFile(path string, missingOk ...bool) (string, error)
}

var _ SSHClient = &sshClient{}

type sshClient struct {
	config ssh.ClientConfig
	host   string
	ctx    context.Context
}

func (s *sshClient) Host() string {
	return s.host
}

func (s *sshClient) Run(commands ...string) (results []string, err error) {

	// Start the command
	for _, cmd := range commands {
		result, err := s.runSingle(cmd)
		if err != nil {
			return results, fmt.Errorf("cannot start cmd '%s': %s", cmd, err)
		}
		results = append(results, result)
	}

	return
}

func (s *sshClient) runSingle(command string) (result string, err error) {
	client, err := ssh.Dial("tcp", s.host, &s.config)
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

// RunStream implements SSHClient.
func (s *sshClient) RunStream(commands []string) (err error) {
	for _, cmd := range commands {
		if err = s.streamSingle(cmd); err != nil {
			return
		}
	}
	return
}

func (s *sshClient) streamSingle(command string) error {
	client, err := ssh.Dial("tcp", s.host, &s.config)
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

	done := make(chan struct{}, 2)

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			tflog.Debug(s.ctx, fmt.Sprintf("[STDOUT] %s", line))
		}
		done <- struct{}{}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			tflog.Debug(s.ctx, fmt.Sprintf("[STDERR] %s", line))
		}
		done <- struct{}{}
	}()

	// Wait for both output streams to finish
	<-done
	<-done

	// Wait for the command to finish
	if err := session.Wait(); err != nil {
		return fmt.Errorf("cannot run cmd '%s': %s", command, err)
	}

	return nil
}

func (s *sshClient) WaitForReady() error {
	maxRetries := 10
	for i := range maxRetries {
		client, err := ssh.Dial("tcp", s.host, &s.config)
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

func (s *sshClient) ReadFile(path string, missingOk ...bool) (string, error) {

	skipMissing := false
	if len(missingOk) > 0 {
		skipMissing = missingOk[0]
	}

	client, err := ssh.Dial("tcp", s.host, &s.config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	sftpclient, err := sftp.NewClient(client)
	if err != nil {
		return "", err
	}
	defer sftpclient.Close()

	file, err := sftpclient.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && skipMissing {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	buf := new(bufio.Reader)
	buf.Reset(file)
	contents, err := buf.ReadString(0)
	if err != nil && err.Error() != "EOF" {
		return "", err
	}
	return contents, nil

}

func signerFromPem(pemBytes []byte) (ssh.Signer, error) {
	err := errors.New("pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing plain private key failed %v", err)
	}

	return signer, nil
}
