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
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
)

// Types in the model were defined as strings for readability not enums with
// numbers
const (
	VppEntryTypeInterface      = "interface"
	VppEntryTypeLinuxInterface = "linuxif"
	VppEntryTypeL2BD           = "l2bd"
	VppEntryTypeL3Route        = "l3vrf"
	VppEntryTypeL2XC           = "l2xc"
)

// KVType tracks each allocated key/value vnf/vpp agent
type KVType struct {
	modelType    string
	VppKey       string
	VppEntryType string
	IFace        *interfaces.Interfaces_Interface     `json:"IFace,omitempty"`
	L2BD         *l2.BridgeDomains_BridgeDomain       `json:"L2BD,omitempty"`
	L3Route      *l3.StaticRoutes_Route               `json:"L3Route,omitempty"`
	XConn        *l2.XConnectPairs_XConnectPair       `json:"XConn,omitempty"`
	LinuxIFace   *linuxIntf.LinuxInterfaces_Interface `json:"LinuxIFace,omitempty"`
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
func (kv *KVType) InterfaceSet(iface *interfaces.Interfaces_Interface) {
	kv.IFace = iface
}

// L3StaticRouteSet updates the interface
func (kv *KVType) L3StaticRouteSet(l3sr *l3.StaticRoutes_Route) {
	kv.L3Route = l3sr
}

// LinuxInterfaceSet updates the interface
func (kv *KVType) LinuxInterfaceSet(iface *linuxIntf.LinuxInterfaces_Interface) {
	kv.LinuxIFace = iface
}

// L2BDSet updates the interface
func (kv *KVType) L2BDSet(l2bd *l2.BridgeDomains_BridgeDomain) {
	kv.L2BD = l2bd
}

// L2XCSet updates the interface
func (kv *KVType) L2XCSet(l2xc *l2.XConnectPairs_XConnectPair) {
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
		l3Route1.Description = ""
		l3Route2 := *kv2.L3Route
		l3Route2.Description = ""
		if l3Route1.String() != l3Route2.String() {
			return false
		}
	default:
		log.Errorf("Equal: unknown interface type: %v", kv)
		return false
	}

	return true
}

// WriteToEtcd puts the vppkey and value into etcd
func (kv *KVType) WriteToEtcd(db keyval.ProtoBroker) error {

	var err error
	switch kv.VppEntryType {
	case VppEntryTypeInterface:
		err = database.WriteToDatastore(kv.VppKey, kv.IFace)
	case VppEntryTypeL2BD:
		err = database.WriteToDatastore(kv.VppKey, kv.L2BD)
	case VppEntryTypeL2XC:
		err = database.WriteToDatastore(kv.VppKey, kv.XConn)
	case VppEntryTypeLinuxInterface:
		err = database.WriteToDatastore(kv.VppKey, kv.LinuxIFace)
	case VppEntryTypeL3Route:
		err = database.WriteToDatastore(kv.VppKey, kv.L3Route)
	default:
		msg := fmt.Sprintf("WriteToEtcd: unknown vpp entry type: %v", kv)
		log.Errorf(msg)
		err = fmt.Errorf(msg)
	}
	return err
}

// ReadFromEtcd gets the vppkey and value into etcd
func (kv *KVType) ReadFromEtcd(db keyval.ProtoBroker) (bool, error) {

	var err error
	var found bool

	switch kv.VppEntryType {
	case VppEntryTypeInterface:
		iface := &interfaces.Interfaces_Interface{}
		found, _, err = db.GetValue(kv.VppKey, iface)
		if found && err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, iface)
			kv.InterfaceSet(iface)
		}
	case VppEntryTypeLinuxInterface:
		iface := &linuxIntf.LinuxInterfaces_Interface{}
		found, _, err = db.GetValue(kv.VppKey, iface)
		if found && err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, iface)
			kv.LinuxInterfaceSet(iface)
		}
	case VppEntryTypeL2BD:
		l2bd := &l2.BridgeDomains_BridgeDomain{}
		found, _, err = db.GetValue(kv.VppKey, l2bd)
		if found && err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l2bd)
			kv.L2BDSet(l2bd)
		}
	case VppEntryTypeL2XC:
		l2xc := &l2.XConnectPairs_XConnectPair{}
		found, _, err = db.GetValue(kv.VppKey, l2xc)
		if found && err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l2xc)
			kv.L2XCSet(l2xc)
		}
	case VppEntryTypeL3Route:
		l3sr := &l3.StaticRoutes_Route{}
		found, _, err = db.GetValue(kv.VppKey, l3sr)
		if found && err == nil {
			log.Debugf("ReadFromEtcd: read etcd key %s: %v", kv.VppKey, l3sr)
			kv.L3StaticRouteSet(l3sr)
		}
	default:
		msg := fmt.Sprintf("ReadFromEtcd: unsupported vpp entry type: %v", kv)
		log.Errorf(msg)
		err = fmt.Errorf(msg)
	}
	return found, err
}

// DeleteFromDatastore removes the specified entry fron etcd
func (kv *KVType) DeleteFromDatastore() {

	log.Debugf("DeleteFromDatastore: key: '%s'", kv.VppKey)
	database.DeleteFromDatastore(kv.VppKey)
}
