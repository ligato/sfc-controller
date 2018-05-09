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

// The core plugin which drives the SFC Controller.  The core initializes the
// CNP dirver plugin based on command line args.  The database is initialized,
// and a resync is preformed based on what was already in the database.

package controller

import ()

// Types in the model were defined as strings for readability not enums with
// numbers
const (
	IfTypeLoopBack    = "loopback"
	IfTypeEthernet    = "ethernet"
	IfTypeVxlanTunnel = "vxlan_tunnel"
	IfTypeMemif       = "memif"
	IfTypeVeth        = "veth"
	IfTypeTap         = "tap"

	IfAdminStatusEnabled  = "enabled"
	IfAdminStatusDisabled = "disabled"

	IfMemifModeEhernet             = "ethernet"
	IfMemifModeIP                  = "ip"
	IfMemifModePuntInject          = "puntinject"
	IfMemifInterPodConnTypeDirect  = "direct"
	IfMemifInterPodConnTypeVswitch = "via-vswitch"

	NetworkPodTypeVPPVswitch      = "vppvswitch"
	NetworkPodTypeExternal        = "external"
	NetworkPodTypeVPPContainer    = "vppcontainer"
	NetworkPodTypeNonVPPContainer = "nonvppcontainer"

	RxModeInterrupt = "interrupt"
	RxModePolling   = "polling"
	RxModeAdaptive  = "adaptive"

	ConnTypeL2PP = "l2pp"
	ConnTypeL2MP = "l2mp"

	OperStatusUp   = "OperUp"
	OperStatusDown = "OperDown"

	NetworkNodeOverlayTypeMesh        = "mesh"
	NetworkNodeOverlayTypeHubAndSpoke = "hub_and_spoke"

	NetworkNodeOverlayConnectionTypeVxlan = "vxlan"

	IPAMPoolScopeAny            = "any" // dont use this, this is for cleaning up
	IPAMPoolScopeSystem         = "system"
	IPAMPoolScopeNode           = "node"
	IPAMPoolScopeNetworkService = "network_service"
)
