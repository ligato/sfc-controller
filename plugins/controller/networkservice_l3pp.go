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

// The L3PP topology is rendered in this module for a connection with a vnf-service

// RenderConnL3PP renders this L3PP connection
func (ns *NetworkService) RenderConnL3PP(
	conn *controller.Connection,
	connIndex uint32) error {

	var p2nArray [2]NetworkPodToNodeMap
	netPodInterfaces := make([]*controller.Interface, 2)
	networkPodTypes := make([]string, 2)

	allPodsAssignedToNodes := true
	staticNodesInInterfacesSpecified := false

	log.Debugf("RenderConnL3PP: num pod interfaces: %d", len(conn.PodInterfaces))
	log.Debugf("RenderConnL3PP: num node interfaces: %d", len(conn.NodeInterfaces))
	log.Debugf("RenderConnL3PP: num node labels: %d", len(conn.NodeInterfaceLabels))

	ifIndex := 0

	// let see if all interfaces in the conn are associated with a node
	for i, connPodInterface := range conn.PodInterfaces {

		if ifIndex >= 2 {
			msg := fmt.Sprintf("Too many connection segments specified for a l3pp connection")
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		connPodName, connInterfaceName := ConnPodInterfaceNames(connPodInterface)

		p2n, exists := ctlrPlugin.ramCache.NetworkPodToNodeMap[connPodName]
		if !exists || p2n.Node == "" {
			msg := fmt.Sprintf("connection segment %d: %s, network pod not mapped to a node in network_pod_to_node_map",
				i, connPodInterface)
			ns.AppendStatusMsg(msg)
			allPodsAssignedToNodes = false
			continue
		}
		_, exists = ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationR(p2n.Node)
		if !exists {
			msg := fmt.Sprintf("connection segment %d: %s, network pod references non existant host: %s",
				i, connPodInterface, p2n.Node)
			ns.AppendStatusMsg(msg)
			allPodsAssignedToNodes = false
			continue
		}

		p2nArray[ifIndex] = *p2n
		netPodInterface, networkPodType := ns.findNetworkPodAndInterfaceInList(connPodName,
			connInterfaceName, ns.Spec.NetworkPods)
		netPodInterfaces[ifIndex] = netPodInterface
		netPodInterfaces[ifIndex].Parent = connPodName
		networkPodTypes[ifIndex] = networkPodType

		ifIndex++
	}

	for _, nodeInterface := range conn.NodeInterfaces {

		if ifIndex >= 2 {
			msg := fmt.Sprintf("Too many connection segments specified for a l3pp connection")
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		connNodeName, connInterfaceName := NodeInterfaceNames(nodeInterface)

		nodeInterface, nodeIfType := ctlrPlugin.NetworkNodeMgr.FindInterfaceInNode(connNodeName, connInterfaceName)
		netPodInterfaces[ifIndex] = nodeInterface
		netPodInterfaces[ifIndex].Parent = connNodeName
		networkPodTypes[ifIndex] = nodeIfType
		p2nArray[ifIndex].Node = connNodeName
		p2nArray[ifIndex].Pod = connNodeName
		staticNodesInInterfacesSpecified = true

		ifIndex++
	}

	if !allPodsAssignedToNodes {
		msg := fmt.Sprintf("network-service: %s, not all pods in this connection are mapped to nodes",
			ns.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	if len(conn.NodeInterfaceLabels) != 0 {

		if ifIndex >= 2 {
			msg := fmt.Sprintf("Too many connection segments specified for a l2pp connection")
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		if ifIndex != 1 || len(conn.NodeInterfaceLabels) != 1 {
			msg := fmt.Sprintf("network service: %s, need 1 interface in conn and  1 nodeInterface label: incorrect config",
				ns.Metadata.Name)
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		nodeInterfaces, nodeIfTypes := ctlrPlugin.NetworkNodeMgr.FindInterfacesForThisLabelInNode(p2nArray[0].Node, conn.NodeInterfaceLabels)
		if len(nodeInterfaces) != 1 {
			msg := fmt.Sprintf("network service: %s, nodeLabels %v: must match only 1 node interface: incorrect config",
				ns.Metadata.Name, conn.NodeInterfaceLabels)
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		netPodInterfaces[1] = nodeInterfaces[0]
		netPodInterfaces[1].Parent = p2nArray[0].Node
		networkPodTypes[1] = nodeIfTypes[0]
		p2nArray[1].Node = p2nArray[0].Node
		p2nArray[1].Pod = p2nArray[0].Node
	}

	log.Debugf("RenderConnL3PP: p2nArray=%v, netPodIf=%v, conn=%v", p2nArray, netPodInterfaces, conn)

	// see if the vnfs are on the same node ...
	if p2nArray[0].Node == p2nArray[1].Node {
		return ns.renderConnL3PPSameNode(p2nArray[0].Node, conn, netPodInterfaces, networkPodTypes)
	} else if staticNodesInInterfacesSpecified {
		msg := fmt.Sprintf("netwrok service: %s, nodes %s/%s must be the same",
			ns.Metadata.Name,
			p2nArray[0].Node,
			p2nArray[1].Node)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	// not on same node so ensure there is an NetworkNodeOverlay specified
	if conn.NetworkNodeOverlayName == "" {
		msg := fmt.Sprintf("network-service: %s, %s/%s to %s/%s no node overlay specified",
			ns.Metadata.Name,
			netPodInterfaces[0].Parent, netPodInterfaces[0].Name,
			netPodInterfaces[1].Parent, netPodInterfaces[1].Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	// look up the vnf service mesh
	nno, exists := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationR(conn.NetworkNodeOverlayName)
	if !exists {
		msg := fmt.Sprintf("network-service: %s, %s/%s to %s/%s referencing a missing node overlay: %s",
			ns.Metadata.Name,
			netPodInterfaces[0].Parent, netPodInterfaces[0].Name,
			netPodInterfaces[1].Parent, netPodInterfaces[1].Name,
			conn.NetworkNodeOverlayName)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	// now setup the connection between nodes
	return ns.renderConnL3PPInterNode(conn, connIndex, netPodInterfaces,
		nno, p2nArray, networkPodTypes)
}

// renderConnL3PPSameNode renders this L3PP connection on same node
func (ns *NetworkService) renderConnL3PPSameNode(
	vppAgent string,
	conn *controller.Connection,
	networkPodInterfaces []*controller.Interface,
	networkPodTypes []string) error {

	// if both interfaces are memIf's, we can do a direct inter-vnf memif
	// otherwise, each interface drops into the vswitch and an l2xc is used
	// to connect the interfaces inside the vswitch
	// both interfaces can override direct by specifying "vswitch" as its
	// inter vnf connection type

	memifConnType := controller.IfMemifInterPodConnTypeDirect // assume direct
	for i := 0; i < 2; i++ {
		if networkPodInterfaces[i].MemifParms != nil {
			if networkPodInterfaces[i].MemifParms.InterPodConn != "" &&
				networkPodInterfaces[i].MemifParms.InterPodConn != controller.IfMemifInterPodConnTypeDirect {
				memifConnType = networkPodInterfaces[i].MemifParms.InterPodConn
			}
		}
	}

	if networkPodInterfaces[0].IfType == networkPodInterfaces[1].IfType &&
		networkPodInterfaces[0].IfType == controller.IfTypeMemif &&
		memifConnType == controller.IfMemifInterPodConnTypeDirect {

		err := ns.RenderConnDirectInterPodMemifPair(networkPodInterfaces, controller.IfTypeMemif)
		if err != nil {
			return err
		}

	} else {

		var xconn [2]string
		// render the if's, and then l2xc them
		for i := 0; i < 2; i++ {

			ifName, _, err := ns.RenderConnInterfacePair(vppAgent, conn, networkPodInterfaces[i], networkPodTypes[i])
			if err != nil {
				return err
			}
			xconn[i] = ifName
		}

		for i := 0; i < 2; i++ {
			// create xconns between vswitch side of the container interfaces and the vxlan ifs
			vppKVs := vppagent.ConstructXConnect(vppAgent, xconn[i], xconn[^i&1])
			RenderTxnAddVppEntriesToTxn(ns.Status.RenderedVppAgentEntries,
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				vppKVs)
		}
	}

	return nil
}

// renderConnL3PPInterNode renders this L3PP connection between nodes
func (ns *NetworkService) renderConnL3PPInterNode(
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	nno *NetworkNodeOverlay,
	p2nArray [2]NetworkPodToNodeMap,
	networkPodTypes []string) error {

	var xconn [2][2]string // [0][i] for vnf interfaces [1][i] for vxlan
	var ifStatuses [2]*controller.InterfaceStatus

	if conn.VrfId == 0 {
		conn.VrfId = ctlrPlugin.ramCache.VrfIDAllocator.Allocate()
	}

	// create the interfaces in the containers and vswitch on each node
	for i := 0; i < 2; i++ {

		ifName, ifStatus, err := ns.RenderConnInterfacePair(p2nArray[i].Node, conn, networkPodInterfaces[i], networkPodTypes[i])
		if err != nil {
			return err
		}
		ifStatuses[i] = ifStatus
		xconn[0][i] = ifName

		if len(ifStatus.IpAddresses) != 0 {
			desc := fmt.Sprintf("L3PP NS_%s_VRF_%d_CONN_%d", ns.Metadata.Name, conn.VrfId, connIndex+1)
			l3sr := &controller.L3VRFRoute{
				VrfId:             conn.VrfId,
				Description:       desc,
				DstIpAddr:         vppagent.StripSlashAndSubnetIPAddress(ifStatus.IpAddresses[0]),
				OutgoingInterface: ifName,
			}
			vppKV := vppagent.ConstructStaticRoute(p2nArray[i].Node, l3sr)
			RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
				ModelTypeNetworkService + "/" + ns.Metadata.Name,
				vppKV)
		}
	}

	switch nno.Spec.ConnectionType {
	case controller.NetworkNodeOverlayConnectionTypeVxlan:
		switch nno.Spec.ServiceMeshType {
		case controller.NetworkNodeOverlayTypeMesh:
			return nno.renderConnL3PPVxlanMesh(ns,
				conn,
				connIndex,
				networkPodInterfaces,
				ifStatuses,
				p2nArray,
				xconn)
		case controller.NetworkNodeOverlayTypeHubAndSpoke:
			msg := fmt.Sprintf("vnf-service: %s, conn: %d, %s to %s node overlay: %s type not supported for L3PP",
				ns.Metadata.Name,
				connIndex,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name)
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}
	default:
		msg := fmt.Sprintf("vnf-service: %s, conn: %d, %s to %s node overlay: %s type not implemented",
			ns.Metadata.Name,
			connIndex,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	return nil
}
