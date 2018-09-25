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

// The vppagent provides routines to create objects like interfaces, BDs,
// routes, etc.  Each routine will return the key for the object in etcd, and
// the object itself.

package vppagent

import (
	"sort"
	"strings"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/addrs"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

var (
	log = logrus.DefaultLogger()
)

func rxModeControllerToInterface(contrtollerRxMode string) *interfaces.Interfaces_Interface_RxModeSettings {

	rxSettings := &interfaces.Interfaces_Interface_RxModeSettings{}
	switch contrtollerRxMode {
	case controller.RxModePolling:
		rxSettings.RxMode = interfaces.RxModeType_POLLING
		return rxSettings
	case controller.RxModeInterrupt:
		rxSettings.RxMode = interfaces.RxModeType_INTERRUPT
		return rxSettings
	case controller.RxModeAdaptive:
		rxSettings.RxMode = interfaces.RxModeType_ADAPTIVE
		return rxSettings
	}
	return nil
}

// ConstructL2BD returns an KVType
func ConstructL2BD(vppAgent string,
	bdName string,
	ifaces []*l2.BridgeDomains_BridgeDomain_Interfaces,
	bdParms *controller.BDParms) *KVType {

	// keep the ifaces sorted so subsequent comparing during transaction processing
	// is easier as it does a String() then compares strings and an unsorted array
	// banjaxes the compare if two bridges are equal but have a same set of iFaces but
	// are in a different order
	var sortedInterfaces []*l2.BridgeDomains_BridgeDomain_Interfaces
	for _, iface := range ifaces {
		sortedInterfaces = insertSortBridgedInterface(sortedInterfaces, iface)
	}

	l2bd := &l2.BridgeDomains_BridgeDomain{
		Name:                bdName,
		Flood:               bdParms.Flood,
		UnknownUnicastFlood: bdParms.UnknownUnicastFlood,
		Forward:             bdParms.Forward,
		Interfaces:          sortedInterfaces,
		Learn:               bdParms.Learn,
		ArpTermination:      bdParms.ArpTermination,
		MacAge:              bdParms.MacAgeMinutes,
	}

	key := L2BridgeDomainKey(vppAgent, bdName)

	log.Debugf("ConstructL2BD: key='%s', l2bd='%v", key, l2bd)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeL2BD,
		L2BD:         l2bd,
	}
	return kv
}

// AppendInterfacesToL2BD returns an KVType
func AppendInterfacesToL2BD(vppAgent string,
	l2bd *l2.BridgeDomains_BridgeDomain,
	ifaces []*l2.BridgeDomains_BridgeDomain_Interfaces) *KVType {

	// keep the ifaces sorted so subsequent comparing during transaction processing
	// is easier as it does a String() then compares strings and an unsorted array
	// banjaxes the compare if two bridges are equal but have a same set of iFaces but
	// are in a different order
	sortedInterfaces := l2bd.Interfaces // start with the existing sorted set
	for _, iface := range ifaces {
		sortedInterfaces = insertSortBridgedInterface(sortedInterfaces, iface)
	}
	l2bd.Interfaces = sortedInterfaces

	key := L2BridgeDomainKey(vppAgent, l2bd.Name)

	log.Debugf("AppendInterfacesToL2BD: key='%s', l2bd='%v", key, l2bd)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeL2BD,
		L2BD:         l2bd,
	}
	return kv
}

func insertSortBridgedInterface(
	data []*l2.BridgeDomains_BridgeDomain_Interfaces,
	el *l2.BridgeDomains_BridgeDomain_Interfaces) []*l2.BridgeDomains_BridgeDomain_Interfaces {
	index := sort.Search(len(data), func(i int) bool { return data[i].Name > el.Name })
	data = append(data, &l2.BridgeDomains_BridgeDomain_Interfaces{})
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}

// ConstructEthernetInterface returns an KVType
func ConstructEthernetInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	rxMode string) *KVType {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_ETHERNET_CSMACD,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
	}

	iface.RxModeSettings = rxModeControllerToInterface(rxMode)

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructEthernetInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

func sortedIPAddresses(ipAddresses []string) []string {
	// keep the addresses sorted so subsequent comparing during transaction processing
	// is easier as it does a String() then compares strings and an unsorted array
	// banjaxes the compare if two interfaces are equal but have a same set of addresses but
	// are in a different order
	var sortedAddresses []string
	for _, ipAddress := range ipAddresses {
		sortedAddresses = insertSortIPAddress(sortedAddresses, ipAddress)
	}
	return sortedAddresses
}

