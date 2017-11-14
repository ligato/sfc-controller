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

// The L2 VxLan Tunnel CNP driver takes care of wiring the inter/intra
// hosts and external entity connections. The "internal" resource management is
// taken care of by this module.  The state of the wiring is stored in the ETCD
// just as the config of the system is.  For example, there are 2 EEs, and 3 HEs.
// This means we need to wire vxLans between each HE to all EEs, and all other
// HEs.  Then we have to do this in the reverse direction. There is, in effect,
// a star vxlan from each EE to each HE, and HE has a vxLan back to its EE.
// Also, there is a bidirectional mesh of VxLans between each HE to every other
// HE.  This involves allocating indicies for the VxLans ... these will be
// allocated and tracked by this module and stored in the ETCD datastore so
// upon sfc ctlr restart, we dont lose track of any "tracked" resource.

//go:generate protoc --proto_path=model --gogo_out=model model/l2.proto

package l2driver

import (
	"errors"
	"fmt"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/ligato/cn-infra/servicelabel"
	l2driver "github.com/ligato/sfc-controller/controller/cnpdriver/l2driver/model"
	"github.com/ligato/sfc-controller/controller/extentitydriver"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/ligato/sfc-controller/controller/utils"
	"github.com/ligato/vpp-agent/clientv1/linux"
	"github.com/ligato/vpp-agent/clientv1/linux/remoteclient"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l3plugin/model/l3"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linuxplugin/ifplugin/model/interfaces"
	"strconv"
	"strings"
)

var (
	log = logroot.StandardLogger()
)

type sfcCtlrL2CNPDriver struct {
	dbFactory           func(string) keyval.ProtoBroker
	db                  keyval.ProtoBroker
	name                string
	l2CNPEntityCache    l2CNPEntityCacheType
	l2CNPStateCache     l2CNPStateCacheType
	reconcileBefore     reconcileCacheType
	reconcileAfter      reconcileCacheType
	reconcileInProgress bool
	seq                 sequencer
}

// sequencer groups all sequences used by L2 driver.
// (instead of using global variables that caused
// problems while running automated tests)
type sequencer struct {
	VLanID        uint32
	MemIfID       uint32
	MacInstanceID uint32
	IPInstanceID  uint32
}

type heToEEStateType struct {
	vlanIf  *interfaces.Interfaces_Interface
	bd      *l2.BridgeDomains_BridgeDomain
	l3Route *l3.StaticRoutes_Route
}

type heStateType struct {
	bd *l2.BridgeDomains_BridgeDomain
}

type l2CNPStateCacheType struct {
	HEToEEs map[string]map[string]*heToEEStateType
	HE      map[string]*heStateType
}

type l2CNPEntityCacheType struct {
	EEs      map[string]controller.ExternalEntity
	HEs      map[string]controller.HostEntity
	SFCs     map[string]controller.SfcEntity
	SysParms controller.SystemParameters
}

// NewRemoteClientTxn new vpp-agent remote client instance on top of key-val DB (ETCD)
// <microserviceLabel> that identifies a specific vpp-agent that needs to be configured
// <dbFactory> returns new instance of DataBroker for accessing key-val DB (ETCD)
func NewRemoteClientTxn(microserviceLabel string, dbFactory func(string) keyval.ProtoBroker) linux.DataChangeDSL {
	prefix := servicelabel.GetDifferentAgentPrefix(microserviceLabel)
	broker := dbFactory(prefix)
	return remoteclient.DataChangeRequestDB(broker)
}

// NewSfcCtlrL2CNPDriver creates new driver/mode for Native SFC Controller L2 Container Networking Policy
// <name> of the driver/plugin
// <dbFactory> returns new instance of DataBroker for accessing key-val DB (ETCD)
func NewSfcCtlrL2CNPDriver(name string, dbFactory func(string) keyval.ProtoBroker) *sfcCtlrL2CNPDriver {

	cnpd := &sfcCtlrL2CNPDriver{}
	cnpd.name = "Sfc Controller L2 Plugin: " + name
	cnpd.dbFactory = dbFactory
	cnpd.db = dbFactory(keyval.Root)

	cnpd.initL2CNPCache()
	cnpd.initReconcileCache()

	return cnpd
}

func (cnpd *sfcCtlrL2CNPDriver) initL2CNPCache() {
	cnpd.l2CNPStateCache.HEToEEs = make(map[string]map[string]*heToEEStateType)
	cnpd.l2CNPStateCache.HE = make(map[string]*heStateType)

	cnpd.l2CNPEntityCache.EEs = make(map[string]controller.ExternalEntity)
	cnpd.l2CNPEntityCache.HEs = make(map[string]controller.HostEntity)
	cnpd.l2CNPEntityCache.SFCs = make(map[string]controller.SfcEntity)

}

// Perform plugin specific initializations
func (cnpd *sfcCtlrL2CNPDriver) InitPlugin() error {
	return nil
}

// Cleanup anything as plugin is being de-reged
func (cnpd *sfcCtlrL2CNPDriver) DeinitPlugin() error {
	return nil
}

// Return user friendly name for this plugin
func (cnpd *sfcCtlrL2CNPDriver) GetName() string {
	return cnpd.name
}

// SetSystemParameters caches the current settings for the system
func (cnpd *sfcCtlrL2CNPDriver) SetSystemParameters(sp *controller.SystemParameters) error {
	cnpd.l2CNPEntityCache.SysParms = *sp
	if cnpd.seq.VLanID != 0 { // only init if this is the first time being set
		cnpd.seq.VLanID = cnpd.l2CNPEntityCache.SysParms.StartingVlanId - 1
	}

	return nil
}

// Perform CNP specific wiring for "connecting" a source host to a dest host
func (cnpd *sfcCtlrL2CNPDriver) WireHostEntityToDestinationHostEntity(sh *controller.HostEntity,
	dh *controller.HostEntity) error {

	// might have to create a vxlan tunnel i/f and assoc it to the e/w bridge on each of the hosts

	return nil
}

// Perform CNP specific wiring for "connecting" an external router to a host server, called from
// WireHostEntityToExternalEntity after host is wired to ee
func (cnpd *sfcCtlrL2CNPDriver) wireExternalEntityToHostEntity(ee *controller.ExternalEntity,
	he *controller.HostEntity) error {

	log.Infof("wireExternalEntityToHostEntity: he", he)
	log.Infof("wireExternalEntityToHostEntity: ee", ee)

	// this holds the relationship from the HE to the map of EEs to which this HE is wired
	heToEEMap, exists := cnpd.l2CNPStateCache.HEToEEs[he.Name]
	if !exists {
		return nil
	}

	// now ensure this HE has not yet been wired to the EE, if it has then wire the EE to the HE
	heToEEState, exists := heToEEMap[ee.Name]
	if !exists {
		return nil
	}

	//bdName := "BD_E2H_" + ee.Name + "_" + he.Name

	// create the vxlan i'f before the BD
	//ifName := "IF_VXLAN_E2H_" + ee.Name + "_" + he.Name
	tmpVlanid := heToEEState.vlanIf.Vxlan.Vni // use the same id as the reverse direction
	//vlanIf, err := cnpd.vxLanCreate(ee.Name, ifName, vlanid, ee.LoopbackIpv4, he.LoopbackIpv4)
	//if err != nil {
	//	log.Error("wireExternalEntityToHostEntity: error creating vxlan: '%s'", ifName)
	//	return err
	//}
	//
	//ifs := make([]*l2.BridgeDomains_BridgeDomain_Interfaces, 1)
	//ifEntry := l2.BridgeDomains_BridgeDomain_Interfaces{
	//	Name: ifName,
	//}
	//ifs[0] = &ifEntry
	//
	//// now create the bridge
	//bd, err := cnpd.bridgedDomainCreateWithIfs(ee.Name, bdName, ifs)
	//if err != nil {
	//	log.Error("wireExternalEntityToHostEntity: error creating BD: '%s'", bd.Name)
	//	return err
	//}

	// configure static route from this external router to the host
	description := "IF_STATIC_ROUTE_E2H_" + he.Name
	sr, err := cnpd.createStaticRoute(ee.Name, description, he.LoopbackIpv4, he.EthIpv4, ee.HostInterface.IfName)
	if err != nil {
		log.Error("wireExternalEntityToHostEntity: error creating static route i/f: '%s'", description)
		return err
	}

	log.Infof("wireExternalEntityToHostEntity: ee: %s, he: %s, vlanid: %d, static route: %s",
		ee.Name, he.Name, tmpVlanid, sr.String())

	// call the external entity api to queue a msg so that the external router config will be sent to the router
	// this will be replace perhaps by a watcher in the ext-ent driver
	extentitydriver.SfcCtlrL2WireExternalEntityToHostEntity(*ee, *he, tmpVlanid, sr)
	return nil
}

