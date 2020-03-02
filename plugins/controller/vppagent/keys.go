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
	"go.ligato.io/vpp-agent/v3/pkg/models"
	linux "go.ligato.io/vpp-agent/v3/proto/ligato/linux/interfaces"
	linuxL3 "go.ligato.io/vpp-agent/v3/proto/ligato/linux/l3"
	interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	l2 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l2"
	l3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"
)

// this must match what utils the vpp-agent uses
var agentPrefix = "/vnf-agent/"
var ifPrefix = "/vpp/config/v2/interface/"
var linuxIfPrefix = "/vpp/config/v2/interface/"

// InterfaceKey constructs interface db key
func InterfaceKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + interfaces.InterfaceKey(ifaceLabel)
}

// LinuxInterfaceKey constructs Linux interface db key
func LinuxInterfaceKey(vppLabel string, ifaceLabel string) string {
	return agentPrefix + vppLabel + "/" + linux.InterfaceKey(ifaceLabel)
}

// L2BridgeDomainKey constructs L2 bridge domain db key
func L2BridgeDomainKey(vppLabel string, bdName string) string {
	return agentPrefix + vppLabel + "/" + l2.BridgeDomainKey(bdName)
}

// L2XConnectKey constructs L2 XConnect db key
func L2XConnectKey(vppLabel string, rxIf string) string {
	return agentPrefix + vppLabel + "/" + l2.XConnectKey(rxIf)
}

// L3RouteKey constructs L3 route db key
func L3RouteKey(vppLabel string, route *l3.Route) string {
	return agentPrefix + vppLabel + "/" + models.Key(route)
}

// LinuxL3RouteKey constructs Linux L3 route db key
func LinuxL3RouteKey(vppLabel string, route *linuxL3.Route) string {
	return agentPrefix + vppLabel + "/" + models.Key(route)
}

// L2FibKey constructs L2 fib db key
func L2FibKey(vppLabel string, fib *l2.FIBEntry) string {
	return agentPrefix + vppLabel + "/" + models.Key(fib)
}

// ArpEntryKey arp key
func ArpEntryKey(vppLabel string, iface string, ipAddress string) string {
	return agentPrefix + vppLabel + "/" + l3.ArpEntryKey(iface, ipAddress)
}
