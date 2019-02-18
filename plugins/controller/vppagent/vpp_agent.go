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
	linuxIntf "github.com/ligato/vpp-agent/api/models/linux/interfaces"
	namespace "github.com/ligato/vpp-agent/api/models/linux/namespace"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	"strconv"
)

var (
	log *logrus.Logger
)

func VppAgentSetLogger(l *logrus.Logger) {
	log = l
}

func rxModeControllerToInterface(controllerRxMode string) *interfaces.Interface_RxModeSettings {

	rxSettings := &interfaces.Interface_RxModeSettings{}
	switch controllerRxMode {
	case controller.RxModePolling:
		rxSettings.RxMode = interfaces.Interface_RxModeSettings_POLLING
		return rxSettings
	case controller.RxModeInterrupt:
		rxSettings.RxMode = interfaces.Interface_RxModeSettings_INTERRUPT
		return rxSettings
	case controller.RxModeAdaptive:
		rxSettings.RxMode = interfaces.Interface_RxModeSettings_ADAPTIVE
		return rxSettings
	}
	return nil
}

// ConstructL2BD returns an KVType
func ConstructL2BD(vppAgent string,
	bdName string,
	ifaces []*l2.BridgeDomain_Interface,
	bdParms *controller.BDParms) *KVType {

	// keep the ifaces sorted so subsequent comparing during transaction processing
	// is easier as it does a String() then compares strings and an unsorted array
	// banjaxes the compare if two bridges are equal but have a same set of iFaces but
	// are in a different order
	var sortedInterfaces []*l2.BridgeDomain_Interface
	for _, iface := range ifaces {
		sortedInterfaces = insertSortBridgedInterface(sortedInterfaces, iface)
	}

	l2bd := &l2.BridgeDomain{
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
	l2bd *l2.BridgeDomain,
	ifaces []*l2.BridgeDomain_Interface) *KVType {

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
	data []*l2.BridgeDomain_Interface,
	el *l2.BridgeDomain_Interface) []*l2.BridgeDomain_Interface {
	index := sort.Search(len(data), func(i int) bool { return data[i].Name > el.Name })
	data = append(data, &l2.BridgeDomain_Interface{})
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

	iface := &interfaces.Interface{
		Name:        ifname,
		Type:        interfaces.Interface_DPDK,
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

	iface := &interfaces.Interface{
		Name:        ifname,
		Type:        interfaces.Interface_SOFTWARE_LOOPBACK,
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

func memifMode(modeString string) interfaces.MemifLink_MemifMode {

	mode := interfaces.MemifLink_ETHERNET
	if modeString != "" {
		switch modeString {
		case controller.IfMemifModeEhernet:
			mode = interfaces.MemifLink_ETHERNET
		case controller.IfMemifModeIP:
			mode = interfaces.MemifLink_IP
		case controller.IfMemifModePuntInject:
			mode = interfaces.MemifLink_PUNT_INJECT
		}
	}
	return mode
}

func strToUInt32(v string) uint32 { // assume str is a uint
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return uint32(i)
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

	ifaceMemif := &interfaces.Interface_Memif{
		Memif: &interfaces.MemifLink{
			Id: memifID,
			Master: isMaster,
		},
	}
	iface := &interfaces.Interface{
		Name:        ifname,
		Type:        interfaces.Interface_MEMIF,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		Link: ifaceMemif,
	}

	if memifParms != nil {
		if memifParms.Mode != "" {
			ifaceMemif.Memif.Mode = memifMode(memifParms.Mode)
		}
		if memifParms.MemifDirectory != "" {
			defaultMemifDirectory = memifParms.MemifDirectory
		}
		if memifParms.RingSize != "" {
			ifaceMemif.Memif.RingSize = strToUInt32(memifParms.RingSize)
		}
		if memifParms.BufferSize != "" {
			ifaceMemif.Memif.BufferSize = strToUInt32(memifParms.BufferSize)
		}
		if memifParms.RxQueues != "" {
			ifaceMemif.Memif.RxQueues = strToUInt32(memifParms.RxQueues)
		}
		if memifParms.TxQueues != "" {
			ifaceMemif.Memif.TxQueues = strToUInt32(memifParms.TxQueues)
		}
		ifaceMemif.Memif.Secret = memifParms.Secret
	}

	if defaultMemifDirectory == "" {
		defaultMemifDirectory = controller.MemifDirectoryName
	}

	ifaceMemif.Memif.SocketFilename = defaultMemifDirectory + "/memif_" + masterVppAgent + ".sock"

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

	iface := &interfaces.Interface{
		Name:    ifname,
		Type:    interfaces.Interface_VXLAN_TUNNEL,
		Enabled: true,
		Link: &interfaces.Interface_Vxlan{
			Vxlan: &interfaces.VxlanLink{
				SrcAddress: ep1,
				DstAddress: ep2,
				Vni:        vni,
			},
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

	xconn := &l2.XConnectPair{
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
	tapParms *controller.Interface_TapParms,
	hostPortLabel string) *KVType {

	ifaceTap := &interfaces.Interface_Tap{
		Tap: &interfaces.TapLink{
			//HostIfName: hostPortLabel,
			Version:    2,
		},
	}
	iface := &interfaces.Interface{
		Name:        ifname,
		Type:        interfaces.Interface_TAP,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		Link: 		 ifaceTap,
	}

	iface.RxModeSettings = rxModeControllerToInterface(rxMode)

	if tapParms != nil {
		//ifaceTap.Tap.ToMicroservice = tapParms.Namespace
		if tapParms.RxRingSize != "" {
			ifaceTap.Tap.RxRingSize = strToUInt32(tapParms.RxRingSize)
		}
		if tapParms.TxRingSize != "" {
			ifaceTap.Tap.TxRingSize = strToUInt32(tapParms.TxRingSize)
		}
	}

	key := InterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructTapInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeInterface,
		IFace:        iface,
	}
	return kv
}

// ConstructLinuxTapInterface returns an KVType
func ConstructLinuxTapInterface(vppAgent string,
	ifname string,
	ipAddresses []string,
	macAddr string,
	mtu uint32,
	adminStatus string,
	hostIfName string,
	vppSideTapName string,
	hostNameSpace string,
	microsServiceLabel string) *KVType {

	linTapIf := &linuxIntf.Interface_Tap{
		Tap: &linuxIntf.TapLink{
			VppTapIfName: vppSideTapName,
		},
	}

	iface := &linuxIntf.Interface{
		Name:        ifname,
		Type:        linuxIntf.Interface_TAP_TO_VPP,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		//HostIfName: hostIfName,
		Link: linTapIf,
	}

	ns := &namespace.NetNamespace{}
	if hostNameSpace == "" {
		ns.Type = namespace.NetNamespace_MICROSERVICE
		ns.Reference = microsServiceLabel
	} else {
		ns.Type = namespace.NetNamespace_NSID
		ns.Reference = hostNameSpace
	}
	iface.Namespace = ns

	key := LinuxInterfaceKey(vppAgent, iface.Name)

	log.Debugf("ConstructLinuxTapInterface: key='%s', iface='%v", key, iface)

	kv := &KVType{
		VppKey:       key,
		VppEntryType: VppEntryTypeLinuxInterface,
		LinuxIFace:   iface,
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

	ifaceAFP := &interfaces.Interface_Afpacket{
		Afpacket: &interfaces.AfpacketLink{
			HostIfName: hostIfName,
		},
	}

	iface := &interfaces.Interface{
		Name:        ifname,
		Type:        interfaces.Interface_AF_PACKET,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		Link:        ifaceAFP,

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
	linuxNamespaceType string,
	linuxNamespaceValue string,
	vnfName string,
	tcpOffloadDisabled bool) *KVType {

	ns := &namespace.NetNamespace{}

	ifaceVETH := &linuxIntf.Interface_Veth{
		Veth: &linuxIntf.VethLink{
			PeerIfName: peerIfName,
		},
	}

	if tcpOffloadDisabled {
		ifaceVETH.Veth.RxChecksumOffloading = linuxIntf.VethLink_CHKSM_OFFLOAD_DISABLED
		ifaceVETH.Veth.TxChecksumOffloading = linuxIntf.VethLink_CHKSM_OFFLOAD_DISABLED
	}

	iface := &linuxIntf.Interface{
		Name:        ifname,
		Type:        linuxIntf.Interface_VETH,
		Enabled:     adminStatusStringToBool(adminStatus),
		PhysAddress: macAddr,
		IpAddresses: sortedIPAddresses(ipAddresses),
		Mtu:         mtu,
		HostIfName:  hostIfName,
		Link:        ifaceVETH,
	}

	if linuxNamespaceType == "" {
		ns.Type = namespace.NetNamespace_MICROSERVICE
		ns.Reference = vnfName
	} else {
		switch linuxNamespaceType {
		case controller.LinuxNamespaceMICROSERVICE:
			ns.Type = namespace.NetNamespace_MICROSERVICE
			ns.Reference = linuxNamespaceValue
		case controller.LinuxNamespaceNAME:
			ns.Type = namespace.NetNamespace_NSID
			ns.Reference = linuxNamespaceValue
		case controller.LinuxNamespacePID:
			ns.Type = namespace.NetNamespace_PID
			ns.Reference = linuxNamespaceValue
		case controller.LinuxNamespaceFILE:
			ns.Type = namespace.NetNamespace_FD
			ns.Reference = linuxNamespaceValue
		}
	}
	iface.Namespace = ns

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

	sr := &l3.Route{
		VrfId:             l3sr.VrfId,
		DstNetwork:        l3sr.DstIpAddr,
		//DstIpAddr:         l3sr.DstIpAddr,
		NextHopAddr:       StripSlashAndSubnetIPAddress(l3sr.NextHopAddr),
		Weight:            l3sr.Weight,
		OutgoingInterface: l3sr.OutgoingInterface,
		Preference:        l3sr.Preference,
	}

	//destIPAddr, _, _ := addrs.ParseIPWithPrefix(sr.DstIpAddr)
	destIPAddr, _, _ := addrs.ParseIPWithPrefix(sr.DstNetwork)
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

	ae := &l3.ARPEntry{
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
