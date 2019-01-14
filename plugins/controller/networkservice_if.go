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
	conn *controller.Connection,
	netPodInterface *controller.Interface,
	networkPodType string) (string, *controller.InterfaceStatus, error) {

	// The interface should be created in the vnf and the vswitch then the nsitch
	// interfaces will be added to the bridge.

	var ifName string
	var ifStatus *controller.InterfaceStatus
	var err error

	switch netPodInterface.IfType {
	case controller.IfTypeMemif:
		ifName, ifStatus, err = ns.RenderConnMemifPair(vppAgent, conn, netPodInterface, networkPodType)
	case controller.IfTypeVeth:
		ifName, ifStatus, err =  ns.RenderConnVethAfpPair(vppAgent, conn, netPodInterface, networkPodType)
	case controller.IfTypeTap:
		ifName, ifStatus, err =  ns.RenderConnTapPair(vppAgent, conn, netPodInterface, networkPodType)
	case controller.IfTypeEthernet:
		// the ethernet interface is special and has been created by the node already
		return netPodInterface.Name, nil, nil
	}

	if err == nil {
		if err := ns.RenderInterfaceForwarding(netPodInterface); err != nil {
			return "", nil, err
		}
	}

	return ifName, ifStatus, err
}

// RenderConnMemifPair renders this vnf/vswitch interface pair
func (ns *NetworkService) RenderConnMemifPair(
	vppAgent string,
	conn *controller.Connection,
	networkPodInterface *controller.Interface,
	networkPodType string) (string, *controller.InterfaceStatus, error) {

	var ifName string

	connPodName := networkPodInterface.Parent
	connInterfaceName := networkPodInterface.Name

	ifStatus, err := InitInterfaceStatus(ns.Metadata.Name, vppAgent, networkPodInterface)
	if err != nil {
		RemoveInterfaceStatus(ns.Status.Interfaces, connPodName, connInterfaceName)
		msg := fmt.Sprintf("network pod interface: %s/%s, %s", connPodName, connInterfaceName, err)
		ns.AppendStatusMsg(msg)
		return "", nil, err
	}
	if ifStatus.MemifID == 0 {
		ifStatus.MemifID = ctlrPlugin.ramCache.MemifIDAllocator.Allocate()
	}
	PersistInterfaceStatus(ns.Status.Interfaces, ifStatus, connPodName, connInterfaceName)

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
		ctlrPlugin.SysParametersMgr.sysParmCache.MemifDirectory,
		vppAgent)

	//vppKV.IFace.Vrf = conn.VrfId

	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	log.Debugf("RenderToplogyMemifPair: ifName: %s, %v", connInterfaceName, vppKV)

	ifName = "IF_MEMIF_VSWITCH_" + connPodName + "_" + connInterfaceName

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
		ctlrPlugin.SysParametersMgr.sysParmCache.MemifDirectory,
		vppAgent)

	vppKV.IFace.Vrf = conn.VrfId

	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	log.Debugf("RenderToplogyMemifPair: ifName: %s, %v", ifName, vppKV)

	return ifName, ifStatus, nil
}

