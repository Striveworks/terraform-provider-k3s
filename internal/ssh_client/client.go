package ssh_client

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

func NewSSHClient(addr string, user string, pem string) (SSHClient, error) {
	signer, err := signerFromPem([]byte(pem), []byte{})
	if err != nil {
		return nil, err
	}

	return &sshClient{host: addr,
		config: ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			// In production, implement proper host key verification
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}}, nil
}

type SSHClient interface {
	// Runs a set of commands, gathering their output into
	// a list of outputs
	Run(commands ...string) ([]string, error)
	// Runs a set of commands, streaming their output to a callbacks
	// Callbacks will be (stdout, stderr) or (stdout + stderr,)
	RunStream(commands []string, callbacks ...func(string)) error
	// Waits for the server to be ready
	WaitForReady(logger func(string)) error
	// Host name/address
	Host() string
}

var _ SSHClient = &sshClient{}

type sshClient struct {
	config ssh.ClientConfig
	host   string
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
func (s *sshClient) RunStream(commands []string, callback ...func(string)) (err error) {
	for _, cmd := range commands {
		if err = s.streamSingle(cmd, callback...); err != nil {
			return
		}
	}
	return
}

func (s *sshClient) streamSingle(command string, callback ...func(string)) error {
	stdoutFunc := func(string) {}
	stderrFunc := func(string) {}

	switch len(callback) {
	case 2:
		stdoutFunc = callback[0]
		stderrFunc = callback[1]
	case 1:
		stderrFunc = callback[0]
		stdoutFunc = callback[0]
	default:
		break
	}

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

	if err := session.Start(command); err != nil {
		return fmt.Errorf("cannot start cmd '%s': %s", command, err)
	}

	done := make(chan struct{}, 2)

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutFunc(line)
		}
		done <- struct{}{}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrFunc(line)
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

func (s *sshClient) WaitForReady(logger func(string)) error {
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		client, err := ssh.Dial("tcp", s.host, &s.config)
		if err == nil {
			client.Close()
			break
		}
		if i == maxRetries-1 {
			return fmt.Errorf("SSH not ready after %d attempts: %v", maxRetries, err)
		}
		logger(fmt.Sprintf("Waiting for SSH to be ready... (%d/%d)", i+1, maxRetries))
		time.Sleep(5 * time.Second)
	}

	return nil
}

func signerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {
	// read pem block
	err := errors.New("pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	// handle encrypted key
	// Note(npapa): Do we want to?
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// decrypt PEM
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
		if err != nil {
			return nil, fmt.Errorf("decrypting PEM block failed %v", err)
		}

		// get RSA, EC or DSA key
		key, err := parsePemBlock(pemBlock)
		if err != nil {
			return nil, err
		}

		// generate signer instance from key
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return nil, fmt.Errorf("creating signer from encrypted key failed %v", err)
		}

		return signer, nil
	} else {
		// generate signer instance from plain key
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("parsing plain private key failed %v", err)
		}

		return signer, nil
	}
}

func parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("parsing private key failed, unsupported key type %q", block.Type)
	}
}
