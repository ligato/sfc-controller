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

package vppagent

import (
	"fmt"

	"github.com/ligato/sfc-controller/plugins/controller/database"
	linuxIntf "go.ligato.io/vpp-agent/v3/proto/ligato/linux/interfaces"
	linuxL3 "go.ligato.io/vpp-agent/v3/proto/ligato/linux/l3"
	interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	l2 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l2"
	l3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"
)

// Types in the model were defined as strings for readability not enums with
// numbers
const (
	VppEntryTypeInterface      = "interface"
	VppEntryTypeLinuxInterface = "linuxif"
	VppEntryTypeL2BD           = "l2bd"
	VppEntryTypeL2Fib          = "l2fib"
	VppEntryTypeL3Route        = "l3route"
	VppEntryTypeL3LinuxRoute   = "l3linuxroute"
	VppEntryTypeL2XC           = "l2xc"
	VppEntryTypeArp            = "arp"
	VppEntryTypeLinuxArp       = "linuxarp"
)

// KVType tracks each allocated key/value vnf/vpp agent
type KVType struct {
	modelType    string
	VppKey       string
	VppEntryType string
	IFace        *interfaces.Interface `json:"IFace,omitempty"`
	L2BD         *l2.BridgeDomain      `json:"L2BD,omitempty"`
	L2Fib        *l2.FIBEntry          `json:"L2Fib,omitempty"`
	LinuxL3Route *linuxL3.Route        `json:"LinuxL3Route,omitempty"`
	L3Route      *l3.Route             `json:"L3Route,omitempty"`
	XConn        *l2.XConnectPair      `json:"XConn,omitempty"`
	LinuxIFace   *linuxIntf.Interface  `json:"LinuxIFace,omitempty"`
	ArpEntry     *l3.ARPEntry          `json:"ArpEntry,omitempty"`
	LinuxArpEntry     *linuxL3.ARPEntry          `json:"LinuxArpEntry,omitempty"`
}

// NewKVEntry initializes a vpp KV entry type
func NewKVEntry(vppKey string, vppEntryType string) *KVType {
	kv := &KVType{
		VppKey:       vppKey,
		VppEntryType: vppEntryType,
	}
	return kv
}

// InterfaceSet updates the interface
func (kv *KVType) InterfaceSet(iface *interfaces.Interface) {
	kv.IFace = iface
}

// L3StaticRouteSet updates the static route
func (kv *KVType) L3StaticRouteSet(l3sr *l3.Route) {
	kv.L3Route = l3sr
}

// L3StaticLinuxRouteSet updates the static linux route
func (kv *KVType) L3StaticLinuxRouteSet(l3sr *linuxL3.Route) {
	kv.LinuxL3Route = l3sr
}

// L2StaticFibSet updates the static fib
func (kv *KVType) L2StaticFibSet(l2fib *l2.FIBEntry) {
	kv.L2Fib = l2fib
}

// ArpEntrySet updates the arp entry
func (kv *KVType) ArpEntrySet(ae *l3.ARPEntry) {
	kv.ArpEntry = ae
}

// LinuxArpEntrySet updates the arp entry
func (kv *KVType) LinuxArpEntrySet(ae *linuxL3.ARPEntry) {
	kv.LinuxArpEntry = ae
}

// LinuxInterfaceSet updates the interface
func (kv *KVType) LinuxInterfaceSet(iface *linuxIntf.Interface) {
	kv.LinuxIFace = iface
}

// L2BDSet updates the interface
func (kv *KVType) L2BDSet(l2bd *l2.BridgeDomain) {
	kv.L2BD = l2bd
}

// L2XCSet updates the interface
func (kv *KVType) L2XCSet(l2xc *l2.XConnectPair) {
	kv.XConn = l2xc
}

// Equal updates the interface
func (kv *KVType) Equal(kv2 *KVType) bool {
	if kv.VppEntryType != kv2.VppEntryType {
		return false
	}
	if kv.VppKey != kv2.VppKey {
		return false
	}
	switch kv.VppEntryType {
	case VppEntryTypeInterface:
		if kv.IFace.String() != kv2.IFace.String() {
			return false
		}
	case VppEntryTypeL2BD:
		if kv.L2BD.String() != kv2.L2BD.String() {
			return false
		}
	case VppEntryTypeL2Fib:
		if kv.L2Fib.String() != kv2.L2Fib.String() {
			return false
		}
	case VppEntryTypeL2XC:
		if kv.XConn.String() != kv2.XConn.String() {
			return false
		}
	case VppEntryTypeLinuxInterface:
		if kv.LinuxIFace.String() != kv2.LinuxIFace.String() {
			return false
		}
	case VppEntryTypeL3Route:
		// disregard route description when comparing routes
		l3Route1 := *kv.L3Route
		l3Route2 := *kv2.L3Route
		if l3Route1.String() != l3Route2.String() {
			return false
		}
	case VppEntryTypeL3LinuxRoute:
		// disregard route description when comparing routes
		l3Route1 := *kv.LinuxL3Route
		l3Route2 := *kv2.LinuxL3Route
		if l3Route1.String() != l3Route2.String() {
			return false
		}
	case VppEntryTypeArp:
		if kv.ArpEntry.String() != kv2.ArpEntry.String() {
			return false
		}
	case VppEntryTypeLinuxArp:
		if kv.LinuxArpEntry.String() != kv2.LinuxArpEntry.String() {
			return false
		}
	default:
		log.Errorf("Equal: unknown interface type: %v", kv)
		return false
	}

	return true
}

