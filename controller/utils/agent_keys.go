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

package utils

import (
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l3plugin/model/l3"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linuxplugin/model/interfaces"
	"net"
	"strings"
)

// this must match what utils the vpp-agent uses
var agentPrefix = "/vnf-agent/"

func GetVppEtcdlabel(key string) string {
	strs := strings.Split(key, "/")
	if len(strs) < 3 {
		return ""
	}
	return strs[2]
}

func GetVppAgentPrefix() string {
	return agentPrefix
}

func InterfaceStateKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceStateKey(ifaceLabel)
}

func InterfaceStatePrefixKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceStateKeyPrefix()
}

func InterfaceKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceKey(ifaceLabel)
}

func InterfacePrefixKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceKeyPrefix()
}

func LinuxInterfaceKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + linuxIntf.InterfaceKey(ifaceLabel)
}

func LinuxInterfacePrefixKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + linuxIntf.InterfaceKeyPrefix()
}

func L2BridgeDomainKey(vppLabel string, bdName string) string {
	return agentPrefix + vppLabel + "/" + l2.BridgeDomainKey(bdName)
}

func L2BridgeDomainKeyPrefix(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + l2.BridgeDomainKeyPrefix()
}

func L2XConnectKey(vppLabel string, rxIf string) string {
	return agentPrefix + vppLabel + "/" + l2.XConnectKey(rxIf)
}

func L3RouteKeyPrefix(vppLabel string) string {
	//return agentPrefix + vppLabel + "/" + l3.RouteKeyPrefix()
	return agentPrefix + vppLabel + "/" + l3.VrfKeyPrefix()
}

func L3RouteKey(vppLabel string, vrf uint32, destNet *net.IPNet, nextHop string) string {
	return agentPrefix + vppLabel + "/" + l3.RouteKey(vrf, destNet, nextHop)
}

func CustomInfoKey(vppLabel string) string {
	return agentPrefix + vppLabel + "/" + "vpp/config/v1/custom"
}
