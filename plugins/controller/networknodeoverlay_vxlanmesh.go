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

	controller "github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
	l2 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l2"
)

// renderConnL2MPVxlanMesh renders these L2MP tunnels between nodes
func (mgr *NetworkNodeOverlayMgr) renderConnL2MPVxlanMesh(
	nno *controller.NetworkNodeOverlay,
	ns *controller.NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	p2nArray []controller.NetworkPodToNodeMap,
	vnfTypes []string,
	nodeMap map[string]bool,
	l2bdIFs map[string][]*l2.BridgeDomain_Interface) error {

	// The nodeMap contains the set of nodes involved in the l2mp connection.  There
	// must be a vxlan mesh created between the nodes.  On each node, the vnf interfaces
	// will join the l2bd created for this connection, and the vxlan endpoint created
	// below is also associated with this bridge.

	// create the vxlan endpoints
	vniAllocator, exists := ctlrPlugin.NetworkNodeOverlayMgr.vniAllocators[nno.Metadata.Name]
	if !exists {
		msg := fmt.Sprintf("network-service: %s, conn: %d, node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}
	vni, err := vniAllocator.AllocateVni()
	if err != nil {
		msg := fmt.Sprintf("network-service: %s, conn: %d, node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}

	// create a vxlan tunnel between each "from" node and "to" node
	for fromNode := range nodeMap {

		for toNode := range nodeMap {

			if fromNode == toNode {
				continue
			}

			ifName := fmt.Sprintf("IVXMSH_%s_C%d_F_%s_T_%s_V%d",
				ns.Metadata.Name, connIndex+1, fromNode, toNode, vni)

			vxlanIPFromAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, fromNode, nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
			if err != nil {
				msg := fmt.Sprintf("network service: %s, conn: %d, overlay: %s %s",
					ns.Metadata.Name,
					connIndex+1,
					nno.Metadata.Name, err)
				ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
				return fmt.Errorf(msg)
			}
			vxlanIPToAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, toNode, nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
			if err != nil {
				msg := fmt.Sprintf("network service: %s, conn: %d, overlay: %s %s",
					ns.Metadata.Name,
					connIndex+1,
					nno.Metadata.Name, err)
				ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
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

			l2bdIF := &l2.BridgeDomain_Interface{
				Name:                    ifName,
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       1,
			}
			l2bdIFs[fromNode] = append(l2bdIFs[fromNode], l2bdIF)

			renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanLoopbackInterfaceAndStaticRoutes(
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				fromNode, toNode, 0,
				vxlanIPFromAddress, vxlanIPToAddress,
				nno.Spec.VxlanMeshParms.CreateLoopbackInterface,
				nno.Spec.VxlanMeshParms.CreateLoopbackStaticRoutes,
				nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)

			for k, v := range renderedEntries {
				ns.Status.RenderedVppAgentEntries[k] = v
			}
		}
	}

	// create the perNode L2BD's and add the vnf interfaces
	for nodeName := range nodeMap {
		if err := ctlrPlugin.NetworkServiceMgr.RenderL2BD(ns, conn, connIndex, nodeName, l2bdIFs[nodeName], networkPodInterfaces); err != nil {
			return err
		}
	}

	return nil
}

// renderConnL2PPVxlanMesh renders this L2PP tunnel between nodes
func (mgr *NetworkNodeOverlayMgr) renderConnL2PPVxlanMesh(
	nno *controller.NetworkNodeOverlay,
	ns *controller.NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	p2nArray [2]controller.NetworkPodToNodeMap,
	xconn [2][2]string) error {

	// create the vxlan endpoints
	vniAllocator, exists := ctlrPlugin.NetworkNodeOverlayMgr.vniAllocators[nno.Metadata.Name]
	if !exists {
		msg := fmt.Sprintf("network service: %s, conn: %d, %s to %s node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}
	vni, err := vniAllocator.AllocateVni()
	if err != nil {
		msg := fmt.Sprintf("network service: %s, conn: %d, %s/%s overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}

	for i := 0; i < 2; i++ {

		from := i
		to := ^i & 1

		ifName := fmt.Sprintf("IVL2XPP_%s_C%d_F_%s_%s_T_%s_%s_V%d",
			ns.Metadata.Name, connIndex+1,
			p2nArray[from].Node, ConnPodInterfaceSlashToUScore(conn.PodInterfaces[from]),
			p2nArray[to].Node, ConnPodInterfaceSlashToUScore(conn.PodInterfaces[to]),
			vni)

		xconn[1][i] = ifName

		vxlanIPFromAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
			nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, p2nArray[i].Node,
			nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
		if err != nil {
			msg := fmt.Sprintf("network-service: %s, conn: %d, %s to %s node overlay: %s, %s",
				ns.Metadata.Name,
				connIndex+1,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name, err)
			ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
			return fmt.Errorf(msg)
		}
		vxlanIPToAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
			nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, p2nArray[^i&1].Node,
			nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
		if err != nil {
			msg := fmt.Sprintf("network-service: %s, conn: %d, %s to %s node overlay: %s %s",
				ns.Metadata.Name,
				connIndex+1,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name, err)
			ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
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

		renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanLoopbackInterfaceAndStaticRoutes(
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			p2nArray[i].Node, p2nArray[^i&1].Node, 0,
			vxlanIPFromAddress, vxlanIPToAddress,
			nno.Spec.VxlanMeshParms.CreateLoopbackInterface,
			nno.Spec.VxlanMeshParms.CreateLoopbackStaticRoutes,
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

// renderConnL3PPVxlanMesh renders this L3PP tunnel between nodes
func (mgr *NetworkNodeOverlayMgr) renderConnL3PPVxlanMesh(
	nno *controller.NetworkNodeOverlay,
	ns *controller.NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	ifStatuses [2]*controller.InterfaceStatus,
	p2nArray [2]controller.NetworkPodToNodeMap,
	xconn [2][2]string,
	l2bdIFs map[string][]*l2.BridgeDomain_Interface) error {

	// create the vxlan endpoints
	vniAllocator, exists := ctlrPlugin.NetworkNodeOverlayMgr.vniAllocators[nno.Metadata.Name]
	if !exists {
		msg := fmt.Sprintf("network service: %s, conn: %d, %s to %s node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}
	vni, err := vniAllocator.AllocateVni()
	if err != nil {
		msg := fmt.Sprintf("network service: %s, conn: %d, %s/%s overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			conn.PodInterfaces[0],
			conn.PodInterfaces[1],
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}

	for i := 0; i < 2; i++ {

		from := i
		to := ^i & 1

		ifName := fmt.Sprintf("IVXL2PP_%s_C%d_F_%s_%s_T_%s_%s_V%d",
			ns.Metadata.Name, connIndex+1,
			p2nArray[from].Node, ConnPodInterfaceSlashToUScore(conn.PodInterfaces[from]),
			p2nArray[to].Node, ConnPodInterfaceSlashToUScore(conn.PodInterfaces[to]),
			vni)

		xconn[1][i] = ifName

		vxlanIPFromAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
			nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, p2nArray[i].Node,
			nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
		if err != nil {
			msg := fmt.Sprintf("network-service: %s, conn: %d, %s to %s node overlay: %s, %s",
				ns.Metadata.Name,
				connIndex+1,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name, err)
			ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
			return fmt.Errorf(msg)
		}
		vxlanIPToAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
			nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, p2nArray[^i&1].Node,
			nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
		if err != nil {
			msg := fmt.Sprintf("network-service: %s, conn: %d, %s to %s node overlay: %s %s",
				ns.Metadata.Name,
				connIndex+1,
				conn.PodInterfaces[0],
				conn.PodInterfaces[1],
				nno.Metadata.Name, err)
			ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
			return fmt.Errorf(msg)
		}

		//ifStatus, err := InitInterfaceStatus(nno.Metadata.Name, nno.Metadata.Name, iFace)
		//if err != nil {
		//	RemoveInterfaceStatus(nno.Status.Interfaces, iFace.Parent, iFace.Name)
		//	msg := fmt.Sprintf("network node overlay tunnel: %s/%s, %s", iFace.Parent, iFace.Name, err)
		//	nno.AppendStatusMsg(msg)
		//	return err
		//}
		//iFace.Vni = vni
		//PersistInterfaceStatus(nno.Status.Interfaces, ifStatus, iFace.Parent, iFace.Name)

		vppKV := vppagent.ConstructVxlanInterface(
			p2nArray[i].Node,
			ifName,
			vni,
			vxlanIPFromAddress,
			vxlanIPToAddress)

		vppKV.IFace.Vrf = conn.VrfId

		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			vppKV)

		renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanLoopbackInterfaceAndStaticRoutes(
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			p2nArray[i].Node, p2nArray[^i&1].Node, conn.VrfId,
			vxlanIPFromAddress, vxlanIPToAddress,
			nno.Spec.VxlanMeshParms.CreateLoopbackInterface,
			nno.Spec.VxlanMeshParms.CreateLoopbackStaticRoutes,
			nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)

		for k, v := range renderedEntries {
			ns.Status.RenderedVppAgentEntries[k] = v
		}

		// now create a route for the dest container's ipaddress using the local vxlan tunnel
		if len(ifStatuses[to].IpAddresses) != 0 {
			desc := fmt.Sprintf("L3PP NS_%s_VRF_%d_CONN_%d", ns.Metadata.Name, conn.VrfId, connIndex+1)

			l3sr := &controller.L3VRFRoute{
				Vpp: &controller.VPPRoute{
					VrfId:             conn.VrfId,
					Description:       desc,
					DstIpAddr:         vppagent.StripSlashAndSubnetIPAddress(ifStatuses[to].IpAddresses[0]),
					OutgoingInterface: ifName,
				},
			}
			vppKV := vppagent.ConstructStaticRoute(p2nArray[from].Node, l3sr)
			RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				vppKV)
		}

	}

	return nil
}

// renderConnL2MPVxlanMesh renders these L2MP tunnels between nodes
func (mgr *NetworkNodeOverlayMgr) renderConnL3MPVxlanMesh(
	nno *controller.NetworkNodeOverlay,
	ns *controller.NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	p2nArray []controller.NetworkPodToNodeMap,
	vnfTypes []string,
	nodeMap map[string]bool,
	l3vrfs map[string][]*controller.L3VRFRoute,
	l2bdIFs map[string][]*l2.BridgeDomain_Interface) error {

	// The nodeMap contains the set of nodes involved in the l2mp connection.  There
	// must be a vxlan mesh created between the nodes.

	// create the vxlan endpoints
	vniAllocator, exists := ctlrPlugin.NetworkNodeOverlayMgr.vniAllocators[nno.Metadata.Name]
	if !exists {
		msg := fmt.Sprintf("network-service: %s, conn: %d, node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}
	vni, err := vniAllocator.AllocateVni()
	if err != nil {
		msg := fmt.Sprintf("network-service: %s, conn: %d, node overlay: %s out of vni's",
			ns.Metadata.Name,
			connIndex+1,
			nno.Metadata.Name)
		ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)
	}

	tunnelMeshMap := make(map[string]string)

	// create a vxlan tunnel between each "from" node and "to" node
	// create l3vrf from each "to" node towards the "from" node tunnel
	for fromNode := range nodeMap {

		for toNode := range nodeMap {

			if fromNode == toNode {
				continue
			}

			ifName := fmt.Sprintf("IVXMSH_%s_C%d_F_%s_T_%s_V%d",
				ns.Metadata.Name, connIndex+1, fromNode, toNode, vni)

			tunnelMeshMap[fromNode+"/"+toNode] = ifName

			vxlanIPFromAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, fromNode, nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
			if err != nil {
				msg := fmt.Sprintf("network service: %s, conn: %d, overlay: %s %s",
					ns.Metadata.Name,
					connIndex+1,
					nno.Metadata.Name, err)
				ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
				return fmt.Errorf(msg)
			}
			vxlanIPToAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanMeshParms.LoopbackIpamPoolName, toNode, nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)
			if err != nil {
				msg := fmt.Sprintf("network service: %s, conn: %d, overlay: %s %s",
					ns.Metadata.Name,
					connIndex+1,
					nno.Metadata.Name, err)
				ctlrPlugin.NetworkServiceMgr.AppendStatusMsg(ns, msg)
				return fmt.Errorf(msg)
			}

			vppKV := vppagent.ConstructVxlanInterface(
				fromNode,
				ifName,
				vni,
				vxlanIPFromAddress,
				vxlanIPToAddress)

			vppKV.IFace.Vrf = conn.VrfId

			RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				vppKV)

			renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanLoopbackInterfaceAndStaticRoutes(
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				fromNode, toNode, conn.VrfId,
				vxlanIPFromAddress, vxlanIPToAddress,
				nno.Spec.VxlanMeshParms.CreateLoopbackInterface,
				nno.Spec.VxlanMeshParms.CreateLoopbackStaticRoutes,
				nno.Spec.VxlanMeshParms.NetworkNodeInterfaceLabel)

			for k, v := range renderedEntries {
				ns.Status.RenderedVppAgentEntries[k] = v
			}

			// fromNode needs a static entry for each container residing in the toNode using the
			// outgoing tunnel from the fromNode to the toNode
			for _, toNodel3Vrf := range l3vrfs[toNode] {

				l3sr := &controller.L3VRFRoute{
					Vpp: &controller.VPPRoute{
						VrfId:             toNodel3Vrf.GetVpp().VrfId,
						DstIpAddr:         toNodel3Vrf.GetVpp().DstIpAddr,
						OutgoingInterface: ifName,
						Description:       toNodel3Vrf.GetVpp().Description,
					},
				}
				vppKV := vppagent.ConstructStaticRoute(fromNode, l3sr)
				RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
					ModelTypeNetworkService+"/"+ns.Metadata.Name,
					vppKV)
			}

			l2bdIF := &l2.BridgeDomain_Interface{
				Name:                    ifName,
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       1,
			}
			l2bdIFs[fromNode] = append(l2bdIFs[fromNode], l2bdIF)
		}
	}

	// create the perNode L2BD's and add the vnf interfaces
	for nodeName := range nodeMap {
		if err := ctlrPlugin.NetworkServiceMgr.RenderL2BD(ns, conn, connIndex, nodeName, l2bdIFs[nodeName], networkPodInterfaces); err != nil {
			return err
		}
	}

	return nil
}