func insertSortIPAddress(
	data []string,
	el string) []string {
	index := sort.Search(len(data), func(i int) bool { return data[i] > el })
	var x string
	data = append(data, x)
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}

// ConstructLoopbackInterface returns an KVType
func ConstructLoopbackInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	rxMode string) *KVType {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_SOFTWARE_LOOPBACK,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
	}

	iface.RxModeSettings = rxModeControllerToInterface(rxMode)

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructLoopbackInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

func adminStatusStringToBool(adminStatusString string) bool {
	if adminStatusString == controller.IfAdminStatusDisabled {
		return false
	}
	return true
}

func memifMode(modeString string) interfaces.Interfaces_Interface_Memif_MemifMode {

	mode := interfaces.Interfaces_Interface_Memif_ETHERNET
	if modeString != "" {
		switch modeString {
		case controller.IfMemifModeEhernet:
			mode = interfaces.Interfaces_Interface_Memif_ETHERNET
		case controller.IfMemifModeIP:
			mode = interfaces.Interfaces_Interface_Memif_IP
		case controller.IfMemifModePuntInject:
			mode = interfaces.Interfaces_Interface_Memif_PUNT_INJECT
		}
	}
	return mode
}

// ConstructMemInterface returns an KVType
func ConstructMemInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	rxMode string,
	memifID uint32,
	isMaster bool,
	memifParms *controller.Interface_MemIFParms,
	defaultMemifDirectory string,
	masterVppAgent string) *KVType {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_MEMORY_INTERFACE,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		Memif: &interfaces.Interfaces_Interface_Memif{
			Id:     memifID,
			Master: isMaster,
		},
	}

	if memifParms != nil {
		if memifParms.Mode != "" {
			iface.Memif.Mode = memifMode(memifParms.Mode)
		}
		if memifParms.MemifDirectory != "" {
			defaultMemifDirectory = memifParms.MemifDirectory
		}
	}

	if defaultMemifDirectory == "" {
		defaultMemifDirectory = controller.MemifDirectoryName
	}

	iface.Memif.SocketFilename = defaultMemifDirectory + "/memif_" + masterVppAgent + ".sock"

	iface.RxModeSettings = rxModeControllerToInterface(rxMode)

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructMemInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

// ConstructVxlanInterface returns an KVType
func ConstructVxlanInterface(vppAgent string,
	ifname string,
	vni uint32,
	ep1IPAddress string,
	ep2IPAddress string) *KVType {

	ep1 := StripSlashAndSubnetIPAddress(ep1IPAddress)
	ep2 := StripSlashAndSubnetIPAddress(ep2IPAddress)

	iface := &interfaces.Interfaces_Interface{
		Name:    ifname,
		Type:    interfaces.InterfaceType_VXLAN_TUNNEL,
		Enabled: true,
		Vxlan: &interfaces.Interfaces_Interface_Vxlan{
			SrcAddress: ep1,
			DstAddress: ep2,
			Vni:        vni,
		},
	}

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ContructVxlanInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

//StripSlashAndSubnetIPAddress if the ip address has a /xx subnet attached, it is stripped off
func StripSlashAndSubnetIPAddress(ipAndSubnetStr string) string {
	strs := strings.Split(ipAndSubnetStr, "/")
	return strs[0]
}

// constructUniDirXConnect creates a unit directional entry returns an KVType
func constructUniDirXConnect(vppAgent, if1, if2 string) *KVType {

	xconn := &l2.XConnectPairs_XConnectPair{
		ReceiveInterface:  if1,
		TransmitInterface: if2,
	}

	key := L2XConnectKey(vppAgent, if1)

	log.Debugf("ConstructXConnect: key='%s', xconn='%v", key, xconn)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeL2XC,
		XConn:        xconn,
	}

	return kv
}

// ConstructXConnect creates a bidir xconn returns an []*KVType
func ConstructXConnect(vppAgent, if1, if2 string) []*KVType {

	kvVPP1 := constructUniDirXConnect(vppAgent, if1, if2)
	kvVPP2 := constructUniDirXConnect(vppAgent, if2, if1)
	return []*KVType{kvVPP1, kvVPP2}
}