// WriteToKVStore puts the vppkey and value into KV store
func (kv *KVType) WriteToKVStore() error {

	var err error
	switch kv.VppEntryType {
	case VppEntryTypeInterface:
		err = database.WriteToDatastore(kv.VppKey, kv.IFace)
	case VppEntryTypeL2BD:
		err = database.WriteToDatastore(kv.VppKey, kv.L2BD)
	case VppEntryTypeL2Fib:
		err = database.WriteToDatastore(kv.VppKey, kv.L2Fib)
	case VppEntryTypeL2XC:
		err = database.WriteToDatastore(kv.VppKey, kv.XConn)
	case VppEntryTypeLinuxInterface:
		err = database.WriteToDatastore(kv.VppKey, kv.LinuxIFace)
	case VppEntryTypeL3Route:
		err = database.WriteToDatastore(kv.VppKey, kv.L3Route)
	case VppEntryTypeL3LinuxRoute:
		err = database.WriteToDatastore(kv.VppKey, kv.LinuxL3Route)
	case VppEntryTypeArp:
		err = database.WriteToDatastore(kv.VppKey, kv.ArpEntry)
	case VppEntryTypeLinuxArp:
		err = database.WriteToDatastore(kv.VppKey, kv.LinuxArpEntry)
	default:
		msg := fmt.Sprintf("WriteToEtcd: unknown vpp entry type: %v", kv)
		log.Errorf(msg)
		err = fmt.Errorf(msg)
	}
	return err
}

// ReadFromKVStore gets the vppkey and value from KVStore
func (kv *KVType) ReadFromKVStore() error {

	var err error

	switch kv.VppEntryType {
	case VppEntryTypeInterface:
		iface := &interfaces.Interface{}
		err = database.ReadFromDatastore(kv.VppKey, iface)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, iface)
			kv.InterfaceSet(iface)
		}
	case VppEntryTypeLinuxInterface:
		iface := &linuxIntf.Interface{}
		err = database.ReadFromDatastore(kv.VppKey, iface)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, iface)
			kv.LinuxInterfaceSet(iface)
		}
	case VppEntryTypeL2BD:
		l2bd := &l2.BridgeDomain{}
		err = database.ReadFromDatastore(kv.VppKey, l2bd)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l2bd)
			kv.L2BDSet(l2bd)
		}
	case VppEntryTypeL2Fib:
		l2fib := &l2.FIBEntry{}
		err = database.ReadFromDatastore(kv.VppKey, l2fib)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l2fib)
			kv.L2StaticFibSet(l2fib)
		}
	case VppEntryTypeL2XC:
		l2xc := &l2.XConnectPair{}
		err = database.ReadFromDatastore(kv.VppKey, l2xc)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l2xc)
			kv.L2XCSet(l2xc)
		}
	case VppEntryTypeL3Route:
		l3sr := &l3.Route{}
		err = database.ReadFromDatastore(kv.VppKey, l3sr)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l3sr)
			kv.L3StaticRouteSet(l3sr)
		}
	case VppEntryTypeL3LinuxRoute:
		l3sr := &linuxL3.Route{}
		err = database.ReadFromDatastore(kv.VppKey, l3sr)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l3sr)
			kv.L3StaticLinuxRouteSet(l3sr)
		}
	case VppEntryTypeArp:
		ae := &l3.ARPEntry{}
		err = database.ReadFromDatastore(kv.VppKey, ae)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, ae)
			kv.ArpEntrySet(ae)
		}
	case VppEntryTypeLinuxArp:
		ae := &linuxL3.ARPEntry{}
		err = database.ReadFromDatastore(kv.VppKey, ae)
		if err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, ae)
			kv.LinuxArpEntrySet(ae)
		}
	default:
		msg := fmt.Sprintf("ReadFromEtcd: unsupported vpp entry type: %v", kv)
		log.Errorf(msg)
		err = fmt.Errorf(msg)
	}
	return err
}

// DeleteFromDatastore removes the specified entry fron etcd
func (kv *KVType) DeleteFromDatastore() {

	log.Debugf("DeleteFromDatastore: key: '%s'", kv.VppKey)
	database.DeleteFromDatastore(kv.VppKey)
}
