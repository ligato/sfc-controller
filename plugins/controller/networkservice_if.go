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
	"fmt"

	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
)

// RenderConnInterfacePair renders this interface pair on the vnf and vswitch
func (ns *NetworkService) RenderConnInterfacePair(
	vppAgent string,
	connPodInterface string,
	vnfInterface *controller.Interface,
	networkPodType string) (string, error) {

	// The interface should be created in the vnf and the vswitch then the nsitch
	// interfaces will be added to the bridge.

	switch vnfInterface.IfType {
	case controller.IfTypeMemif:
		return ns.RenderConnMemifPair(vppAgent, connPodInterface, vnfInterface, networkPodType)
	case controller.IfTypeVeth:
		return ns.RenderConnVethAfpPair(vppAgent, connPodInterface, vnfInterface, networkPodType)
	case controller.IfTypeTap:
		return ns.RenderConnTapPair(vppAgent, connPodInterface, vnfInterface, networkPodType)
	case controller.IfTypeEthernet:
		// the ethernet interface is special and has been created by the node already
		return vnfInterface.Name, nil
	}

	return "", nil
}

// RenderConnMemifPair renders this vnf/vswitch interface pair
func (ns *NetworkService) RenderConnMemifPair(
	vppAgent string,
	connPodInterface string,
	networkPodInterface *controller.Interface,
	networkPodType string) (string, error) {

	var ifName string

	connPodName, connInterfaceName := ConnPodInterfaceNames(connPodInterface)

	ifStatus, err := InitInterfaceStatus(ns.Metadata.Name, vppAgent, connPodInterface, networkPodInterface)
	if err != nil {
		return "", err
	}
	if ifStatus.MemifID == 0 {
		ifStatus.MemifID = ctlrPlugin.ramConfigCache.MemifIDAllocator.Allocate()
	}
	PersistInterfaceStatus(ns.Status.Interfaces, ifStatus, connPodInterface)

	vppKV := vppagent.ConstructMemInterface(
		connPodName,
		connInterfaceName,
		ifStatus.IpAddresses,
		ifStatus.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterface.RxMode),
		ifStatus.MemifID,
		false,
		networkPodInterface.MemifParms,
		vppAgent)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	log.Debugf("RenderToplogyMemifPair: ifName: %s, %v", connInterfaceName, vppKV)

	ifName = "IF_MEMIF_VSWITCH_" + ConnPodInterfaceSlashToUScore(connPodInterface)

	vppKV = vppagent.ConstructMemInterface(
		vppAgent,
		ifName,
		[]string{},
		"",
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterface.RxMode),
		ifStatus.MemifID,
		true,
		networkPodInterface.MemifParms,
		vppAgent)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	log.Debugf("RenderToplogyMemifPair: ifName: %s, %v", ifName, vppKV)

	return ifName, nil
}

// RenderConnDirectInterPodMemifPair renders this pod-pod interface pair
func (ns *NetworkService) RenderConnDirectInterPodMemifPair(
	conn *controller.Connection,
	netPodInterfaces []*controller.Interface,
	networkPodType string) error {

	connPodName0, connInterfaceName0 := ConnPodInterfaceNames(conn.PodInterfaces[0])
	connPodName1, connInterfaceName1 := ConnPodInterfaceNames(conn.PodInterfaces[1])

	if0Status, err := InitInterfaceStatus(ns.Metadata.Name, connPodName0, conn.PodInterfaces[0], netPodInterfaces[0])
	if err != nil {
		return err
	}
	if if0Status.MemifID == 0 {
		if0Status.MemifID = ctlrPlugin.ramConfigCache.MemifIDAllocator.Allocate()
	}
	PersistInterfaceStatus(ns.Status.Interfaces, if0Status, conn.PodInterfaces[0])

	vppKV := vppagent.ConstructMemInterface(
		connPodName0,
		connInterfaceName0,
		if0Status.IpAddresses,
		if0Status.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(netPodInterfaces[0].Mtu),
		netPodInterfaces[0].AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(netPodInterfaces[0].RxMode),
		if0Status.MemifID,
		false,
		netPodInterfaces[0].MemifParms,
		connPodName1)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	log.Debugf("RenderToplogyDirectInterVnfMemifPair: ifName0: %s, %v",
		conn.PodInterfaces[0], vppKV)

	if1Status, err := InitInterfaceStatus(ns.Metadata.Name, connPodName1, conn.PodInterfaces[1], netPodInterfaces[1])
	if err != nil {
		return err
	}
	if1Status.MemifID = if0Status.MemifID
	PersistInterfaceStatus(ns.Status.Interfaces, if1Status, conn.PodInterfaces[1])

	vppKV = vppagent.ConstructMemInterface(
		connPodName1,
		connInterfaceName1,
		if1Status.IpAddresses,
		if1Status.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(netPodInterfaces[1].Mtu),
		netPodInterfaces[1].AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(netPodInterfaces[1].RxMode),
		if1Status.MemifID,
		true,
		netPodInterfaces[1].MemifParms,
		connPodName1)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	log.Debugf("RenderToplogyDirectInterVnfMemifPair: ifName1: %s, %v",
		conn.PodInterfaces[1], vppKV)

	return nil
}