// Perform CNP specific wiring for "connecting" a host server to an external router
func (cnpd *sfcCtlrL2CNPDriver) WireHostEntityToExternalEntity(he *controller.HostEntity,
	ee *controller.ExternalEntity) error {

	cnpd.l2CNPEntityCache.HEs[he.Name] = *he
	cnpd.l2CNPEntityCache.EEs[ee.Name] = *ee

	log.Infof("WireHostEntityToExternalEntity: he", he)
	log.Infof("WireHostEntityToExternalEntity: ee", ee)

	if ee.HostInterface == nil || ee.HostVxlan == nil {
		log.Error("WireHostEntityToExternalEntity: invalid external entity config")
		return errors.New("invalid external entity config")
	}

	// this holds the relationship from the HE to the map of EEs to which this HE is wired
	heToEEMap, exists := cnpd.l2CNPStateCache.HEToEEs[he.Name]
	if !exists {
		heToEEMap = make(map[string]*heToEEStateType)
		cnpd.l2CNPStateCache.HEToEEs[he.Name] = heToEEMap
	}

	// now ensure this HE has not yet been wired to the EE
	heToEEState, exists := heToEEMap[ee.Name]
	if exists {
		// maybe look at contents to see if they are programmed properly but for now just return
		return nil
	}

	bdName := "BD_H2E_" + he.Name + "_" + ee.Name

	// create the vxlan i'f before the BD
	ifName := "IF_VXLAN_H2E_" + he.Name + "_" + ee.Name

	var vlanID uint32

	he2eeID, err := cnpd.DatastoreHE2EEIDsRetrieve(he.Name, ee.Name)
	if he2eeID == nil || he2eeID.VlanId == 0 {
		cnpd.seq.VLanID++
		vlanID = cnpd.seq.VLanID
	} else {
		vlanID = he2eeID.VlanId
	}
	vlanIf, err := cnpd.vxLanCreate(he.Name, ifName, vlanID, he.LoopbackIpv4, ee.HostVxlan.SourceIpv4)
	if err != nil {
		log.Error("WireHostEntityToExternalEntity: error creating vxlan: '%s'", ifName)
		return err
	}

	ifs := make([]*l2.BridgeDomains_BridgeDomain_Interfaces, 1)
	ifEntry := l2.BridgeDomains_BridgeDomain_Interfaces{
		Name: ifName,
	}
	ifs[0] = &ifEntry

	// now create the bridge
	bd, err := cnpd.bridgedDomainCreateWithIfs(he.Name, bdName, ifs)
	if err != nil {
		log.Error("WireHostEntityToExternalEntity: error creating BD: '%s'", bd.Name)
		return err
	}

	// configure static route from this host to the external router
	description := "IF_STATIC_ROUTE_H2E_" + ee.Name
	sr, err := cnpd.createStaticRoute(he.Name, description, ee.HostVxlan.SourceIpv4, ee.HostInterface.Ipv4Addr,
		he.EthIfName)
	if err != nil {
		log.Error("WireHostEntityToExternalEntity: error creating static route i/f: '%s'", description)
		return err
	}

	heToEEState = &heToEEStateType{
		vlanIf:  vlanIf,
		bd:      bd,
		l3Route: sr,
	}

	// now link the he to the ee
	heToEEMap[ee.Name] = heToEEState

	log.Infof("WireHostEntityToExternalEntity: he: %s, ee: %s, vlanIf: %s, bd: %s, static route: %s",
		he.Name, ee.Name, vlanIf.Name, bd.Name, sr.String())

	cnpd.wireExternalEntityToHostEntity(ee, he)

	key, he2eeID, err := cnpd.DatastoreHE2EEIDsCreate(he.Name, ee.Name, vlanID)
	if err == nil && cnpd.reconcileInProgress {
		cnpd.reconcileAfter.he2eeIDs[key] = *he2eeID
	}

	return err
}

// Perform CNP specific wiring for "preparing" a host server example: create an east-west bridge
func (cnpd *sfcCtlrL2CNPDriver) WireInternalsForHostEntity(he *controller.HostEntity) error {

	cnpd.l2CNPEntityCache.HEs[he.Name] = *he

	log.Infof("WireInternalsForHostEntity: caching host: ", he)

	// this holds the state for an HE
	heState, exists := cnpd.l2CNPStateCache.HE[he.Name]
	if exists {
		return nil // if it is being updated .... figure out what that means here
	}
	heState = &heStateType{}
	cnpd.l2CNPStateCache.HE[he.Name] = heState

	mtu := cnpd.getMtu(he.Mtu)

	// configure the nic/ethernet
	if he.EthIfName != "" {
		if err := cnpd.createEthernet(he.Name, he.EthIfName, he.EthIpv4, mtu); err != nil {
			log.Error("WireInternalsForHostEntity: error creating ethernet i/f: '%s'", he.EthIfName)
			return err
		}
	}

	var heID *l2driver.HEIDs
	var loopbackMacAddrId uint32

	if he.LoopbackIpv4 != "" { // if configured, then create a loop back address

		var loopbackMacAddress string

		if he.LoopbackMacAddr == "" { // if not supplied, generate one
			heID, _ = cnpd.DatastoreHEIDsRetrieve(he.Name)
			if heID == nil || heID.LoopbackMacAddrId == 0 {
				cnpd.seq.MacInstanceID++
				loopbackMacAddress = formatMacAddress(cnpd.seq.MacInstanceID)
				loopbackMacAddrId = cnpd.seq.MacInstanceID
			} else {
				loopbackMacAddress = formatMacAddress(heID.LoopbackMacAddrId)
				loopbackMacAddrId = heID.LoopbackMacAddrId
			}
		} else {
			loopbackMacAddress = he.LoopbackMacAddr
		}

		mtu := cnpd.getMtu(he.Mtu)

		// configure loopback interface
		loopIfName := "IF_LOOPBACK_H_" + he.Name
		if err := cnpd.createLoopback(he.Name, loopIfName, loopbackMacAddress, he.LoopbackIpv4, mtu); err != nil {
			log.Error("WireInternalsForHostEntity: error creating loopback i/f: '%s'", loopIfName)
			return err
		}
	}

	bdName := "BD_INTERNAL_EW_" + he.Name
	bd, err := cnpd.bridgedDomainCreateWithIfs(he.Name, bdName, nil)
	if err != nil {
		log.Error("WireInternalsForHostEntity: error creating BD: '%s'", bd.Name)
		return err
	}

	heState.bd = bd

	key, heID, err := cnpd.DatastoreHEIDsCreate(he.Name, loopbackMacAddrId)
	if err == nil && cnpd.reconcileInProgress {
		cnpd.reconcileAfter.heIDs[key] = *heID
	}

	return err
}

