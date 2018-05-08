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
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/l2"
)

// renderConnL2MPVxlanMesh renders these L2MP tunnels between nodes
func (nno *NetworkNodeOverlay) renderConnL2MPVxlanMesh(
	ns *NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	vnfInterfaces []*controller.Interface,
	p2nArray []*NetworkPodToNodeMap,
	vnfTypes []string,
	nodeMap map[string]bool,
	l2bdIFs map[string][]*l2.BridgeDomains_BridgeDomain_Interfaces) error {

	// The nodeMap contains the set of nodes involved in the l2mp connection.  There
	// must be a vxlan mesh created between the nodes.  On each node, the vnf interfaces
	// will join the l2bd created for this connection, and the vxlan endpoint created
	// below is also associated with this bridge.

	// create the vxlan endpoints
	vniAllocator, exists := ctlrPlugin.NetworkNodeOverlayMgr.vniAllocators[nno.Metadata.Name]
	if !exists {
		msg := fmt.Sprintf("network-service: %s, conn: %d, node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex,
			nno.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}
	vni, err := vniAllocator.AllocateVni()
	if err != nil {
		msg := fmt.Sprintf("network-service: %s, conn: %d, node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex,
			nno.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	// create a vxlan tunnel between each "from" node and "to" node
	for fromNode := range nodeMap {

		for toNode := range nodeMap {

			if fromNode == toNode {
				continue
			}

			ifName := fmt.Sprintf("IF_VXLAN_MESH_VSRVC_%s_CONN_%d_FROM_%s_TO_%s_VNI_%d",
				ns.Metadata.Name, connIndex+1, fromNode, toNode, vni)

			vxlanIPFromAddress, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, fromNode)
			if err != nil {
				msg := fmt.Sprintf("vnf-service: %s, conn: %d, service mesh: %s %s",
					ns.Metadata.Name,
					connIndex,
					nno.Metadata.Name, err)
				ns.AppendStatusMsg(msg)
				return fmt.Errorf(msg)
			}
			vxlanIPToAddress, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, toNode)
			if err != nil {
				msg := fmt.Sprintf("vnf-service: %s, conn: %d, service mesh: %s %s",
					ns.Metadata.Name,
					connIndex,
					nno.Metadata.Name, err)
				ns.AppendStatusMsg(msg)
				return fmt.Errorf(msg)
			}

			vppKV := vppagent.ConstructVxlanInterface(
				fromNode,
				ifName,
				vni,
				vxlanIPFromAddress,
				vxlanIPToAddress)
			RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				vppKV)

			l2bdIF := &l2.BridgeDomains_BridgeDomain_Interfaces{
				Name: ifName,
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       1,
			}
			l2bdIFs[fromNode] = append(l2bdIFs[fromNode], l2bdIF)

			renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanStaticRoutes(fromNode, toNode,
				vxlanIPFromAddress, vxlanIPToAddress,
				nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)

			for k, v := range renderedEntries {
				ns.Status.RenderedVppAgentEntries[k] = v
			}
		}
	}

	// create the perNode lsbd's and add the vnf interfaces
	for nodeName := range nodeMap {
		if err := ns.renderL2BD(conn, connIndex, nodeName, l2bdIFs[nodeName]); err != nil {
			return err
		}
	}

	return nil
}

// renderConnL2PPVxlanMesh renders this L2PP tunnel between nodes
func (nno *NetworkNodeOverlay) renderConnL2PPVxlanMesh(
	ns *NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	p2nArray [2]*NetworkPodToNodeMap,
	xconn [2][2]string) error {

	// The nodeMap contains the set of nodes involved in the l2mp connection.  There
	// must be a vxlan mesh created between the nodes.  On each node, the vnf interfaces
	// will join the l2bd created for this connection, and the vxlan endpoint created
	// below is also associated with this bridge.

	// create the vxlan endpoints
	vniAllocator, exists := ctlrPlugin.NetworkNodeOverlayMgr.vniAllocators[nno.Metadata.Name]
	if !exists {
		msg := fmt.Sprintf("vnf-service: %s, conn: %d, %s to %s node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}
	vni, err := vniAllocator.AllocateVni()
	if err != nil {
		msg := fmt.Sprintf("vnf-service: %s, conn: %d, %s/%s to %s/%s service mesh: %s out of vni's",
			ns.Metadata.Name,
			connIndex,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	for i := 0; i < 2; i++ {

		from := i
		to := ^i & 1

		ifName := fmt.Sprintf("IF_VXLAN_L2PP__NET_SRVC_%s_CONN_%d_FROM_%s_%s_TO_%s_%s_VNI_%d",
			ns.Metadata.Name, connIndex+1,
			p2nArray[from].Node, ConnPodInterfaceSlashToUScore(conn.PodInterfaces[from]),
			p2nArray[to].Node, ConnPodInterfaceSlashToUScore(conn.PodInterfaces[to]),
			vni)

		xconn[1][i] = ifName

		vxlanIPFromAddress, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
			nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, p2nArray[i].Node)
		if err != nil {
			msg := fmt.Sprintf("network-service: %s, conn: %d, %s to %s node overlay: %s, %s",
				ns.Metadata.Name,
				connIndex,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name, err)
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}
		vxlanIPToAddress, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
			nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, p2nArray[^i&1].Node)
		if err != nil {
			msg := fmt.Sprintf("network-service: %s, conn: %d, %s to %s node overlay: %s %s",
				ns.Metadata.Name,
				connIndex,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name, err)
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		vppKV := vppagent.ConstructVxlanInterface(
			p2nArray[i].Node,
			ifName,
			vni,
			vxlanIPFromAddress,
			vxlanIPToAddress)
		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			vppKV)


		renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanStaticRoutes(p2nArray[i].Node, p2nArray[^i&1].Node,
			vxlanIPFromAddress, vxlanIPToAddress,
			nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)

		for k, v := range renderedEntries {
			ns.Status.RenderedVppAgentEntries[k] = v
		}
	}

	// create xconns between vswitch side of the container interfaces and the vxlan ifs
	for i := 0; i < 2; i++ {
		vppKVs := vppagent.ConstructXConnect(p2nArray[i].Node, xconn[0][i], xconn[1][i])
		log.Printf("%v", vppKVs)

		RenderTxnAddVppEntriesToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			vppKVs)
	}

	return nil
}
