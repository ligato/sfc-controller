// Copyright (c) 2018 Cisco and/or its affiliates.
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

package controller

import (
	"github.com/ligato/sfc-controller/plugins/controller/model"
)

// Nodes and NetworkServices have interface definitions so an entity is either a node or netservice

func InitInterfaceStatus(
	entityName string,
	vppAgent string,
	connPodInterface string,
	entityInterface *controller.Interface) (*controller.InterfaceStatus, error) {

	ifStatus, exists := ctlrPlugin.ramConfigCache.InterfaceStates[connPodInterface]
	if !exists {
		ifStatus = &controller.InterfaceStatus{
			Name: connPodInterface,
			Node: vppAgent,
		}
	}

	if entityInterface.MacAddress == "" {
		if ifStatus.MacAddress == "" {
			ifStatus.MacAddress, ifStatus.MacAddrID =
				ctlrPlugin.ramConfigCache.MacAddrAllocator.Allocate()
		}
	} else {
		if ifStatus.MacAddress != entityInterface.MacAddress {
			ifStatus.MacAddress = entityInterface.MacAddress
		}
	}
	if len(entityInterface.IpAddresses) == 0 {
		if len(ifStatus.IpAddresses) == 0 {
			if entityInterface.IpamPoolName != "" {
				ipAddress, err := ctlrPlugin.IpamPoolMgr.AllocateAddress(entityInterface.IpamPoolName,
					vppAgent, entityName)
				if err != nil {
					return nil, err
				}
				ifStatus.IpAddresses = []string{ipAddress}
			}
		}
	} else {
		// seems hard coded addresses are provided ... what if they dont match what we have for this
		// interface in the status section
		if len(ifStatus.IpAddresses) == 0 {
			ifStatus.IpAddresses = entityInterface.IpAddresses

		} else if !ipAddressArraysEqual(ifStatus.IpAddresses, entityInterface.IpAddresses) {
			log.Warnf("initInterfaceStatus: provision interface %s, configured ip addresses (%v dont match current state: %v",
				connPodInterface,
				entityInterface.IpAddresses,
				ifStatus.IpAddresses)
			ifStatus.IpAddresses = entityInterface.IpAddresses
		}
	}

	return ifStatus, nil
}

func PersistInterfaceStatus(
	interfaces map[string]*controller.InterfaceStatus,
	ifStatus *controller.InterfaceStatus,
	entityInterfaceName string) {

	interfaces[entityInterfaceName] = ifStatus
	ctlrPlugin.ramConfigCache.InterfaceStates[entityInterfaceName] = ifStatus
}
