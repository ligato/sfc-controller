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

package iosxecfg

import (
	"fmt"
	"strings"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/sfc-controller/controller/extentitydriver/iosxecfg/model/iosxe"
)

// AddBridgeDomain adds a new bridge domain into the router's configuration.
func (s *Session) AddBridgeDomain(bd *iosxe.BridgeDomain) error {
	logFields := log.WithFields(logging.Fields{
		"id": bd.Id,
		"bd": bd,
	})
	logFields.Info("Adding bridge domain")

	cmds := []string{
		fmt.Sprintf("bridge-domain %d", bd.Id),
	}

	// VNI members
	for _, vni := range bd.Vni {
		cmds = append(cmds, fmt.Sprintf("member vni %d", vni))
	}

	// interface members
	for _, iface := range bd.Interfaces {
		cmds = append(cmds, fmt.Sprintf("member %s service-instance %d", iface.Name, iface.ServiceInstance))
		cmds = append(cmds, "exit")
	}

	cmds = append(cmds, "exit")

	// execute the commands on VTY
	s.enterConfigMode()
	resp, err := s.vty.ExecCMD(cmds...)

	if err != nil {
		logFields.Error("Error by configuring bridge domain: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}
	return nil
}

// DeleteBridgeDomain deletes the bridge domain from the router's configuration.
func (s *Session) DeleteBridgeDomain(bd *iosxe.BridgeDomain) error {
	logFields := log.WithFields(logging.Fields{
		"id": bd.Id,
		"bd": bd,
	})
	logFields.Info("Deleting bridge domain")

	// execute the commands on VTY
	s.enterConfigMode()
	resp, err := s.vty.ExecCMD(fmt.Sprintf("no bridge-domain %d", bd.Id))

	if err != nil {
		logFields.Error("Error by deleting bridge domain: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}
	return nil
}

// ModifyBridgeDomain modifies the existing bridge domain config to a new version of it.
func (s *Session) ModifyBridgeDomain(oldBd, newBd *iosxe.BridgeDomain) error {

	// TODO: implement harmless modification (without deleting and impacting of existing config)

	err := s.DeleteBridgeDomain(oldBd)
	if err != nil {
		return err
	}

	err = s.AddBridgeDomain(newBd)
	if err != nil {
		return err
	}

	return nil
}

// DumpBridgeDomains dumps all existing bridge domains from the router into a map indexed by bridge domain IDs.
func (s *Session) DumpBridgeDomains() (map[uint32]*iosxe.BridgeDomain, error) {
	log.Debug("Dumping bridge domains")

	bds := make(map[uint32]*iosxe.BridgeDomain)

	// dump via VTY
	s.exitConfigMode()
	resp, err := s.vty.ExecCMD("show running-config | section bridge-domain")

	if err != nil {
		log.Error("Error by dumping bridge domains: ", err)
		return bds, err
	}
	if err = checkResponse(resp); err != nil {
		return bds, err
	}

	var bdId uint32

	for _, line := range strings.Split(strings.TrimSuffix(resp, "\n"), "\n") {
		if _, err := fmt.Sscanf(line, "bridge-domain %d", &bdId); err == nil {
			// create bridge domain
			bds[bdId] = &iosxe.BridgeDomain{Id: bdId}
		} else {
			// fill in bridge domain details
			if _, ok := bds[bdId]; !ok {
				log.Debug("non-existing bridge domain %d", bdId)
				continue
			}

			// VNI
			var vni uint32
			if _, err := fmt.Sscanf(line, "member vni %d", &vni); err == nil {
				bds[bdId].Vni = append(bds[bdId].Vni, vni)
			}

			// service instance
			var ifName string
			var si uint32
			if _, err := fmt.Sscanf(line, "member %s service-instance %d", &ifName, &si); err == nil {
				bds[bdId].Interfaces = append(bds[bdId].Interfaces, &iosxe.BridgeDomain_Interface{
					Name:            ifName,
					ServiceInstance: si,
				})
			}
		}
	}

	return bds, nil
}