// RenderConnDirectInterPodMemifPair renders this pod-pod interface pair
func (ns *NetworkService) RenderConnDirectInterPodMemifPair(
	networkPodInterfaces []*controller.Interface,
	networkPodType string) error {

	connPodName0 := networkPodInterfaces[0].Parent
	connInterfaceName0 := networkPodInterfaces[0].Name
	connPodName1 := networkPodInterfaces[1].Parent
	connInterfaceName1 := networkPodInterfaces[1].Name

	if0Status, err := InitInterfaceStatus(ns.Metadata.Name, connPodName0, networkPodInterfaces[0])
	if err != nil {
		RemoveInterfaceStatus(ns.Status.Interfaces, connPodName0, connInterfaceName0)
		msg := fmt.Sprintf("network pod interface: %s/%s, %s", connPodName0, connInterfaceName0, err)
		ns.AppendStatusMsg(msg)
		return err
	}
	if if0Status.MemifID == 0 {
		if0Status.MemifID = ctlrPlugin.ramCache.MemifIDAllocator.Allocate()
	}
	PersistInterfaceStatus(ns.Status.Interfaces, if0Status, connPodName0, connInterfaceName0)

	vppKV := vppagent.ConstructMemInterface(
		connPodName0,
		connInterfaceName0,
		if0Status.IpAddresses,
		if0Status.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterfaces[0].Mtu),
		networkPodInterfaces[0].AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterfaces[0].RxMode),
		if0Status.MemifID,
		false,
		networkPodInterfaces[0].MemifParms,
		ctlrPlugin.SysParametersMgr.sysParmCache.MemifDirectory,
		connPodName1)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	if err = ns.RenderInterfaceForwarding(networkPodInterfaces[0]); err != nil {
		return err
	}

	log.Debugf("RenderToplogyDirectInterVnfMemifPair: ifName0: %s/%s, %v",
		connPodName0, connInterfaceName0, vppKV)

	if1Status, err := InitInterfaceStatus(ns.Metadata.Name, connPodName1, networkPodInterfaces[1])
	if err != nil {
		RemoveInterfaceStatus(ns.Status.Interfaces, connPodName1, connInterfaceName1)
		msg := fmt.Sprintf("network pod interface: %s/%s, %s", connPodName1, connInterfaceName1, err)
		ns.AppendStatusMsg(msg)
		return err
	}
	if1Status.MemifID = if0Status.MemifID
	PersistInterfaceStatus(ns.Status.Interfaces, if1Status, connPodName1, connInterfaceName1)

	vppKV = vppagent.ConstructMemInterface(
		connPodName1,
		connInterfaceName1,
		if1Status.IpAddresses,
		if1Status.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterfaces[1].Mtu),
		networkPodInterfaces[1].AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterfaces[1].RxMode),
		if1Status.MemifID,
		true,
		networkPodInterfaces[1].MemifParms,
		ctlrPlugin.SysParametersMgr.sysParmCache.MemifDirectory,
		connPodName1)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	if err = ns.RenderInterfaceForwarding(networkPodInterfaces[1]); err != nil {
		return err
	}

	log.Debugf("RenderToplogyDirectInterVnfMemifPair: ifName1: %s/%s, %v",
		connPodName1, connInterfaceName1, vppKV)

	return nil
}

// RenderConnTapPair renders this pod/vswitch tap interface pair
func (ns *NetworkService) RenderConnTapPair(
	vppAgent string,
	conn *controller.Connection,
	networkPodInterface *controller.Interface,
	networkPodType string) (string, *controller.InterfaceStatus, error) {

	var ifName string

	connPodName := networkPodInterface.Parent
	connInterfaceName := networkPodInterface.Name

	ifStatus, err := InitInterfaceStatus(ns.Metadata.Name, vppAgent, networkPodInterface)
	if err != nil {
		RemoveInterfaceStatus(ns.Status.Interfaces, connPodName, connInterfaceName)
		msg := fmt.Sprintf("network pod interface: %s/%s, %s", connPodName, connInterfaceName, err)
		ns.AppendStatusMsg(msg)
		return "", nil, err
	}

	linTapIfName := "IF_TAP_VNF_" + connPodName + "_" + connInterfaceName
	tapIfName := "IF_TAP_VSWITCH_" + connPodName + "_" + connInterfaceName

	hostPortLabel := networkPodInterface.HostPortLabel
	if hostPortLabel == "" {
		hostPortLabel = constructBaseHostName(connPodName, connInterfaceName)
	}

	ifStatus.HostPortLabel = hostPortLabel
	PersistInterfaceStatus(ns.Status.Interfaces, ifStatus, connPodName, connInterfaceName)

	microServiceLabel := connPodName
	hostNameSpace := ""
	if networkPodInterface.TapParms != nil {
		hostNameSpace = networkPodInterface.TapParms.Namespace
	}
	// Configure the linux tap interface for the VNF end
	vppKV := vppagent.ConstructLinuxTapInterface(vppAgent,
		linTapIfName,
		ifStatus.IpAddresses,
		ifStatus.MacAddress,
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		hostPortLabel,
		hostNameSpace,
		microServiceLabel)

	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	// Configure the tap interface for the VSWITCH end
	vppKV = vppagent.ConstructTapInterface(vppAgent,
		tapIfName,
		[]string{},
		"",
		ctlrPlugin.SysParametersMgr.ResolveMtu(networkPodInterface.Mtu),
		networkPodInterface.AdminStatus,
		ctlrPlugin.SysParametersMgr.ResolveRxMode(networkPodInterface.RxMode),
		networkPodInterface.TapParms,
		hostPortLabel)

	vppKV.IFace.Vrf = conn.VrfId

	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	return ifName, ifStatus, nil
}

