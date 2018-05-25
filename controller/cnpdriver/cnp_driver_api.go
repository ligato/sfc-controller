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
	"errors"
	"fmt"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/ligato/sfc-controller/controller/cnpdriver/l2driver"
)

var (
	cnpDriverRegistered = false
	cnpDriverName       string
	log = logrus.DefaultLogger()
)

// SfcControllerCNPDriverAPI is interface that is implemented by each Container Networking Policy (CNP) Driver.  As n/b api's
// are invoked, the sfc controller will call methods on this interface for the
// registered plugin to effect the inter-host, intra-host network connectivity.  As well,
// as host-external entity wiring.
// sfc controller natively supports its own container networking policy driver for both
// l3 and l2 overlays.
type SfcControllerCNPDriverAPI interface {
	InitPlugin() error
	DeinitPlugin() error
	GetName() string
	ReconcileStart(vppEtcdLabels map[string]struct{}) error
	ReconcileEnd() error
	DatastoreReInitialize() error
	WireHostEntityToDestinationHostEntity(sh *controller.HostEntity, dh *controller.HostEntity) error
	WireHostEntityToExternalEntity(he *controller.HostEntity, ee *controller.ExternalEntity) error
	WireInternalsForHostEntity(he *controller.HostEntity) error
	WireInternalsForExternalEntity(ee *controller.ExternalEntity) error
	WireSfcEntity(sfc *controller.SfcEntity) error
	SetSystemParameters(sp *controller.SystemParameters) error
	GetSfcInterfaceIPAndMac(container string, port string) (string, string, error)
	Dump()
}

// RegisterCNPDriverPlugin registers the container networking policy driver mode: example: sfcctlr layer 2, ...
func RegisterCNPDriverPlugin(name string, dbFactory func(string) keyval.ProtoBroker) (SfcControllerCNPDriverAPI, error) {

	var cnpDriverAPI SfcControllerCNPDriverAPI

	if cnpDriverRegistered {
		//Commented out because of the test (global variables make testing hard)
		//This change should not harm normal production code.
		//errMsg := fmt.Sprintf("RegisterCNPDriverPlugin: CNPDriver '%s' is currently registered", cnpDriverName)
		//log.Error(errMsg)
		//return nil, errors.New(errMsg)
	}

	switch name {
	case "sfcctlrl2":
		cnpDriverAPI = l2driver.NewSfcCtlrL2CNPDriver(name, dbFactory)
	default:
		errMsg := fmt.Sprintf("RegisterCNPDriverPlugin: CNPDriver '%s' not recognized", name)
		log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	cnpDriverName = name
	cnpDriverRegistered = true

	return cnpDriverAPI, nil
}
