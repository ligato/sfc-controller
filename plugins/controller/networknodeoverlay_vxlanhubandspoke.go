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
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// renderConnL2MPVxlanHubAndSpoke renders these L2MP tunnels between hub and nodes
func (nno *NetworkNodeOverlay) renderConnL2MPVxlanHubAndSpoke(
	ns *NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	networkPodInterfaces []*controller.Interface,
	p2nArray []NetworkPodToNodeMap,
	vnfTypes []string,
	spokeNodeMap map[string]bool,
	l2bdIFs map[string][]*l2.BridgeDomains_BridgeDomain_Interfaces) error {

	// The spokeNodeMap contains the set of nodes involved in the l2mp connection.  The node
	// in the node network node overlay is the hub and this set of nodes in the nodeMap are the spokes.
	// Need to create an l2bd on the hub node and add each of the vxlan tunnels to it, also
	// need to create an l2bd on each of the spoke nodes and add the vxlan spoke to it.
	// Note that the l2bdIFs map passed into this function in the input parameters already
	// has the per spoke vnf interfaces in it.

	hubNodeName := nno.Spec.VxlanHubAndSpokeParms.HubNodeName
	if _, exists := ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationR(hubNodeName); !exists {
		msg := fmt.Sprintf("network-service: %s, conn: %d, network node overlay: %s, hub_node: %s not found",
			ns.Metadata.Name,
			connIndex+1,
			nno.Metadata.Name,
			hubNodeName)
		ns.AppendStatusMsg(msg)
		return fmt.Errorf(msg)
	}

	vni := nno.Spec.VxlanHubAndSpokeParms.Vni

	// for each spoke, create a hub-spoke and spoke-hub vxlan i/f
	for spokeNodeName := range spokeNodeMap {

		if hubNodeName == spokeNodeName {
			msg := fmt.Sprintf("network-service: %s, conn: %d, network node overlay: %s hub node same as spoke %s",
				ns.Metadata.Name,
				connIndex+1,
				nno.Metadata.Name, hubNodeName)
			ns.AppendStatusMsg(msg)
			return fmt.Errorf(msg)
		}

		// create both ends of the tunnel, hub end and spoke end
		hubAndSpokeNodePair := []string{hubNodeName, spokeNodeName}
		for i := range hubAndSpokeNodePair {

			fromNode := hubAndSpokeNodePair[i]
			toNode := hubAndSpokeNodePair[^i&1]

			var ifName string
			if i == 0 {
				ifName = fmt.Sprintf("IF_VXLAN_NET_SRVC_%s_CONN_%d_FROM_HUB_%s_TO_SPOKE_%s_VNI_%d",
					ns.Metadata.Name, connIndex+1, fromNode, toNode, vni)
			} else {
				ifName = fmt.Sprintf("IF_VXLAN_NET_SRVC_%s_CONN_%d_FROM_SPOKE_%s_TO_HUB_%s_VNI_%d",
					ns.Metadata.Name, connIndex+1, fromNode, toNode, vni)
			}

			vxlanIPFromAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanHubAndSpokeParms.LoopbackIpamPoolName, fromNode,
				nno.Spec.VxlanHubAndSpokeParms.NetworkNodeInterfaceLabel)
			if err != nil {
				msg := fmt.Sprintf("network-service: %s, conn: %d, network node overlay: %s %s",
					ns.Metadata.Name,
					connIndex+1,
					nno.Metadata.Name, err)
				ns.AppendStatusMsg(msg)
				return fmt.Errorf(msg)
			}
			vxlanIPToAddress, _, err := ctlrPlugin.NetworkNodeOverlayMgr.AllocateVxlanAddress(
				nno.Spec.VxlanHubAndSpokeParms.LoopbackIpamPoolName, toNode,
				nno.Spec.VxlanHubAndSpokeParms.NetworkNodeInterfaceLabel)
			if err != nil {
				msg := fmt.Sprintf("network-service: %s, conn: %d, network node overlay: %s %s",
					ns.Metadata.Name,
					connIndex+1,
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

			// internode VNFs reach each other via the hub node, need split-horizon 0
			l2bdIF := &l2.BridgeDomains_BridgeDomain_Interfaces{
				Name: ifName,
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       0,
			}
			l2bdIFs[fromNode] = append(l2bdIFs[fromNode], l2bdIF)

			renderedEntries := ctlrPlugin.NetworkNodeMgr.RenderVxlanLoopbackInterfaceAndStaticRoutes(
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				fromNode, toNode, 0,
				vxlanIPFromAddress, vxlanIPToAddress,
				nno.Spec.VxlanHubAndSpokeParms.CreateLoopbackInterface,
				nno.Spec.VxlanHubAndSpokeParms.CreateLoopbackStaticRoutes,
				nno.Spec.VxlanHubAndSpokeParms.NetworkNodeInterfaceLabel)

			for k, v := range renderedEntries {
				ns.Status.RenderedVppAgentEntries[k] = v
			}
		}
	}

	// create the spoke node l2bd's and add the vnf interfaces and vxlan if's from abve
	for nodeName := range spokeNodeMap {
		if err := ns.RenderL2BD(conn, connIndex, nodeName, l2bdIFs[nodeName]); err != nil {
			return err
		}
	}
	// create the hub l2bd and add the vxaln if's from above
	if err := ns.RenderL2BD(conn, connIndex, hubNodeName, l2bdIFs[hubNodeName]); err != nil {
		return err
	}

	return nil
}
