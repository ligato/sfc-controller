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

// The L2PP topology is rendered in this module for a connection with a vnf-service

// RenderConnL2PP renders this L2PP connection
func (ns *NetworkService) RenderConnL2PP(
	conn *controller.Connection,
	connIndex uint32) error {

	var p2nArray [2]*NetworkPodToNodeMap
	netPodInterfaces := make([]*controller.Interface, 2)
	networkPodTypes := make([]string, 2)

	allPodsAssignedToNodes := true

	log.Debugf("RenderConnL2PP: num interfaces: %d", len(conn.PodInterfaces))

	// let see if all interfaces in the conn are associated with a node
	for i, connPodInterface := range conn.PodInterfaces {

		connPodName, connInterfaceName := ConnPodInterfaceNames(connPodInterface)

		p2n, exists := ctlrPlugin.ramConfigCache.NetworkPodToNodeMap[connPodName]
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

		p2nArray[i] = p2n
		netPodInterface, networkPodType := ns.findNetworkPodAndInterfaceInList(connPodName,
			connInterfaceName, ns.Spec.NetworkPods)
		netPodInterfaces[i] = netPodInterface
		networkPodTypes[i] = networkPodType
	}

	if !allPodsAssignedToNodes {
		msg := fmt.Sprintf("network-service: %s, not all pods in this connection are mapped to nodes",
			ns.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	log.Debugf("RenderConnL2PP: p2nArray=%v, netPodIf=%v, conn=%v", p2nArray, netPodInterfaces, conn)

	// see if the vnfs are on the same node ...
	if p2nArray[0].Node == p2nArray[1].Node {
		return ns.renderConnL2PPSameNode(p2nArray[0].Node, conn, connIndex,
			netPodInterfaces, networkPodTypes)
	}

	// not on same node so ensure there is an NetworkNodeOverlay specified
	if conn.NetworkNodeOverlayName == "" {
		msg := fmt.Sprintf("network-service: %s, %s to %s no node overlay specified",
			ns.Metadata.Name,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1])
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	// look up the vnf service mesh
	nno, exists := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationR(conn.NetworkNodeOverlayName)
	if !exists {
		msg := fmt.Sprintf("network-service: %s, %s to %s referencing a missing node overlay: %s",
			ns.Metadata.Name,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			conn.NetworkNodeOverlayName)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	// now setup the connection between nodes
	return ns.renderConnL2PPInterNode(conn, connIndex, netPodInterfaces,
		nno, p2nArray, networkPodTypes)
}

// renderConnL2PPSameNode renders this L2PP connection on same node
func (ns *NetworkService) renderConnL2PPSameNode(
	vppAgent string,
	conn *controller.Connection,
	connIndex uint32,
	netPodInterfaces []*controller.Interface,
	networkPodTypes []string) error {

	// if both interfaces are memIf's, we can do a direct inter-vnf memif
	// otherwise, each interface drops into the vswitch and an l2xc is used
	// to connect the interfaces inside the vswitch
	// both interfaces can override direct by specifying "vswitch" as its
	// inter vnf connection type

	memifConnType := controller.IfMemifInterPodConnTypeDirect // assume direct
	for i := 0; i < 2; i++ {
		if netPodInterfaces[i].MemifParms != nil {
			if netPodInterfaces[i].MemifParms.InterPodConn != "" &&
				netPodInterfaces[i].MemifParms.InterPodConn != controller.IfMemifInterPodConnTypeDirect {
				memifConnType = netPodInterfaces[i].MemifParms.InterPodConn
			}
		}
	}

	if netPodInterfaces[0].IfType == netPodInterfaces[1].IfType &&
		netPodInterfaces[0].IfType == controller.IfTypeMemif &&
		memifConnType == controller.IfMemifInterPodConnTypeDirect {

		err := ns.RenderConnDirectInterPodMemifPair(conn, netPodInterfaces, controller.IfTypeMemif)
		if err != nil {
			return err
		}

	} else {

		var xconn [2]string
		// render the if's, and then l2xc them
		for i := 0; i < 2; i++ {

			ifName, err := ns.RenderConnInterfacePair(vppAgent, conn.PodInterfaces[i],
				netPodInterfaces[i], networkPodTypes[i])
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

// renderConnL2PPInterNode renders this L2PP connection between nodes
func (ns *NetworkService) renderConnL2PPInterNode(
	conn *controller.Connection,
	connIndex uint32,
	netPodInterfaces []*controller.Interface,
	nno *NetworkNodeOverlay,
	p2nArray [2]*NetworkPodToNodeMap,
	networkPodTypes []string) error {

	var xconn [2][2]string // [0][i] for vnf interfaces [1][i] for vxlan

	// create the interfaces in the containers and vswitch on each node
	for i := 0; i < 2; i++ {

		ifName, err := ns.RenderConnInterfacePair(p2nArray[i].Node, conn.PodInterfaces[i],
			netPodInterfaces[i], networkPodTypes[i])
		if err != nil {
			return err
		}
		xconn[0][i] = ifName
	}

	switch nno.Spec.ConnectionType {
	case controller.NetworkNodeOverlayConnectionTypeVxlan:
		switch nno.Spec.ServiceMeshType {
		case controller.NetworkNodeOverlayTypeMesh:
			return nno.renderConnL2PPVxlanMesh(ns,
				conn,
				connIndex,
				netPodInterfaces,
				p2nArray,
				xconn)
		case controller.NetworkNodeOverlayTypeHubAndSpoke:
			msg := fmt.Sprintf("vnf-service: %s, conn: %d, %s to %s node overlay: %s type not supported for L2PP",
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
