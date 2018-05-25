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

//go:generate protoc --proto_path=model/iosxe --gogo_out=model/iosxe model/iosxe/iosxe.proto

package iosxecfg

import (
	"errors"
	"net"
	"strings"

	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/ligato/sfc-controller/controller/utils/vty"
	"github.com/ligato/sfc-controller/controller/utils/vty/ssh"
)

// Session represents a configuration session with an IOS XE Router.
type Session struct {
	ssh          *ssh.Connection
	vty          *vty.Session
	inConfigMode bool
}

var log = logroot.StandardLogger()

// NewSession creates a new configuration session with an IOS XE Router from an existing (open) VTY session.
// The session should be closed with the Close() method.
func NewSession(vty *vty.Session) (*Session, error) {
	return &Session{vty: vty}, nil
}

// NewSSHSession connects to the router via SSH and creates a new configuration session with the IOS XE Router.
// The session should be closed with the Close() method.
func NewSSHSession(host string, port uint32, userName string, password string) (*Session, error) {
	var err error
	s := &Session{}

	s.ssh, err = ssh.Connect(host, port, userName, password)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	s.vty, err = vty.NewSession(s.ssh)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return s, nil
}

// Close closes the session and releases all resources tied to it.
func (s *Session) Close() {
	if s.vty != nil {
		s.vty.Close()
	}
	if s.ssh != nil {
		s.ssh.Close()
	}
}

// CopyRunningToStartup copies running configuration into the starup configuration of the router.
// It does the equivalent to the following CLI config:
//  copy running-config startup-config
func (s *Session) CopyRunningToStartup() (err error) {
	s.exitConfigMode()
	resp, err := s.vty.ExecCMD("copy running-config startup-config", "")

	if err != nil {
		log.Error("Error by copying running to startup: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}
	return nil
}

// enterConfigMode enters configuration mode ('configure terminal') on the router.
func (s *Session) enterConfigMode() error {
	if s.inConfigMode {
		return nil
	}
	resp, err := s.vty.ExecCMD("conf t")

	if err != nil {
		log.Error("Error by entering config mode: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}

	s.inConfigMode = true
	return nil
}

// exitConfigMode exits configuration mode on the router.
func (s *Session) exitConfigMode() error {
	if !s.inConfigMode {
		return nil
	}
	resp, err := s.vty.ExecCMD("exit")

	if err != nil {
		log.Error("Error by exiting config mode: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}

	s.inConfigMode = false
	return nil
}

// checkResponse checks whether the response from router contains some errors.
func checkResponse(response string) error {
	if strings.Contains(response, "Invalid input") {
		log.Error("Error in the response from the router")
		return errors.New("error in the response from the router")
	}
	return nil
}

// ipv4CidrToIPMask converts ipv4 cidr prefix (e.g. 1.2.3.4/24) to IP address and netmask strings.
func ipv4CidrToIPMask(cidr string) (ip, netmask string, err error) {
	address, network, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Errorf("Unable to convert cidr prefix to IP and netmask (%s): %s", cidr, err)
		return
	}
	ip = address.To4().String()
	netmask = net.IP(network.Mask).To4().String()
	return
}
