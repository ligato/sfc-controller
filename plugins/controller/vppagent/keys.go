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

package vppagent

import (
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/l2"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/l3"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linuxplugin/common/model/interfaces"
	"net"
	"strings"
)

// this must match what utils the vpp-agent uses
var agentPrefix = "/vnf-agent/"

// GetVppEtcdlabel extracts from db key VppEtcd label
func GetVppEtcdlabel(key string) string {
	strs := strings.Split(key, "/")
	if len(strs) < 3 {
		return ""
	}
	return strs[2]
}

// GetVppAgentPrefix provides vpp agent prefix for db keys
func GetVppAgentPrefix() string {
	return agentPrefix
}

// InterfaceStateKey constructs interface state db key
func InterfaceStateKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceStateKey(ifaceLabel)
}

// InterfaceStatePrefixKey constructs interface state prefix db key
func InterfaceStatePrefixKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceStateKeyPrefix()
}

// InterfaceKey constructs interface db key
func InterfaceKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceKey(ifaceLabel)
}

// InterfacePrefixKey constructs interface prefix db key
func InterfacePrefixKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceKeyPrefix()
}

// LinuxInterfaceKey constructs Linux interface db key
func LinuxInterfaceKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + linuxIntf.InterfaceKey(ifaceLabel)
}

// LinuxInterfacePrefixKey constructs Linux interface prefix db key
func LinuxInterfacePrefixKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + linuxIntf.InterfaceKeyPrefix()
}

// L2BridgeDomainKey constructs L2 bridge domain db key
func L2BridgeDomainKey(vppLabel string, bdName string) string {
	return agentPrefix + vppLabel + "/" + l2.BridgeDomainKey(bdName)
}

// L2BridgeDomainKeyPrefix constructs L2 bridge domain db key prefix
func L2BridgeDomainKeyPrefix(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + l2.BridgeDomainKeyPrefix()
}

// L2XConnectKey constructs L2 XConnect db key
func L2XConnectKey(vppLabel string, rxIf string) string {
	return agentPrefix + vppLabel + "/" + l2.XConnectKey(rxIf)
}

// L3RouteKeyPrefix constructs L3 route db key prefix
func L3RouteKeyPrefix(vppLabel string) string {
	//return agentPrefix + vppLabel + "/" + l3.RouteKeyPrefix()
	return agentPrefix + vppLabel + "/" + l3.VrfKeyPrefix()
}

// L3RouteKey constructs L3 route db key
func L3RouteKey(vppLabel string, vrf uint32, destNet *net.IPNet, nextHop string) string {
	return agentPrefix + vppLabel + "/" + l3.RouteKey(vrf, destNet.String(), nextHop)
}

// ArpEntryKey arp key
func ArpEntryKey(vppLabel string, iface string, ipAddress string) string {
	return agentPrefix + vppLabel + "/" + l3.ArpEntryKey(iface, ipAddress)
}
