// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"

	"k8s.io/client-go/tools/clientcmd"
)

type K3sAuth interface {
	SshUser() string
	Pem() string
	Address() string
}

type K3sClient struct {
	config *ssh.ClientConfig
	host   string
}

func NewK3sClient(auth K3sAuth) (*K3sClient, error) {
	signer, err := signerFromPem([]byte(auth.Pem()), []byte{})
	if err != nil {
		return nil, err
	}
	config := ssh.ClientConfig{
		User: auth.SshUser(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, implement proper host key verification
	}

	return &K3sClient{
		host:   auth.Address(),
		config: &config,
	}, nil
}

func (s *K3sClient) Prereqs(outputStream func(string)) error {
	return s.runSSHWithCallback("sudo apt-get update && sudo apt-get install -y curl", outputStream)
}

func (s *K3sClient) InstallServer(outputStream func(string)) error {
	return s.runSSHWithCallback("curl -sfL https://get.k3s.io | sudo sh -", outputStream)
}

// Deletes the server
func (s *K3sClient) DeleteServer(outputStream func(string)) error {
	return s.runSSHWithCallback("sudo bash /usr/local/bin/k3s-uninstall.sh", outputStream)
}

// Deletes the server
func (s *K3sClient) IsActive() (bool, error) {
	var active bool
	err := s.runSSHWithCallback("sudo systemctl is-active k3s", func(out string) {
		active = (out == "active")
	})
	if err != nil {
		return false, err
	}

	return active, nil
}

func (s *K3sClient) ServerToken() (string, error) {
	return s.runSSH("sudo cat /var/lib/rancher/k3s/server/token")
}

// Retrieves the kubeconfig with the proper host endpoint
func (s *K3sClient) KubeConfig() (string, error) {
	kubeconfigText, err := s.runSSH("sudo cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return "", err
	}
	return updateKubeConfig(kubeconfigText, s.host)

}

// Runs ssh with a stdout, stderr
// First callback is stdout, if no second is passed,
// stderr will be used on the same
func (s *K3sClient) runSSHWithCallback(cmd string, callbacks ...func(string)) error {
	var stdoutF func(string)
	var stderrF func(string)

	if len(callbacks) == 1 {
		stdoutF = callbacks[0]
		stderrF = callbacks[0]
	} else if len(callbacks) == 2 {
		stdoutF = callbacks[0]
		stderrF = callbacks[1]
	} else {
		stdoutF = func(s string) {}
		stderrF = func(s string) {}
	}

	client, err := ssh.Dial("tcp", s.host, s.config)
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
		return fmt.Errorf("cannot open stdout pipe for cmd '%s': %s", cmd, err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot open stderr pipe for cmd '%s': %s", cmd, err)
	}

	// Start the command
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("cannot start cmd '%s': %s", cmd, err)
	}

	done := make(chan struct{}, 2)

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutF(line)
		}
		done <- struct{}{}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrF(line)
		}
		done <- struct{}{}
	}()

	// Wait for both output streams to finish
	<-done
	<-done

	// Wait for the command to finish
	if err := session.Wait(); err != nil {
		return fmt.Errorf("cannot run cmd '%s': %s", cmd, err)
	}

	return nil
}

// Runs a direct command and returns output
func (s *K3sClient) runSSH(cmd string) (string, error) {
	client, err := ssh.Dial("tcp", s.host, s.config)
	if err != nil {
		return "", fmt.Errorf("create client failed %v", err)
	}
	defer client.Close()

	// open session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create session failed %v", err)
	}
	defer session.Close()

	// Start the command
	var out []byte
	if out, err = session.CombinedOutput(cmd); err != nil {
		return "", fmt.Errorf("cannot start cmd '%s': %s", cmd, err)
	}

	return string(out), nil
}

func signerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {
	// read pem block
	err := errors.New("pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	// handle encrypted key
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

func updateKubeConfig(kubeconfigText string, host string) (string, error) {
	config, err := clientcmd.Load([]byte(kubeconfigText))
	if err != nil {
		return "", err
	}

	this := *config.Clusters["default"]
	this.Server = fmt.Sprintf("https://%s:6443", host)
	config.Clusters["default"] = &this

	fixed, err := clientcmd.Write(*config)
	if err != nil {
		return "", err
	}

	return string(fixed), nil
}
