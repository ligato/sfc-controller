// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"fmt"
	"io"
	"net"

	"github.com/ligato/cn-infra/logging/logrus"
	"golang.org/x/crypto/ssh"
)

// Connection represents a SSH connection.
type Connection struct {
	client  *ssh.Client
	session *ssh.Session
}

var log = logrus.DefaultLogger()

// Connect connects top a host via SSH using password authentication.
func Connect(host string, port uint32, userName string, password string) (*Connection, error) {
	var err error
	c := &Connection{}

	// prepare SSH config
	config := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil // TODO verify or at least log host public key
		},
	}

	// connect
	c.client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		log.Printf("Failed to dial: %s", err)
		return nil, err
	}

	// start a new session
	c.session, err = c.client.NewSession()
	if err != nil {
		log.Printf("Failed to create session: %s", err)
		return nil, err
	}

	return c, nil
}

// GetVTY returns the interface to VTY for reading and writing.
func (c *Connection) GetVTY() (io.Reader, io.Writer, error) {

	// request a PTY
	err := c.session.RequestPty("xterm", 5000, 5000, ssh.TerminalModes{})
	if err != nil {
		log.Printf("Failed to request a PTY: %s", err)
		return nil, nil, err
	}

	// create pipe to the stdin of the SSH process
	w, err := c.session.StdinPipe()
	if err != nil {
		log.Printf("Failed to create stdin pipe: %s", err)
		return nil, nil, err
	}

	// create pipe to the stdout of the SSH process
	r, err := c.session.StdoutPipe()
	if err != nil {
		log.Printf("Failed to create stdout pipe: %s", err)
		return nil, nil, err
	}

	// start the shell
	err = c.session.Shell()
	if err != nil {
		log.Printf("Failed to start the shell: %s", err)
		return nil, nil, err
	}

	return r, w, nil
}

// Close closes the SSH connection.
func (c *Connection) Close() {
	c.session.Close()
	c.client.Close()
}