// RenderConnVethAfpPair renders this pod/vswitch veth/afp interface pair
func (ns *NetworkService) RenderConnVethAfpPair(
	vppAgent string,
	conn *controller.Connection,
	networkPodInterface *controller.Interface,
	networkPodType string) (string, *controller.InterfaceStatus, error) {

	var ifName string

	connPodName := networkPodInterface.Parent
	connInterfaceName := networkPodInterface.Name

	ifStatus, err := InitInterfaceStatus(ns.Metadata.Name, vppAgent, networkPodInterface)
	if err != nil {
		RemoveInterfaceStatus(ns.Status.Interfaces, connPodName, connInterfaceName)
		msg := fmt.Sprintf("network pod interface: %s/%s, %s", connPodName, connInterfaceName, err)
		ns.AppendStatusMsg(msg)
		return "", nil, err
	}

	// Create a VETH i/f for the vnf container, the ETH will get created
	// by the vpp-agent in a more privileged vswitch.
	// Note: In Linux kernel the length of an interface name is limited by
	// the constant IFNAMSIZ. In most distributions this is 16 characters
	// including the terminating NULL character. The hostname uses chars
	// from the container for a total of 15 chars.

	veth1Name := "IF_VETH_VNF_" + connPodName + "_" + connInterfaceName
	veth2Name := "IF_VETH_VSWITCH_" + connPodName + "_" + connInterfaceName
	host1Name := connInterfaceName

	host2Name := networkPodInterface.HostPortLabel
	if host2Name == "" {
		host2Name = constructBaseHostName(connPodName, connInterfaceName)
	}

	ifStatus.HostPortLabel = host2Name
	PersistInterfaceStatus(ns.Status.Interfaces, ifStatus, connPodName, connInterfaceName)

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
		networkPodInterface.LinuxNamespace,
		connPodName)

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
		networkPodInterface.LinuxNamespace,
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

		vppKV.IFace.Vrf = conn.VrfId

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

	vppKV.IFace.Vrf = conn.VrfId

	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService + "/" + ns.Metadata.Name,
		vppKV)

	return ifName, ifStatus, nil
}

// each interface can have a set of fwd-ing instructions so render them against the interface
func (ns *NetworkService) RenderInterfaceForwarding(
	networkPodInterface *controller.Interface) error {

	log.Debugf("renderInterfaceForwarding: %v", networkPodInterface)

	if 	networkPodInterface.Fwd == nil {
		return nil
	}

	vppAgent := networkPodInterface.Parent

	for _, l3Vrf := range networkPodInterface.Fwd.L3VrfRoute {
		desc := fmt.Sprintf("FWD NS_%s_IF_%s_VRF_%d_DST_%s", ns.Metadata.Name,
			networkPodInterface.Name, l3Vrf.VrfId, l3Vrf.DstIpAddr)
		l3sr := &controller.L3VRFRoute{
			VrfId:             l3Vrf.VrfId,
			Description:       desc,
			DstIpAddr:         l3Vrf.DstIpAddr,
			NextHopAddr:       l3Vrf.NextHopAddr,
			OutgoingInterface: networkPodInterface.Name,
		}
		vppKV := vppagent.ConstructStaticRoute(vppAgent, l3sr)
		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService + "/" + ns.Metadata.Name,
			vppKV)
	}
	for _, l3Arp := range networkPodInterface.Fwd.L3Arp {
		ae := &controller.L3ArpEntry{
			IpAddress: l3Arp.IpAddress,
			PhysAddress: l3Arp.PhysAddress,
			OutgoingInterface: networkPodInterface.Name,
		}
		vppKV := vppagent.ConstructStaticArpEntry(vppAgent, ae)
		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService + "/" + ns.Metadata.Name,
			vppKV)
	}

	return nil
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