// Perform CNP specific wiring for "preparing" an external entity
func (cnpd *sfcCtlrL2CNPDriver) WireInternalsForExternalEntity(ee *controller.ExternalEntity) error {

	extentitydriver.SfcCtlrL2WireExternalEntityInternals(*ee)

	return nil
}

// Perform CNP specific wiring for inter-container wiring, and container to external router wiring
func (cnpd *sfcCtlrL2CNPDriver) WireSfcEntity(sfc *controller.SfcEntity) error {

	var err error
	// the semantic difference between a north_south vs an east-west sfc entity, it what is the bridge that
	// the memIf/afPkt if's will be associated.
	switch sfc.Type {

	case controller.SfcType_SFC_NS_VXLAN:
		// north/south VXLAN type, memIfs/cntrs connect to vrouters/RASs bridge
		cnpd.l2CNPEntityCache.SFCs[sfc.Name] = *sfc
		err = cnpd.wireSfcNorthSouthVXLANElements(sfc)

	case controller.SfcType_SFC_NS_NIC_BD:
		fallthrough
	case controller.SfcType_SFC_NS_NIC_L2XCONN:
		// north/south NIC type, memIfs/cntrs connect to physical NIC
		cnpd.l2CNPEntityCache.SFCs[sfc.Name] = *sfc
		err = cnpd.wireSfcNorthSouthNICElements(sfc)

	case controller.SfcType_SFC_EW_MEMIF:
		fallthrough
	case controller.SfcType_SFC_EW_BD:
		fallthrough
	case controller.SfcType_SFC_EW_L2XCONN:
		// east/west type, memIfs/cntrs connect to the hosts easet/west bridge
		cnpd.l2CNPEntityCache.SFCs[sfc.Name] = *sfc
		err = cnpd.wireSfcEastWestElements(sfc)

	default:
		err = fmt.Errorf("WireSfcEntity: unknown entity type: '%s'", sfc.Type)
		log.Error(err.Error())
	}

	return err
}

