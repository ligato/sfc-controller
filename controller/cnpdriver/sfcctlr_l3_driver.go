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

// The IPV6 L3 CNP implemetation ... coming soon
package cnpdriver

import (
	"github.com/ligato/sfc-controller/controller/model/controller"
)

const (
)

type sfcCtlrL3CNPDriver struct {
	name string
}

// Init the driver/mode for Native SFC Controller L3 Container Networking Policy
func NewSfcCtlrL3CNPDriver(name string) *sfcCtlrL3CNPDriver {

	cnpd := &sfcCtlrL3CNPDriver{}
	cnpd.name = "Sfc Controller L3 Plugin: " + name
	return cnpd
}

// Perform plugin specific initializations
func (cnpd *sfcCtlrL3CNPDriver) InitPlugin() error {
	return nil
}

// Cleanup anything as plugin is being de-reged
func (cnpd *sfcCtlrL3CNPDriver) DeinitPlugin() error {
	return nil
}

// Return user friendly name for this plugin
func (cnpd *sfcCtlrL3CNPDriver) GetName() string {
	return cnpd.name
}

// Perform start processing for the reconcile of the CNP datastore
func (cnpd *sfcCtlrL3CNPDriver) ReconcileStart(vppEtcdLabels map[string]struct{}) error {
	return nil
}


// Perform end processing for the reconcile of the CNP datastore
func (cnpd *sfcCtlrL3CNPDriver) ReconcileEnd() error {
	return nil
}

// Perform CNP specific wiring for "connecting" an external router to a host server
func (cnpd *sfcCtlrL3CNPDriver) wireExternalEntityToHostEntity(ee *controller.ExternalEntity, he *controller.HostEntity) error {

	return nil
}

// Perform CNP specific wiring for "connecting" a source host to a dest host
func (cnpd *sfcCtlrL3CNPDriver) WireHostEntityToDestinationHostEntity(sh *controller.HostEntity, dh *controller.HostEntity) error {
	return nil
}

// Perform CNP specific wiring for "connecting" a host server to an external router
func (cnpd *sfcCtlrL3CNPDriver) WireHostEntityToExternalEntity(he *controller.HostEntity, ee *controller.ExternalEntity) error {
	return nil
}

// Perform CNP specific wiring for "preparing" a host server example: create an east-west bridge
func (cnpd *sfcCtlrL3CNPDriver) WireInternalsForHostEntity(he *controller.HostEntity) error {
	return nil
}

// Perform CNP specific wiring for "preparing" an external entity
func (cnpd *sfcCtlrL3CNPDriver) WireInternalsForExternalEntity(ee *controller.ExternalEntity) error {
	return nil
}

// Perform CNP specific wiring for inter-container wiring, and container to external router wiring
func (cnpd *sfcCtlrL3CNPDriver) WireSfcEntity(sfc *controller.SfcEntity) error {
	return nil
}

// Debug dump routine
func (cnpd *sfcCtlrL3CNPDriver) Dump() {

}