// RenderConnTapPair renders this pod/vswitch tap interface pair
func (ns *NetworkService) RenderConnTapPair(
	vppAgent string,
	connPodInterface string,
	vnfInterface *controller.Interface,
	networkPodType string) (string, error) {

	return "", fmt.Errorf("tap not supported")
}

// RenderConnVethAfpPair renders this pod/vswitch veth/afp interface pair
func (ns *NetworkService) RenderConnVethAfpPair(
	vppAgent string,
	connPodInterface string,
	networkPodInterface *controller.Interface,
	networkPodType string) (string, error) {

	var ifName string

	connPodName, connInterfaceName := ConnPodInterfaceNames(connPodInterface)

	ifStatus, err := InitInterfaceStatus(ns.Metadata.Name, vppAgent, connPodName, networkPodInterface)
	if err != nil {
		return "", err
	}
	PersistInterfaceStatus(ns.Status.Interfaces, ifStatus, connPodInterface)

	// Create a VETH i/f for the vnf container, the ETH will get created
	// by the vpp-agent in a more privileged vswitch.
	// Note: In Linux kernel the length of an interface name is limited by
	// the constant IFNAMSIZ. In most distributions this is 16 characters
	// including the terminating NULL character. The hostname uses chars
	// from the container for a total of 15 chars.

	veth1Name := "IF_VETH_VNF_" + connPodName + "_" + connInterfaceName
	veth2Name := "IF_VETH_VSWITCH_" + connPodName + "_" + connInterfaceName
	host1Name := connInterfaceName
	baseHostName := constructBaseHostName(connPodName, connInterfaceName)
	host2Name := baseHostName

	vethIPAddresses := ifStatus.IpAddresses
	if networkPodType == controller.NetworkPodTypeVPPContainer {
		vethIPAddresses = []string{}
	}
	// Configure the VETH interface for the VNF end
	vppKV := vppagent.ConstructVEthInterface(vppAgent,
		veth1Name,
		vethIPAddresses,
		ifStatus.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		host1Name,
		veth2Name,
		connPodName)

	log.Printf("%v", vppKV)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	// Configure the VETH interface for the VSWITCH end
	vppKV = vppagent.ConstructVEthInterface(vppAgent,
		veth2Name,
		[]string{},
		"",
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		host2Name,
		veth1Name,
		vppAgent)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	// Configure the AFP interface for the VNF end
	if networkPodType == controller.NetworkPodTypeVPPContainer {
		vppKV = vppagent.ConstructAFPacketInterface(connPodName,
			networkPodInterface.Name,
			ifStatus.IpAddresses,
			ifStatus.MacAddress,
			ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
			networkPodInterface.AdminStatus,
			ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterface.RxMode),
			host1Name)
		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService + "/" + ns.Metadata.Name,
			vppKV)
	}
	// Configure the AFP interface for the VSWITCH end
	ifName = "IF_AFPIF_VSWITCH_" + connPodName + "_" + connInterfaceName
	vppKV = vppagent.ConstructAFPacketInterface(vppAgent,
		ifName,
		[]string{},
		"",
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterface.RxMode),
		host2Name)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	return ifName, nil
}

func stringFirstNLastM(n int, m int, str string) string {
	if len(str) <= n+m {
		return str
	}
	outStr := ""
	for i := 0; i < n; i++ {
		outStr += fmt.Sprintf("%c", str[i])
	}
	for i := 0; i < m; i++ {
		outStr += fmt.Sprintf("%c", str[len(str)-m+i])
	}
	return outStr
}

func constructBaseHostName(container string, port string) string {

	// Use at most 8 chrs from cntr name, and 7 from port
	// If cntr is less than 7 then can use more for port and visa versa.  Also, when cntr and port name
	// is more than 7 chars, use first few chars and last few chars from name ... brain dead scheme?
	// will it be readable?

	cb := 4 // 4 from beginning of container string
	ce := 4 // 4 from end of container string
	pb := 3 // 3 from beginning of port string
	pe := 4 // 4 from end of port string

	if len(container) < 8 {
		// increase char budget for port if container is less than max budget of 8
		switch len(container) {
		case 7:
			pb++
		case 6:
			pb++
			pe++
		case 5:
			pb += 2
			pe++
		case 4:
			pb += 2
			pe += 2
		case 3:
			pb += 3
			pe += 2
		case 2:
			pb += 3
			pe += 3
		case 1:
			pb += 4
			pe += 3
		}
	}

	if len(port) < 7 {
		// increase char budget for container if port is less than max budget of 7
		switch len(port) {
		case 6:
			cb++
		case 5:
			cb++
			ce++
		case 4:
			cb += 2
			ce++
		case 3:
			cb += 2
			ce += 2
		case 2:
			cb += 3
			ce += 2
		case 1:
			cb += 3
			ce += 3
		}
	}

	return stringFirstNLastM(cb, ce, container) + stringFirstNLastM(pb, pe, port)
}