// ConstructTapInterface returns an KVType
func ConstructTapInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	rxMode string,
	hostIfName string) *KVType {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_TAP_INTERFACE,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		Tap: &interfaces.Interfaces_Interface_Tap{
			HostIfName: hostIfName,
		},
	}

	iface.RxModeSettings = rxModeControllerToInterface(rxMode)

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructTapInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

// ConstructAFPacketInterface returns an KVType
func ConstructAFPacketInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	rxMode string,
	hostIfName string) *KVType {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_AF_PACKET_INTERFACE,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		Afpacket: &interfaces.Interfaces_Interface_Afpacket{
			HostIfName: hostIfName,
		},
	}

	iface.RxModeSettings = rxModeControllerToInterface(rxMode)

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructAFPacketInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

// ConstructVEthInterface returns an KVType
func ConstructVEthInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	hostIfName string,
	peerIfName string,
	vnfName string) *KVType {

	iface := &linuxIntf.LinuxInterfaces_Interface{
		Name:        ifname,
		Type:        linuxIntf.LinuxInterfaces_VETH,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		HostIfName:  hostIfName,
		Namespace: &linuxIntf.LinuxInterfaces_Interface_Namespace{
			Type:         linuxIntf.LinuxInterfaces_Interface_Namespace_MICROSERVICE_REF_NS,
			Microservice: vnfName,
		},
		Veth: &linuxIntf.LinuxInterfaces_Interface_Veth{
			PeerIfName: peerIfName,
		},
	}

	key := LinuxInterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructVEthInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeLinuxInterface,
		LinuxIFace:   iface,
	}
	return kv
}

// ConstructStaticRoute returns an KVType
func ConstructStaticRoute(vppAgent string, l3sr *controller.L3VRFRoute) *KVType {

	sr := &l3.StaticRoutes_Route{
		VrfId:             l3sr.VrfId,
		Description:       l3sr.Description,
		DstIpAddr:         l3sr.DstIpAddr,
		NextHopAddr:       StripSlashAndSubnetIPAddress(l3sr.NextHopAddr),
		Weight:            l3sr.Weight,
		OutgoingInterface: l3sr.OutgoingInterface,
		Preference:        l3sr.Preference,
	}

	destIPAddr, _, _ := addrs.ParseIPWithPrefix(sr.DstIpAddr)
	key := L3RouteKey(vppAgent, sr.VrfId, destIPAddr, sr.NextHopAddr)

	log.Debugf("ConstructStaticRoute: key='%s', sr='%v", key, sr)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeL3Route,
		L3Route:      sr,
	}
	return kv
}

func ConstructStaticArpEntry(vppAgent string, l3ae *controller.L3ArpEntry) *KVType {

	ae := &l3.ArpTable_ArpEntry{
		Interface:   l3ae.OutgoingInterface,
		Static:      true,
		IpAddress:   l3ae.IpAddress,
		PhysAddress: l3ae.PhysAddress,
	}

	key := ArpEntryKey(vppAgent, ae.Interface, ae.PhysAddress)

	log.Debugf("ConstructStaticArpEntry: key='%s', arp='%v", key, ae)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeArp,
		ArpEntry:      ae,
	}
	return kv
}

// func (vppapi *VppAgentAPIType) createL2FibEntry(etcdPrefix string, bdName string, destMacAddr string,
// 	outGoingIf string) (*l2.FibTableEntries_FibTableEntry, error) {

// 	l2fib := &l2.FibTableEntries_FibTableEntry{
// 		BridgeDomain:      bdName,
// 		PhysAddress:       destMacAddr,
// 		Action:            l2.FibTableEntries_FibTableEntry_FORWARD,
// 		OutgoingInterface: outGoingIf,
// 		StaticConfig:      true,
// 	}

// 	//if vppapi.reconcileInProgress {
// 	//	vppapi.reconcileL2FibEntry(etcdPrefix, l2fib)
// 	//} else {

// 	log.Println(l2fib)

// 	rc := NewRemoteClientTxn(etcdPrefix, vppapi.dbFactory)
// 	err := rc.Put().BDFIB(l2fib).Send().ReceiveReply()

// 	if err != nil {
// 		log.Error("createL2Fib: databroker.Store: ", err)
// 		return nil, err

// 	}
// 	//}

// 	return l2fib, nil
// }
