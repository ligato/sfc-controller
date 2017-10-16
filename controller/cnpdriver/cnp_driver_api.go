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

// cnp_driver_api is an interface definition for the container network policy.
// The goal of this interface is to provide generic routines for wiring containers,
// host to hosts, and hosts to external routers.
//
// The current networking policies are the L2 policy with VxLan tunnels between
// the hosts, and between hosts and external routers.  Soon, and IPV6 L3 impl
// will be worked on so this interface is bound to change.

package cnpdriver

import (
	"github.com/ligato/sfc-controller/controller/model/controller"
)

// Each Container Networking Policy (CNP) Driver implements this interface.  As n/b api's
// are invoked, the sfc controller will call methods on this interface for the
// registered plugin to effect the inter-host, intra-host network connectivity.  As well,
// as host-external entity wiring.
// sfc controller natively supports its own container networking policy driver for both
// l3 and l2 overlays.
type SfcControllerCNPDriverAPI interface {
	GetName() string
	ReconcileStart() error
	ReconcileEnd() error

	WireHostEntity
	WireExtEntity
	WireSfcEntity(sfc *controller.SfcEntity) error

	Dump()
}

// WireExtEntity defines methods for wiring/rendering Ext. Entity configuration from SFC Controller model
type WireExtEntity interface {
	WireHostEntityToExternalEntity(he *controller.HostEntity, ee *controller.ExternalEntity) error
	WireInternalsForExternalEntity(ee *controller.ExternalEntity) error
}

// WireHostEntity defines methods for wiring/rendering Host Entity configuration from SFC Controller model
type WireHostEntity interface {
	WireHostEntityToDestinationHostEntity(sh *controller.HostEntity, dh *controller.HostEntity) error
	WireInternalsForHostEntity(he *controller.HostEntity) error
}