// for now, ensure there is only one ee ... as each container will be wirred to it
func (cnpd *sfcCtlrL2CNPDriver) wireSfcNorthSouthVXLANElements(sfc *controller.SfcEntity) error {

	eeCount := 0
	eeName := ""

	// find the external entity and ensure there is only one allowed
	for i, sfcEntityElement := range sfc.GetElements() {

		log.Infof("wireSfcEastWestElements: sfc entity element[%d]: ", i, sfcEntityElement)

		switch sfcEntityElement.Type {
		case controller.SfcElementType_EXTERNAL_ENTITY:
			eeCount++
			if eeCount > 1 {
				err := fmt.Errorf("wireSfcNorthSouthVXLANElements: only one ee allowed for n/s sfc: '%s'",
					sfc.Name)
				log.Error(err.Error())
				return err
			}

			eeName = sfcEntityElement.Container
			if _, exists := cnpd.l2CNPEntityCache.EEs[sfcEntityElement.Container]; !exists {
				err := fmt.Errorf("wireSfcNorthSouthVXLANElements: ee not found: '%s' for n/s sfc: '%s'",
					eeName, sfc.Name)
				log.Error(err.Error())
				return err
			}
		}
	}

	if eeCount == 0 {
		err := fmt.Errorf("wireSfcNorthSouthVXLANElements: NO ee specified for n/s sfc: '%s'", sfc.Name)
		log.Error(err.Error())
		return err
	}

	// now wire each container to the bridge wired from the host to the ee
	for i, sfcEntityElement := range sfc.GetElements() {

		log.Infof("wireSfcNorthSouthVXLANElements: sfc entity element[%d]: ", i, sfcEntityElement)

		switch sfcEntityElement.Type {

		case controller.SfcElementType_VPP_CONTAINER_AFP:
			fallthrough
		case controller.SfcElementType_NON_VPP_CONTAINER_AFP:

			if _, exists := cnpd.l2CNPEntityCache.HEs[sfcEntityElement.EtcdVppSwitchKey]; !exists {
				err := fmt.Errorf("wireSfcNorthSouthVXLANElements: cannot find host '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}
			// the container has which host it is assoc'ed with, get the ee bridge
			heToEEState, exists := cnpd.l2CNPStateCache.HEToEEs[sfcEntityElement.EtcdVppSwitchKey][eeName]
			if !exists {
				err := fmt.Errorf("wireSfcNorthSouthVXLANElements: cannot find host/bridge: '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}
			if err := cnpd.createAFPacketVEthPairAndAddToBridge(sfc, heToEEState.bd, sfcEntityElement); err != nil {
				log.Error("wireSfcNorthSouthVXLANElements: error creating memIf pair: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}
			if err := cnpd.createCustomLabel(sfcEntityElement.Container, sfcEntityElement.GetCustomInfo()); err != nil {
				log.Error("wireSfcNorthSouthVXLANElements: error creating customLabel: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}

		case controller.SfcElementType_VPP_CONTAINER_MEMIF:
			fallthrough
		case controller.SfcElementType_NON_VPP_CONTAINER_MEMIF:

			if _, exists := cnpd.l2CNPEntityCache.HEs[sfcEntityElement.EtcdVppSwitchKey]; !exists {
				err := fmt.Errorf("wireSfcNorthSouthVXLANElements: cannot find host '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}
			// the container has which host it is assoc'ed with, get the ee bridge
			heToEEState, exists := cnpd.l2CNPStateCache.HEToEEs[sfcEntityElement.EtcdVppSwitchKey][eeName]
			if !exists {
				err := fmt.Errorf("wireSfcNorthSouthVXLANElements: cannot find host/bridge: '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}
			if err := cnpd.createMemIfPairAndAddToBridge(sfc, sfcEntityElement.EtcdVppSwitchKey, heToEEState.bd,
				sfcEntityElement, false); err != nil {
				log.Error("wireSfcNorthSouthVXLANElements: error creating memIf pair: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}
			if err := cnpd.createCustomLabel(sfcEntityElement.Container, sfcEntityElement.GetCustomInfo()); err != nil {
				log.Error("wireSfcNorthSouthVXLANElements: error creating customLabel: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}
		}
	}

	return nil
}

// north/south NIC type, memIfs/cntrs connect to physical NIC
func (cnpd *sfcCtlrL2CNPDriver) wireSfcNorthSouthNICElements(sfc *controller.SfcEntity) error {

	heCount := 0
	var he *controller.SfcEntity_SfcElement

	// find the host entity and ensure there is only one allowed
	for i, sfcEntityElement := range sfc.GetElements() {

		log.Infof("wireSfcNorthSouthNICElements: sfc entity element[%d]: ", i, sfcEntityElement)

		switch sfcEntityElement.Type {
		case controller.SfcElementType_HOST_ENTITY:
			heCount++
			if heCount > 1 {
				err := fmt.Errorf("wireSfcNorthSouthNICElements: only one he allowed for n/s sfc: '%s'", sfc.Name)
				log.Error(err.Error())
				return err
			}
			he = sfcEntityElement
		}
	}

	if heCount == 0 {
		err := fmt.Errorf("wireSfcNorthSouthNICElements: NO he specified for n/s sfc: '%s'", sfc.Name)
		log.Error(err.Error())
		return err
	}

	// now wire each container to the bridge on the he
	for i, sfcEntityElement := range sfc.GetElements() {

		log.Infof("wireSfcNorthSouthNICElements: sfc entity element[%d]: ", i, sfcEntityElement)

		switch sfcEntityElement.Type {

		case controller.SfcElementType_VPP_CONTAINER_AFP:
			fallthrough
		case controller.SfcElementType_NON_VPP_CONTAINER_AFP:

			if _, exists := cnpd.l2CNPEntityCache.HEs[sfcEntityElement.EtcdVppSwitchKey]; !exists {
				err := fmt.Errorf("wireSfcNorthSouthNICElements: cannot find host '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}

			mtu := cnpd.getMtu(he.Mtu)
			// physical NIC
			if err := cnpd.createEthernet(he.Container, he.PortLabel, "", mtu); err != nil {
				log.Error("wireSfcNorthSouthNICElements: error creating ethernet i/f: '%s'", he.PortLabel)
				return err
			}

			if sfc.Type == controller.SfcType_SFC_NS_NIC_BD {
				// bridge domain -based wiring
				ifEntry := &l2.BridgeDomains_BridgeDomain_Interfaces{
					Name: he.PortLabel,
				}
				bdName := "BD_INTERNAL_NS_" + sfcEntityElement.Container + "_" + sfcEntityElement.PortLabel
				bd, err := cnpd.bridgedDomainCreateWithIfs(sfcEntityElement.EtcdVppSwitchKey, bdName,
					[]*l2.BridgeDomains_BridgeDomain_Interfaces{ifEntry})
				if err != nil {
					log.Error("wireSfcNorthSouthNICElements: error creating BD: '%s'", bd.Name)
					return err
				}
				// veth pair
				if err := cnpd.createAFPacketVEthPairAndAddToBridge(sfc, bd, sfcEntityElement); err != nil {
					log.Error("wireSfcNorthSouthNICElements: error creating veth pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
			} else {
				// l2xconnect -based wiring
				afIfName, err := cnpd.createAFPacketVEthPair(sfc, sfcEntityElement)
				if err != nil {
					log.Error("wireSfcNorthSouthNICElements: error creating veth pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
				err = cnpd.createXConnectPair(sfcEntityElement.EtcdVppSwitchKey, he.PortLabel, afIfName)
				if err != nil {
					return err
				}
			}

			// custom label
			if err := cnpd.createCustomLabel(sfcEntityElement.Container, sfcEntityElement.GetCustomInfo()); err != nil {
				log.Error("wireSfcNorthSouthNICElements: error creating customLabel: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}

		case controller.SfcElementType_VPP_CONTAINER_MEMIF:
			fallthrough
		case controller.SfcElementType_NON_VPP_CONTAINER_MEMIF:

			if _, exists := cnpd.l2CNPEntityCache.HEs[sfcEntityElement.EtcdVppSwitchKey]; !exists {
				err := fmt.Errorf("wireSfcNorthSouthNICElements: cannot find host '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}

			mtu := cnpd.getMtu(he.Mtu)
			// physical NIC
			if err := cnpd.createEthernet(he.Container, he.PortLabel, "", mtu); err != nil {
				log.Error("wireSfcNorthSouthNICElements: error creating ethernet i/f: '%s'", he.PortLabel)
				return err
			}

			if sfc.Type == controller.SfcType_SFC_NS_NIC_BD {
				// bridge domain - based wiring
				ifEntry := &l2.BridgeDomains_BridgeDomain_Interfaces{
					Name: he.PortLabel,
				}
				bdName := "BD_INTERNAL_NS_" + sfcEntityElement.Container + "_" + sfcEntityElement.PortLabel
				bd, err := cnpd.bridgedDomainCreateWithIfs(sfcEntityElement.EtcdVppSwitchKey, bdName,
					[]*l2.BridgeDomains_BridgeDomain_Interfaces{ifEntry})
				if err != nil {
					log.Error("wireSfcNorthSouthNICElements: error creating BD: '%s'", bd.Name)
					return err
				}

				// memif
				if err := cnpd.createMemIfPairAndAddToBridge(sfc, sfcEntityElement.EtcdVppSwitchKey, bd,
					sfcEntityElement, false); err != nil {
					log.Error("wireSfcNorthSouthNICElements: error creating memIf pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
			} else {
				// l2xconnect-based wiring
				memIfName, err := cnpd.createMemIfPair(sfc, sfcEntityElement.EtcdVppSwitchKey, sfcEntityElement,
					false)
				if err != nil {
					log.Error("wireSfcNorthSouthNICElements: error creating memIf pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
				err = cnpd.createXConnectPair(sfcEntityElement.EtcdVppSwitchKey, he.PortLabel, memIfName)
				if err != nil {
					return err
				}
			}

			// custom label
			if err := cnpd.createCustomLabel(sfcEntityElement.Container, sfcEntityElement.GetCustomInfo()); err != nil {
				log.Error("wireSfcNorthSouthNICElements: error creating customLabel: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}
		}
	}

	return nil
}

// This is a group of containers that need to be wired to the e/w bridge.  Each container has a host that it is
// supposed to be wired to.  When we have k8s, each container in the sfc-entity will have to be resolved as to which
// host it has been deployed on.  Questions: ip addressing, mac addresses for memIf's.  Am I using one space for
// the system .... ie each memIf in each container will have a unique ip address in the 10.*.*.* space, and a unique
// macAddress in the 02:*:*:*:*:* space.  Also, is east-west bridge connected via vxLan's?
func (cnpd *sfcCtlrL2CNPDriver) wireSfcEastWestElements(sfc *controller.SfcEntity) error {

	prevMemIfName := ""

	if sfc.Type == controller.SfcType_SFC_EW_MEMIF {
		if len(sfc.GetElements())%2 != 0 {
			err := fmt.Errorf("wireSfcEastWestElements: e-w memif sfc should have pairs of entries: '%s'", sfc.Name)
			log.Error(err.Error())
			return err
		}
	}

	for i, sfcEntityElement := range sfc.GetElements() {

		log.Infof("wireSfcEastWestElements: sfc entity element[%d]: ", i, sfcEntityElement)

		switch sfcEntityElement.Type {

		case controller.SfcElementType_EXTERNAL_ENTITY:
			err := fmt.Errorf("wireSfcEastWestElements: external entity not allowed in e-w sfc: '%s'", sfc.Name)
			log.Error(err.Error())
			return err

		case controller.SfcElementType_VPP_CONTAINER_AFP:
			fallthrough
		case controller.SfcElementType_NON_VPP_CONTAINER_AFP:
			// the container has which host it is assoc'ed with, get the host e/w bridge
			heState, exists := cnpd.l2CNPStateCache.HE[sfcEntityElement.EtcdVppSwitchKey]
			if !exists {
				err := fmt.Errorf("wireSfcEastWestElements: cannot find host/bridge: '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}
			if sfc.Type == controller.SfcType_SFC_EW_BD {
				// bridge domain -based wiring
				if err := cnpd.createAFPacketVEthPairAndAddToBridge(sfc, heState.bd, sfcEntityElement); err != nil {
					log.Error("wireSfcEastWestElements: error creating memIf pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
			} else {
				// l2xconnect -based wiring
				afIfName, err := cnpd.createAFPacketVEthPair(sfc, sfcEntityElement)
				if err != nil {
					log.Error("wireSfcEastWestElements: error creating veth pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
				if prevMemIfName != "" {
					err = cnpd.createXConnectPair(sfcEntityElement.EtcdVppSwitchKey, afIfName, prevMemIfName)
					prevMemIfName = ""
					if err != nil {
						return err
					}
				} else {
					prevMemIfName = afIfName
				}
			}
			if err := cnpd.createCustomLabel(sfcEntityElement.Container, sfcEntityElement.GetCustomInfo()); err != nil {
				log.Error("wireSfcNorthSouthVXLANElements: error creating customLabel: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}

		case controller.SfcElementType_VPP_CONTAINER_MEMIF:
			fallthrough
		case controller.SfcElementType_NON_VPP_CONTAINER_MEMIF:

			// the container has which host it is assoc'ed with, get the host e/w bridge
			heState, exists := cnpd.l2CNPStateCache.HE[sfcEntityElement.EtcdVppSwitchKey]
			if !exists {
				err := fmt.Errorf("wireSfcEastWestElements: cannot find host/bridge: '%s' for this sfc: '%s'",
					sfcEntityElement.EtcdVppSwitchKey, sfc.Name)
				return err
			}

			if sfc.Type == controller.SfcType_SFC_EW_MEMIF {
				if i%2 == 0 {
					// need to create an inter-container memif, use the left of the pair to create the pair
					if err := cnpd.createOneOrMoreInterContainerMemIfPairs(sfc.Name, sfc.Elements[i], sfc.Elements[i+1],
						sfc.VnfRepeatCount); err != nil {
						log.Error("wireSfcEastWestElements: error creating memIf pair: sfc: '%s', Container: '%s', i='%d'",
							sfc.Name, sfcEntityElement.Container, i)
						return err
					}
				}
			} else if sfc.Type == controller.SfcType_SFC_EW_BD {
				// bridge domain -based wiring
				if err := cnpd.createMemIfPairAndAddToBridge(sfc, sfcEntityElement.EtcdVppSwitchKey, heState.bd,
					sfcEntityElement, true); err != nil {
					log.Error("wireSfcEastWestElements: error creating memIf pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
			} else {
				// l2xconnect -based wiring
				memIfName, err := cnpd.createMemIfPair(sfc, sfcEntityElement.EtcdVppSwitchKey, sfcEntityElement,
					false)
				if err != nil {
					log.Error("wireSfcEastWestElements: error creating memIf pair: sfc: '%s', Container: '%s'",
						sfc.Name, sfcEntityElement.Container)
					return err
				}
				if prevMemIfName != "" {
					err = cnpd.createXConnectPair(sfcEntityElement.EtcdVppSwitchKey, memIfName, prevMemIfName)
					prevMemIfName = ""
					if err != nil {
						return err
					}
				} else {
					prevMemIfName = memIfName
				}
			}
			if err := cnpd.createCustomLabel(sfcEntityElement.Container, sfcEntityElement.GetCustomInfo()); err != nil {
				log.Error("wireSfcNorthSouthVXLANElements: error creating customLabel: sfc: '%s', Container: '%s'",
					sfc.Name, sfcEntityElement.Container)
				return err
			}
		}
	}

	return nil
}

// createOneOrMoreInterContainerMemIfPairs creates memif pair and returns vswitch-end memif interface name
func (cnpd *sfcCtlrL2CNPDriver) createOneOrMoreInterContainerMemIfPairs(
	sfcName string,
	vnfElement1 *controller.SfcEntity_SfcElement,
	vnfElement2 *controller.SfcEntity_SfcElement,
	vnfRepeatCount uint32) error {

	log.Infof("createInterContainerMemIfPair: sfc: '%s', vnf1: '%s', vnf2: '%s', repeatCount: '%d'",
		sfcName, vnfElement1.Container, vnfElement2.Container, vnfRepeatCount)

	vnf1Port := ""
	vnf2Port := ""
	mtu := cnpd.getMtu(vnfElement1.Mtu)
	container1Name := ""
	container2Name := ""

	for repeatCount := uint32(0); repeatCount <= vnfRepeatCount; repeatCount++ {

		if repeatCount == 0 {
			container1Name = vnfElement1.Container
			vnf1Port = vnfElement1.PortLabel
		} else {
			container1Name = "vnfx-" + strconv.Itoa(int(repeatCount-1))
			vnf1Port = vnfElement1.PortLabel
		}
		if repeatCount == vnfRepeatCount {
			container2Name = vnfElement2.Container
			vnf2Port = vnfElement2.PortLabel
		} else {
			container2Name = "vnfx-" + strconv.Itoa(int(repeatCount))
			vnf2Port = vnfElement2.PortLabel
		}

		var memifID uint32

		sfcID, _ := cnpd.DatastoreSFCIDsRetrieve(sfcName, container1Name, vnf1Port)
		if sfcID == nil || sfcID.MemifId == 0 {
			cnpd.seq.MemIfID++
			memifID = cnpd.seq.MemIfID
		} else {
			memifID = sfcID.MemifId
		}

		// create a memif in the vnf container
		if err := cnpd.createInterContainerMemIfPair(
			sfcName,
			container1Name, vnf1Port,
			container2Name, vnf2Port,
			mtu,
			memifID); err != nil {
			return err
		}

		key, sfcID, err := cnpd.DatastoreSFCIDsCreate(sfcName, container1Name, vnf1Port, 0, 0, memifID)
		if err == nil && cnpd.reconcileInProgress {
			cnpd.reconcileAfter.sfcIDs[key] = *sfcID
		}
	}

	return nil
}

// createInterContainerMemIfPair creates memif pair and returns vswitch-end memif interface name
func (cnpd *sfcCtlrL2CNPDriver) createInterContainerMemIfPair(
	sfcName string,
	vnf1Container string, vnf1Port string,
	vnf2Container string, vnf2Port string,
	mtu uint32,
	memIFID uint32) error {

	log.Infof("createInterContainerMemIfPair: vnf1: '%s'/'%s', vnf2: '%s'/'%s', memIfID: '%d'",
		vnf1Container, vnf1Port, vnf2Container, vnf2Port, memIFID)

	// create a memif in the vnf container 1
	if _, err := cnpd.memIfCreate(vnf1Container, vnf1Port, memIFID, true, "", "", mtu); err != nil {
		log.Error("createInterContainerMemIfPair: error creating memIf for container: '%s'/'%s', memIF: '%d'",
			vnf1Container, vnf1Port, memIFID)
		return err
	}

	// create a memif in the vnf container 2
	if _, err := cnpd.memIfCreate(vnf2Container, vnf2Port, memIFID, false, "", "", mtu); err != nil {

		log.Error("createInterContainerMemIfPair: error creating memIf for container: '%s'/'%s', memIF: '%d'",
			vnf1Container, vnf1Port, memIFID)
		return err
	}

	return nil
}

// createMemIfPair creates memif pair and returns vswitch-end memif interface name
func (cnpd *sfcCtlrL2CNPDriver) createMemIfPair(sfc *controller.SfcEntity, hostName string,
	vnfChainElement *controller.SfcEntity_SfcElement, generateAddresses bool) (string, error) {

	log.Infof("createMemIfPair: vnf: '%s', host: '%s'", vnfChainElement.Container, hostName)

	var memifID uint32
	var macAddrID uint32
	var ipID uint32

	sfcID, err := cnpd.DatastoreSFCIDsRetrieve(sfc.Name, vnfChainElement.Container, vnfChainElement.PortLabel)
	if sfcID == nil || sfcID.MemifId == 0 {
		cnpd.seq.MemIfID++
		memifID = cnpd.seq.MemIfID
	} else {
		memifID = sfcID.MemifId
	}

	var macAddress string
	var ipv4Address string

	// the sfc controller can be responsible for managing the mac and ip addresses if not provided t
	// TODO: figure out how ipam, and macam should be done, for now just start at 1 and go up :-(
	if vnfChainElement.Ipv4Addr == "" {
		if generateAddresses {
			if sfc.SfcIpv4Prefix != "" {
				// still need to do ipam, but I will fill in the last octet with the instance id for now
				if sfcID == nil || sfcID.IpId == 0 {
					cnpd.seq.IPInstanceID++
					ipv4Address = formatIpv4AddressFromSfcPrefix(sfc.SfcIpv4Prefix, cnpd.seq.IPInstanceID)
					ipID = cnpd.seq.IPInstanceID
				} else {
					ipv4Address = formatIpv4AddressFromSfcPrefix(sfc.SfcIpv4Prefix, sfcID.IpId)
					ipID = sfcID.IpId
				}
			}
		}
	} else {
		ipv4Address = vnfChainElement.Ipv4Addr + "/24"
	}

	if vnfChainElement.MacAddr == "" {
		if generateAddresses {
			if sfcID == nil || sfcID.MacAddrId == 0 {
				cnpd.seq.MacInstanceID++
				macAddress = formatMacAddress(cnpd.seq.MacInstanceID)
				macAddrID = cnpd.seq.MacInstanceID
			} else {
				macAddress = formatMacAddress(sfcID.MacAddrId)
				macAddrID = sfcID.MacAddrId
			}
		}
	} else {
		macAddress = vnfChainElement.MacAddr
	}

	mtu := cnpd.getMtu(vnfChainElement.Mtu)

	// create a memif in the vnf container
	memIfName := vnfChainElement.PortLabel
	if _, err := cnpd.memIfCreate(vnfChainElement.Container, memIfName, memifID,
		false, ipv4Address, macAddress, mtu); err != nil {
		log.Error("createMemIfPair: error creating memIf for container: '%s'", memIfName)
		return "", err
	}

	// now create a memif for the vpp switch
	memIfName = "IF_MEMIF_VSWITCH_" + vnfChainElement.Container + "_" + vnfChainElement.PortLabel
	memIf, err := cnpd.memIfCreate(vnfChainElement.EtcdVppSwitchKey, memIfName, memifID,
		true, "", "", mtu)
	if err != nil {
		log.Error("createMemIfPair: error creating memIf for vpp switch: '%s'", memIf.Name)
		return "", err
	}

	key, sfcID, err := cnpd.DatastoreSFCIDsCreate(sfc.Name, vnfChainElement.Container, vnfChainElement.PortLabel,
		ipID, macAddrID, memifID)
	if err == nil && cnpd.reconcileInProgress {
		cnpd.reconcileAfter.sfcIDs[key] = *sfcID
	}

	return memIfName, err
}

// createMemIfPairAndAddToBridge creates a memif pair and adds the vswitch-end interface into the provided bridge domain
func (cnpd *sfcCtlrL2CNPDriver) createMemIfPairAndAddToBridge(sfc *controller.SfcEntity, hostName string,
	bd *l2.BridgeDomains_BridgeDomain, vnfChainElement *controller.SfcEntity_SfcElement, generateAddresses bool) error {

	memIfName, err := cnpd.createMemIfPair(sfc, hostName, vnfChainElement, generateAddresses)
	if err != nil {
		return err
	}

	ifEntry := l2.BridgeDomains_BridgeDomain_Interfaces{
		Name: memIfName,
	}
	ifs := make([]*l2.BridgeDomains_BridgeDomain_Interfaces, 1)
	ifs[0] = &ifEntry

	if err := cnpd.bridgedDomainAssociateWithIfs(vnfChainElement.EtcdVppSwitchKey, bd, ifs); err != nil {
		log.Error("createMemIfPairAndAddToBridge: error creating BD: '%s'", bd.Name)
		return err
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) createAFPacketVEthPair(sfc *controller.SfcEntity,
	vnfChainElement *controller.SfcEntity_SfcElement) (string, error) {

	log.Infof("createAFPacketVEthPair: vnf: '%s', host: '%s'", vnfChainElement.Container,
		vnfChainElement.EtcdVppSwitchKey)

	var macAddrID uint32
	var ipID uint32
	var macAddress string
	var ipv4Address string

	sfcID, err := cnpd.DatastoreSFCIDsRetrieve(sfc.Name, vnfChainElement.Container, vnfChainElement.PortLabel)

	// the sfc controller can be responsible for managing the mac and ip addresses if not provided t
	if vnfChainElement.Type != controller.SfcElementType_VPP_CONTAINER_AFP {
		// no IP address in case that VPP is also on the vnf side of the vnf-vswitch entry

		// TODO: figure out how ipam, and macam should be done, for now just start at 1 and increment :-(
		if vnfChainElement.Ipv4Addr == "" {
			if sfc.SfcIpv4Prefix != "" {
				// still need to do ipam, but I will fill in the last octet with the instance id for now
				if sfcID == nil || sfcID.IpId == 0 {
					cnpd.seq.IPInstanceID++
					ipv4Address = formatIpv4AddressFromSfcPrefix(sfc.SfcIpv4Prefix, cnpd.seq.IPInstanceID)
					ipID = cnpd.seq.IPInstanceID
				} else {
					ipv4Address = formatIpv4AddressFromSfcPrefix(sfc.SfcIpv4Prefix, sfcID.IpId)
					ipID = sfcID.IpId
				}
			}
		} else {
			ipv4Address = vnfChainElement.Ipv4Addr + "/24"
		}
	}
	if vnfChainElement.MacAddr == "" {
		if sfcID == nil || sfcID.MacAddrId == 0 {
			cnpd.seq.MacInstanceID++
			macAddress = formatMacAddress(cnpd.seq.MacInstanceID)
			macAddrID = cnpd.seq.MacInstanceID
		} else {
			macAddress = formatMacAddress(sfcID.MacAddrId)
			macAddrID = sfcID.MacAddrId
		}
	} else {
		macAddress = vnfChainElement.MacAddr
	}

	mtu := cnpd.getMtu(vnfChainElement.Mtu)

	// Create a VETH if for the vnf container. VETH will get created by the agent from a more privileged vswitch.
	// Note: In Linux kernel the length of an interface name is limited by the constant IFNAMSIZ.
	//       In most distributions this is 16 characters including the terminating NULL character.
	veth1Name := utils.TruncateString(vnfChainElement.PortLabel, 15)
	veth2Name := utils.TruncateString(vnfChainElement.Container, 7) + "_" +
		utils.TruncateString(vnfChainElement.PortLabel, 7)
	if err := cnpd.vEthIfCreate(vnfChainElement.EtcdVppSwitchKey, veth1Name, veth2Name, vnfChainElement.Container,
		macAddress, ipv4Address, mtu); err != nil {
		log.Error("createAFPacketVEthPair: error creating veth if '%s' for container: '%s'", veth1Name,
			vnfChainElement.Container)
		return "", err
	}
	// Configure opposite side of the VETH interface for the vpp switch
	if err := cnpd.vEthIfCreate(vnfChainElement.EtcdVppSwitchKey, veth2Name, veth1Name, vnfChainElement.EtcdVppSwitchKey,
		"", "", mtu); err != nil {
		log.Error("createAFPacketVEthPair: error creating veth if '%s' for container: '%s'", veth2Name,
			vnfChainElement.EtcdVppSwitchKey)
		return "", err
	}

	// create af_packet for the vnf -end of the veth
	if vnfChainElement.Type == controller.SfcElementType_VPP_CONTAINER_AFP {
		afPktIf1, err := cnpd.afPacketCreate(vnfChainElement.Container, vnfChainElement.PortLabel,
			veth1Name, "", "", mtu)
		if err != nil {
			log.Error("createAFPacketVEthPair: error creating afpacket for vpp switch: '%s'", afPktIf1.Name)
			return "", err
		}
	}

	// create af_packet for the vswitch -end of the veth
	afPktName := "IF_AFPIF_VSWITCH_" + vnfChainElement.Container + "_" + vnfChainElement.PortLabel
	afPktIf2, err := cnpd.afPacketCreate(vnfChainElement.EtcdVppSwitchKey, afPktName, veth2Name,
		"", "", mtu)
	if err != nil {
		log.Error("createAFPacketVEthPair: error creating afpacket for vpp switch: '%s'", afPktIf2.Name)
		return "", err
	}

	key, sfcID, err := cnpd.DatastoreSFCIDsCreate(sfc.Name, vnfChainElement.Container, vnfChainElement.PortLabel,
		ipID, macAddrID, 0)
	if err == nil && cnpd.reconcileInProgress {
		cnpd.reconcileAfter.sfcIDs[key] = *sfcID
	}

	return afPktIf2.Name, nil
}

func (cnpd *sfcCtlrL2CNPDriver) createAFPacketVEthPairAndAddToBridge(sfc *controller.SfcEntity,
	bd *l2.BridgeDomains_BridgeDomain, vnfChainElement *controller.SfcEntity_SfcElement) error {

	log.Infof("createAFPacketVEthPairAndAddToBridge: vnf: '%s', host: '%s'", vnfChainElement.Container,
		vnfChainElement.EtcdVppSwitchKey)

	afPktIfName, err := cnpd.createAFPacketVEthPair(sfc, vnfChainElement)
	if err != nil {
		return err
	}

	ifEntry := l2.BridgeDomains_BridgeDomain_Interfaces{
		Name: afPktIfName,
	}
	ifs := make([]*l2.BridgeDomains_BridgeDomain_Interfaces, 1)
	ifs[0] = &ifEntry

	if err := cnpd.bridgedDomainAssociateWithIfs(vnfChainElement.EtcdVppSwitchKey, bd, ifs); err != nil {
		log.Error("createAFPacketVEthPairAndAddToBridge: error creating BD: '%s'", bd.Name)
		return err
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) bridgedDomainCreateWithIfs(etcdVppSwitchKey string, bdName string,
	ifs []*l2.BridgeDomains_BridgeDomain_Interfaces) (*l2.BridgeDomains_BridgeDomain, error) {

	bd := &l2.BridgeDomains_BridgeDomain{
		Name:                bdName,
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      false,
		MacAge:              0,
		Interfaces:          ifs,
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileBridgeDomain(etcdVppSwitchKey, bd)
	} else {

		log.Println(bd)

		rc := NewRemoteClientTxn(etcdVppSwitchKey, cnpd.dbFactory)
		err := rc.Put().BD(bd).Send().ReceiveReply()

		if err != nil {
			log.Error("vxLanCreate: databroker.Store: ", err)
			return nil, err

		}
	}

	return bd, nil
}

// using the existing bridge, append the new if to the existing ifs in the bridge
func (cnpd *sfcCtlrL2CNPDriver) bridgedDomainAssociateWithIfs(etcdVppSwitchKey string,
	bd *l2.BridgeDomains_BridgeDomain,
	ifs []*l2.BridgeDomains_BridgeDomain_Interfaces) error {

	// only add the interface to bd array if it is not already in the bridge's interface array
	for _, iface := range ifs {
		found := false
		for _, bi := range bd.Interfaces {
			if bi.Name == iface.Name {
				found = true
				break
			}
		}
		if !found {
			bd.Interfaces = append(bd.Interfaces, iface)
		}
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileBridgeDomain(etcdVppSwitchKey, bd)
	} else {

		log.Println(bd)

		rc := NewRemoteClientTxn(etcdVppSwitchKey, cnpd.dbFactory)
		err := rc.Put().BD(bd).Send().ReceiveReply()

		if err != nil {
			log.Error("vxLanCreate: databroker.Store: ", err)
			return err

		}
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) vxLanCreate(etcdVppSwitchKey string, ifname string, vni uint32,
	srcStr string, dstStr string) (*interfaces.Interfaces_Interface, error) {

	src := stripSlashAndSubnetIpv4Address(srcStr)
	dst := stripSlashAndSubnetIpv4Address(dstStr)

	iface := &interfaces.Interfaces_Interface{
		Name:    ifname,
		Type:    interfaces.InterfaceType_VXLAN_TUNNEL,
		Enabled: true,
		Vxlan: &interfaces.Interfaces_Interface_Vxlan{
			SrcAddress: src,
			DstAddress: dst,
			Vni:        vni,
		},
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileInterface(etcdVppSwitchKey, iface)
	} else {

		log.Println(*iface)

		rc := NewRemoteClientTxn(etcdVppSwitchKey, cnpd.dbFactory)
		err := rc.Put().VppInterface(iface).Send().ReceiveReply()

		if err != nil {
			log.Error("vxLanCreate: databroker.Store: ", err)
			return nil, err

		}
	}

	return iface, nil
}

func (cnpd *sfcCtlrL2CNPDriver) memIfCreate(etcdPrefix string, memIfName string, memifID uint32, isMaster bool,
	ipv4 string, macAddress string, mtu uint32) (*interfaces.Interfaces_Interface, error) {

	memIf := &interfaces.Interfaces_Interface{
		Name:        memIfName,
		Type:        interfaces.InterfaceType_MEMORY_INTERFACE,
		Enabled:     true,
		PhysAddress: macAddress,
		Mtu:         mtu,
		Memif: &interfaces.Interfaces_Interface_Memif{
			Id:             memifID,
			Master:         isMaster,
			SocketFilename: "/tmp/memif.sock",
		},
	}

	if ipv4 != "" {
		memIf.IpAddresses = make([]string, 1)
		memIf.IpAddresses[0] = ipv4
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileInterface(etcdPrefix, memIf)
	} else {

		log.Println(*memIf)

		rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
		err := rc.Put().VppInterface(memIf).Send().ReceiveReply()

		if err != nil {
			log.Error("memIfCreate: databroker.Store: ", err)
			return nil, err

		}
	}

	return memIf, nil
}

func (cnpd *sfcCtlrL2CNPDriver) createEthernet(etcdPrefix string, ifname string, ipv4Addr string, mtu uint32) error {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_ETHERNET_CSMACD,
		Enabled:     true,
		PhysAddress: "",
		Mtu:         mtu,
	}
	if ipv4Addr != "" {
		iface.IpAddresses = make([]string, 1)
		iface.IpAddresses[0] = ipv4Addr
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileInterface(etcdPrefix, iface)
	} else {

		log.Println(*iface)

		rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
		err := rc.Put().VppInterface(iface).Send().ReceiveReply()

		if err != nil {
			log.Error("createEthernet: databroker.Store: ", err)
			return err

		}
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) afPacketCreate(etcdPrefix string, ifName string, hostIfName string,
	ipv4 string, macAddress string, mtu uint32) (*interfaces.Interfaces_Interface, error) {

	afPacketIf := &interfaces.Interfaces_Interface{
		Name:        ifName,
		Type:        interfaces.InterfaceType_AF_PACKET_INTERFACE,
		Enabled:     true,
		PhysAddress: macAddress,
		Mtu:         mtu,
		Afpacket: &interfaces.Interfaces_Interface_Afpacket{
			HostIfName: hostIfName,
		},
	}

	if ipv4 != "" {
		afPacketIf.IpAddresses = make([]string, 1)
		afPacketIf.IpAddresses[0] = ipv4
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileInterface(etcdPrefix, afPacketIf)
	} else {

		log.Println(*afPacketIf)

		rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
		err := rc.Put().VppInterface(afPacketIf).Send().ReceiveReply()

		if err != nil {
			log.Error("afPacketCreate: databroker.Store: ", err)
			return nil, err

		}
	}

	return afPacketIf, nil
}

func (cnpd *sfcCtlrL2CNPDriver) createLoopback(etcdPrefix string, ifname string, physAddr string, ipv4Addr string,
	mtu uint32) error {

	iface := &interfaces.Interfaces_Interface{
		Name:        ifname,
		Type:        interfaces.InterfaceType_SOFTWARE_LOOPBACK,
		Enabled:     true,
		PhysAddress: physAddr,
		Mtu:         mtu,
	}
	if ipv4Addr != "" {
		iface.IpAddresses = make([]string, 1)
		iface.IpAddresses[0] = ipv4Addr
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileInterface(etcdPrefix, iface)
	} else {

		log.Println(*iface)

		rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
		err := rc.Put().VppInterface(iface).Send().ReceiveReply()

		if err != nil {
			log.Error("createLoopback: databroker.Store: ", err)
			return err

		}
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) vEthIfCreate(etcdPrefix string, ifname string, peerIfName string, container string,
	physAddr string, ipv4Addr string, mtu uint32) error {

	linuxif := &linuxIntf.LinuxInterfaces_Interface{
		Name:        ifname,
		Type:        linuxIntf.LinuxInterfaces_VETH,
		Enabled:     true,
		PhysAddress: physAddr,
		HostIfName:  ifname,
		Mtu:         mtu,
		Namespace: &linuxIntf.LinuxInterfaces_Interface_Namespace{
			Type:         linuxIntf.LinuxInterfaces_Interface_Namespace_MICROSERVICE_REF_NS,
			Microservice: container,
		},
		Veth: &linuxIntf.LinuxInterfaces_Interface_Veth{
			PeerIfName: peerIfName,
		},
	}

	if ipv4Addr != "" {
		linuxif.IpAddresses = make([]string, 1)
		linuxif.IpAddresses[0] = ipv4Addr
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileLinuxInterface(etcdPrefix, ifname, linuxif)
	} else {

		log.Println(linuxif)

		rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
		err := rc.Put().LinuxInterface(linuxif).Send().ReceiveReply()

		if err != nil {
			log.Error("createLoopback: databroker.Store: ", err)
			return err
		}
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) createStaticRoute(etcdPrefix string, description string, destIpv4AddrStr string,
	netHopIpv4Addr string, outGoingIf string) (*l3.StaticRoutes_Route, error) {

	sr := &l3.StaticRoutes_Route{
		VrfId:             0,
		Description:       description,
		DstIpAddr:         destIpv4AddrStr,
		NextHopAddr:       stripSlashAndSubnetIpv4Address(netHopIpv4Addr),
		Weight:            5,
		OutgoingInterface: outGoingIf,
	}

	if cnpd.reconcileInProgress {
		cnpd.reconcileStaticRoute(etcdPrefix, sr)
	} else {

		log.Println(sr)

		rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
		err := rc.Put().StaticRoute(sr).Send().ReceiveReply()

		if err != nil {
			log.Error("createStaticRoute: databroker.Store: ", err)
			return nil, err

		}
	}

	return sr, nil
}

func (cnpd *sfcCtlrL2CNPDriver) createXConnectPair(etcdPrefix, if1, if2 string) error {

	err := cnpd.createXConnect(etcdPrefix, if1, if2)
	if err != nil {
		return err
	}

	err = cnpd.createXConnect(etcdPrefix, if2, if1)
	if err != nil {
		return err
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) createXConnect(etcdPrefix, rxIf, txIf string) error {

	xconn := &l2.XConnectPairs_XConnectPair{
		ReceiveInterface:  rxIf,
		TransmitInterface: txIf,
	}

	log.Debugf("Storing l2xconnect config: %s", xconn)

	rc := NewRemoteClientTxn(etcdPrefix, cnpd.dbFactory)
	err := rc.Put().XConnect(xconn).Send().ReceiveReply()
	if err != nil {
		log.Errorf("Error by storing l2xconnect: %s", err)
		return err
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) createCustomLabel(etcdPrefix string, ci *controller.CustomInfoType) error {

	if ci != nil && ci.Label != "" {
		key := utils.CustomInfoKey(etcdPrefix)

		log.Println(key)
		log.Println(ci)

		err := cnpd.db.Put(key, ci)
		if err != nil {
			log.Error("createCustomLabel: error storing key: '%s'", key)
			log.Error("createCustomLabel: databroker.Store: ", err)
			return err
		}
	}

	return nil
}

// Debug dump routine
func (cnpd *sfcCtlrL2CNPDriver) Dump() {
	log.Println(cnpd.seq)
	log.Println(cnpd.l2CNPEntityCache)
	log.Println(cnpd.l2CNPStateCache)
}

func (cnpd *sfcCtlrL2CNPDriver) getHEToEEState(heName string, eeName string) *heToEEStateType {

	eeMap, exists := cnpd.l2CNPStateCache.HEToEEs[heName]
	if !exists {
		return nil
	}
	return eeMap[eeName]
}

func (cnpd *sfcCtlrL2CNPDriver) getMtu(mtu uint32) uint32 {

	log.Info("getMtu: ", mtu)
	if mtu == 0 {
		mtu = cnpd.l2CNPEntityCache.SysParms.Mtu
		log.Info("getMtu: replacing with system value: ", mtu)
	}
	return mtu
}

func formatMacAddress(macInstanceID uint32) string {
	return "02:00:00:00:00:" + fmt.Sprintf("%02X", macInstanceID)
}

// if the ip address has a /xx subnet attached, it is stripped off
func stripSlashAndSubnetIpv4Address(ipAndSubnetStr string) string {
	strs := strings.Split(ipAndSubnetStr, "/")
	return strs[0]
}

func formatIpv4AddressFromSfcPrefix(ipAndSubnetStr string, ipInstanceID uint32) string {
	strs := strings.Split(ipAndSubnetStr, "/")
	octets := strings.Split(strs[0], ".")
	newStr := octets[0] + "." + octets[1] + "." + octets[2] + "." +
		fmt.Sprintf("%d", ipInstanceID) + "/" + strs[1]
	log.Info("formatIpv4AddressFromSfcPrefix: ", strs, octets, ipInstanceID)
	log.Info("formatIpv4AddressFromSfcPrefix: result ip addr: ", newStr)
	return newStr
}
