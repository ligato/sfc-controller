//Copyright (c) 2017 Cisco and/or its affiliates.
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

// The reconcile/resync procedures are implemented is this file.

package l2driver

import (
	"fmt"
	"github.com/ligato/cn-infra/utils/addrs"
	l2driver "github.com/ligato/sfc-controller/controller/cnpdriver/l2driver/model"
	"github.com/ligato/sfc-controller/controller/utils"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l3plugin/model/l3"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linuxplugin/ifplugin/model/interfaces"
)

type reconcileCacheType struct {
	// maps of ETCD entries indexed by ETCD vpp label
	ifs      map[string]interfaces.Interfaces_Interface
	lifs     map[string]linuxIntf.LinuxInterfaces_Interface
	bds      map[string]l2.BridgeDomains_BridgeDomain
	l3Routes map[string]l3.StaticRoutes_Route

	// maps of ETCD entries indexed by ETCD key
	heIDs    map[string]l2driver.HEIDs
	he2eeIDs map[string]l2driver.HE2EEIDs
	he2heIDs map[string]l2driver.HE2HEIDs
	sfcIDs   map[string]l2driver.SFCIDs
}

func (cnpd *sfcCtlrL2CNPDriver) initReconcileCache() error {

	cnpd.reconcileBefore.ifs = make(map[string]interfaces.Interfaces_Interface)
	cnpd.reconcileBefore.lifs = make(map[string]linuxIntf.LinuxInterfaces_Interface)
	cnpd.reconcileBefore.bds = make(map[string]l2.BridgeDomains_BridgeDomain)
	cnpd.reconcileBefore.l3Routes = make(map[string]l3.StaticRoutes_Route)
	cnpd.reconcileBefore.heIDs = make(map[string]l2driver.HEIDs)
	cnpd.reconcileBefore.he2eeIDs = make(map[string]l2driver.HE2EEIDs)
	cnpd.reconcileBefore.he2heIDs = make(map[string]l2driver.HE2HEIDs)
	cnpd.reconcileBefore.sfcIDs = make(map[string]l2driver.SFCIDs)

	cnpd.reconcileAfter.ifs = make(map[string]interfaces.Interfaces_Interface)
	cnpd.reconcileAfter.lifs = make(map[string]linuxIntf.LinuxInterfaces_Interface)
	cnpd.reconcileAfter.bds = make(map[string]l2.BridgeDomains_BridgeDomain)
	cnpd.reconcileAfter.l3Routes = make(map[string]l3.StaticRoutes_Route)
	cnpd.reconcileAfter.heIDs = make(map[string]l2driver.HEIDs)
	cnpd.reconcileAfter.he2eeIDs = make(map[string]l2driver.HE2EEIDs)
	cnpd.reconcileAfter.he2heIDs = make(map[string]l2driver.HE2HEIDs)
	cnpd.reconcileAfter.sfcIDs = make(map[string]l2driver.SFCIDs)

	return nil
}

// Perform start processing for the reconcile of the CNP datastore
func (cnpd *sfcCtlrL2CNPDriver) ReconcileStart(vppEtcdLabels map[string]struct{}) error {

	// The reconcile for the l2 overlay data structures consists of all the types of objects that are created as a
	// result of processing the EEs, HEs, and SFCs.  These are interfaces, bridged domains, and static routes, etc.
	// When reconcile starts, we read all of these from ETCD and store them in the reconcile "before" cache.
	// Then as the configuration is processed, the "new" objects are added to a reconcile "after" cache.  When all
	// the configuration is processed, a post processing of the before and after caches is performed.
	// All entries in the before cache are processed one-by-one.  If the before entry is not in the after cache, then
	// clearly it is not needed and removed from ETCD.  If it is in the after cache, then there are two cases.
	// If the entries match, it is removed from the after cache and ETCD is not "touched".  If the entries are
	// different, it remains in the after cache and awaits "after" cache post processing.  Once all the before
	// entries have been processed, the after cache is processed.  If there are entries in this cache, they are
	// all written to ETCD.

	// The reason for this reconcile approach is as follows: some ETCD entries will be added and updated multiple
	// times during processing of the configuration and there is NO sense continually changing ETCD for an entry
	// until it is fully modified by the configuration processing.  This is why an "after" cache is maintained.  Then
	// post processing will ensure the "final" values of an object are written only ONCE to the ETCD cache.
	// An example of this is bridge domains.  Initially for a host, a default east-west BD is added to the system,
	// then as interfaces are associated with the BD, the BD is updated.  If we tried to continually update the
	// ETCD entry for this BD as we went along, we would improperly set the BD to interim configs until it all
	// the config is performed and the BD reaches its final config.  This would have bad effects on the vpp-agents
	// as they would be forced to react to each BD change and data flow would be affected.  The goal of the
	// reconcile resync is to ONLY make changes if there are new and/or obselete configs.  Existing configs should
	// reamin un-affected by the resync process.

	log.Info("ReconcileStart: begin ...")
	defer log.Info("ReconcileStart: exit ...")

	cnpd.reconcileStateSet(true)

	for vppEtdLabel := range vppEtcdLabels {
		cnpd.reconcileLoadInterfacesIntoCache(vppEtdLabel)
		cnpd.reconcileLoadLinuxInterfacesIntoCache(vppEtdLabel)
		cnpd.reconcileLoadBridgeDomainsIntoCache(vppEtdLabel)
		cnpd.reconcileLoadStaticRoutesIntoCache(vppEtdLabel)
	}

	cnpd.reconcileLoadHEIDsIntoCache()
	cnpd.reconcileLoadHE2EEIDsIntoCache()
	cnpd.reconcileLoadHE2HEIDsIntoCache()
	cnpd.reconcileLoadSFCIDsIntoCache()

    cnpd.sequencerInitFromReconcileCache()

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileStateSet(state bool) {
	cnpd.reconcileInProgress = state
}

// Perform end processing for the reconcile of the CNP datastore
func (cnpd *sfcCtlrL2CNPDriver) ReconcileEnd() error {

	log.Info("ReconcileEnd: begin ...")
	log.Infof("ReconcileEnd: reconcileBefore", cnpd.reconcileBefore)
	log.Infof("ReconcileEnd: reconcileAfter", cnpd.reconcileAfter)
	defer cnpd.reconcileStateSet(false)
	defer log.Info("ReconcileEnd: exit ...")

	// 1) For each entry in the before cache, look it up in the after cache
	//    if it is not in the after cache, delete it from ETCD, and from the after cache
	//    if it is in the after cache, then compare the entry
	//        if equal, remove from the after cache
	//        if not equal do nothing
	// 2) I am using the String() method to convert the entry then comparing strings (inefficient/ok?)

	// Interfaces: traverse the before cache
	for key := range cnpd.reconcileBefore.ifs {
		beforeIF := cnpd.reconcileBefore.ifs[key]
		afterIF, existsInAfterCache := cnpd.reconcileAfter.ifs[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove i/f key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.ifs, key)
		} else {
			if beforeIF.String() == afterIF.String() {
				delete(cnpd.reconcileAfter.ifs, key)
			}
		}
	}
	// Interfaces: now post process the after cache
	for key := range cnpd.reconcileAfter.ifs {
		afterIF := cnpd.reconcileAfter.ifs[key]
		log.Info("ReconcileEnd: add i/f key to etcd: ", key, afterIF)
		err := cnpd.db.Put(key, &afterIF)
		if err != nil {
			log.Error("ReconcileEnd: error storing i/f: '%s'", key, err)
			return err
		}
	}

	// Linux Interfaces: traverse the before cache
	for key := range cnpd.reconcileBefore.lifs {
		beforeIF := cnpd.reconcileBefore.lifs[key]
		afterIF, existsInAfterCache := cnpd.reconcileAfter.lifs[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove linux i/f key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.ifs, key)
		} else {
			if beforeIF.String() == afterIF.String() {
				delete(cnpd.reconcileAfter.lifs, key)
			}
		}
	}
	// Linux Interfaces: now post process the after cache
	for key := range cnpd.reconcileAfter.lifs {
		afterIF := cnpd.reconcileAfter.lifs[key]
		log.Info("ReconcileEnd: add linux i/f key to etcd: ", key, afterIF)
		err := cnpd.db.Put(key, &afterIF)
		if err != nil {
			log.Error("ReconcileEnd: error storing i/f: '%s'", key, err)
			return err
		}
	}

	// Bridge Domains: traverse the before cache
	for key := range cnpd.reconcileBefore.bds {
		beforeBD := cnpd.reconcileBefore.bds[key]
		afterBD, existsInAfterCache := cnpd.reconcileAfter.bds[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove BD key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.bds, key)
		} else {
			cnpd.sortBridgedInterfaces(beforeBD.Interfaces)
			cnpd.sortBridgedInterfaces(afterBD.Interfaces)
			if beforeBD.String() == afterBD.String() {
				delete(cnpd.reconcileAfter.bds, key)
			}
		}
	}
	// Bridge Domains: now post process the after cache
	for key := range cnpd.reconcileAfter.bds {
		afterBD := cnpd.reconcileAfter.bds[key]
		log.Info("ReconcileEnd: add BD key to etcd: ", key, afterBD)
		err := cnpd.db.Put(key, &afterBD)
		if err != nil {
			log.Error("ReconcileEnd: error storing BD: '%s'", key, err)
			return err
		}
	}

	// Static Routes: traverse the before cache
	for key := range cnpd.reconcileBefore.l3Routes {
		beforeSR := cnpd.reconcileBefore.l3Routes[key]
		afterSR, existsInAfterCache := cnpd.reconcileAfter.l3Routes[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove static route key from etcd and reconcile cache: ", key, exists, err)
			log.Info("ReconcileEnd: remove static route before entry: ", beforeSR)
			delete(cnpd.reconcileAfter.l3Routes, key)
		} else {
			if beforeSR == afterSR {
				delete(cnpd.reconcileAfter.l3Routes, key)
			}
		}
	}
	// Static Routes: now post process the after cache
	for key := range cnpd.reconcileAfter.l3Routes {
		afterSR := cnpd.reconcileAfter.l3Routes[key]
		log.Info("ReconcileEnd: add static route key to etcd: ", key, afterSR)
		err := cnpd.db.Put(key, &afterSR)
		if err != nil {
			log.Error("ReconcileEnd: error storing static route: '%s'", key, err)
			return err
		}
	}

	// HE IDs: traverse the before cache
	for key := range cnpd.reconcileBefore.heIDs {
		beforeHEID := cnpd.reconcileBefore.heIDs[key]
		afterHEID, existsInAfterCache := cnpd.reconcileAfter.heIDs[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove HE ID key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.heIDs, key)
		} else {
			if beforeHEID.String() == afterHEID.String() {
				delete(cnpd.reconcileAfter.heIDs, key)
			}
		}
	}
	// HE IDs: now post process the after cache
	for key := range cnpd.reconcileAfter.heIDs {
		afterHEID := cnpd.reconcileAfter.heIDs[key]
		log.Info("ReconcileEnd: add HE ID key to etcd: ", key, afterHEID)
		err := cnpd.db.Put(key, &afterHEID)
		if err != nil {
			log.Error("ReconcileEnd: error storing HE ID: '%s'", key, err)
			return err
		}
	}

	// HE to EE IDs: traverse the before cache
	for key := range cnpd.reconcileBefore.he2eeIDs {
		beforeHE2EEID := cnpd.reconcileBefore.he2eeIDs[key]
		afterHE2EEID, existsInAfterCache := cnpd.reconcileAfter.he2eeIDs[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove HE2EE ID key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.he2eeIDs, key)
		} else {
			if beforeHE2EEID.String() == afterHE2EEID.String() {
				delete(cnpd.reconcileAfter.he2eeIDs, key)
			}
		}
	}
	// HE to EE IDs: now post process the after cache
	for key := range cnpd.reconcileAfter.he2eeIDs {
		afterHE2EEID := cnpd.reconcileAfter.he2eeIDs[key]
		log.Info("ReconcileEnd: add HE2EE ID key to etcd: ", key, afterHE2EEID)
		err := cnpd.db.Put(key, &afterHE2EEID)
		if err != nil {
			log.Error("ReconcileEnd: error storing HE2EE ID: '%s'", key, err)
			return err
		}
	}

	// HE to HE IDs: traverse the before cache
	for key := range cnpd.reconcileBefore.he2heIDs {
		beforeHE2HEID := cnpd.reconcileBefore.he2heIDs[key]
		afterHE2HEID, existsInAfterCache := cnpd.reconcileAfter.he2heIDs[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove HE2HE ID key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.he2heIDs, key)
		} else {
			if beforeHE2HEID.String() == afterHE2HEID.String() {
				delete(cnpd.reconcileAfter.he2heIDs, key)
			}
		}
	}
	// HE to HE IDs: now post process the after cache
	for key := range cnpd.reconcileAfter.he2heIDs {
		afterHE2HEID := cnpd.reconcileAfter.he2heIDs[key]
		log.Info("ReconcileEnd: add HE2HE ID key to etcd: ", key, afterHE2HEID)
		err := cnpd.db.Put(key, &afterHE2HEID)
		if err != nil {
			log.Error("ReconcileEnd: error storing HE2HE ID: '%s'", key, err)
			return err
		}
	}

	// SFC IDs: traverse the before cache
	for key := range cnpd.reconcileBefore.sfcIDs {
		beforeSFCID := cnpd.reconcileBefore.sfcIDs[key]
		afterSFCID, existsInAfterCache := cnpd.reconcileAfter.sfcIDs[key]
		if !existsInAfterCache {
			exists, err := cnpd.db.Delete(key)
			log.Info("ReconcileEnd: remove SFC ID key from etcd and reconcile cache: ", key, exists, err)
			delete(cnpd.reconcileAfter.sfcIDs, key)
		} else {
			if beforeSFCID.String() == afterSFCID.String() {
				delete(cnpd.reconcileAfter.sfcIDs, key)
			}
		}
	}
	// SFC IDs: now post process the after cache
	for key := range cnpd.reconcileAfter.sfcIDs {
		afterSFCID := cnpd.reconcileAfter.sfcIDs[key]
		log.Info("ReconcileEnd: add SFC ID key to etcd: ", key, afterSFCID)
		err := cnpd.db.Put(key, &afterSFCID)
		if err != nil {
			log.Error("ReconcileEnd: error storing SFC ID: '%s'", key, err)
			return err
		}
	}

	return nil
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileBridgeDomain(etcdVppSwitchKey string, bd *l2.BridgeDomains_BridgeDomain) {
	bdKey := utils.L2BridgeDomainKey(etcdVppSwitchKey, bd.Name)
	cnpd.reconcileAfter.bds[bdKey] = *bd
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileInterface(etcdVppSwitchKey string, currIf *interfaces.Interfaces_Interface) {
	ifKey := utils.InterfaceKey(etcdVppSwitchKey, currIf.Name)
	cnpd.reconcileAfter.ifs[ifKey] = *currIf
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLinuxInterface(etcdPrefix string, ifname string,
	currIf *linuxIntf.LinuxInterfaces_Interface) {

	ifKey := utils.LinuxInterfaceKey(etcdPrefix, ifname)
	cnpd.reconcileAfter.lifs[ifKey] = *currIf
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileStaticRoute(etcdPrefix string, sr *l3.StaticRoutes_Route) {

	destIPAddr, _, _ := addrs.ParseIPWithPrefix(sr.DstIpAddr)
	key := utils.L3RouteKey(etcdPrefix, sr.VrfId, destIPAddr, sr.NextHopAddr)
	cnpd.reconcileAfter.l3Routes[key] = *sr
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadInterfacesIntoCache(etcdVppLabel string) error {

	kvi, err := cnpd.db.ListValues(utils.InterfacePrefixKey(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &interfaces.Interfaces_Interface{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}

		fmt.Println("reconcileLoadInterfacesIntoCache: adding Interface: ", etcdVppLabel, kv.GetKey(), entry)
		cnpd.reconcileBefore.ifs[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadLinuxInterfacesIntoCache(etcdVppLabel string) error {

	kvi, err := cnpd.db.ListValues(utils.LinuxInterfacePrefixKey(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &linuxIntf.LinuxInterfaces_Interface{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadLinuxInterfacesIntoCache: adding linux nterface: ",
			etcdVppLabel, kv.GetKey(), entry)
		cnpd.reconcileBefore.lifs[kv.GetKey()] = *entry

	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadBridgeDomainsIntoCache(etcdVppLabel string) error {

	kvi, err := cnpd.db.ListValues(utils.L2BridgeDomainKeyPrefix(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l2.BridgeDomains_BridgeDomain{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadBridgeDomainsIntoCache: adding bridge doamin: ",
			etcdVppLabel, kv.GetKey(), entry)
		cnpd.reconcileBefore.bds[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadStaticRoutesIntoCache(etcdVppLabel string) error {

	kvi, err := cnpd.db.ListValues(utils.L3RouteKeyPrefix(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l3.StaticRoutes_Route{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadStaticRoutesIntoCache: adding static route: ", etcdVppLabel, kv.GetKey(), entry)
		cnpd.reconcileBefore.l3Routes[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadHEIDsIntoCache() error {

	kvi, err := cnpd.db.ListValues(l2driver.HEIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l2driver.HEIDs{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadHEIDsIntoCache: adding HE IDs: ", kv.GetKey(), entry)
		cnpd.reconcileBefore.heIDs[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadHE2EEIDsIntoCache() error {

	kvi, err := cnpd.db.ListValues(l2driver.HE2EEIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l2driver.HE2EEIDs{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadHE2EEIDsIntoCache: adding HE to EE IDs: ", kv.GetKey(), entry)
		cnpd.reconcileBefore.he2eeIDs[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadHE2HEIDsIntoCache() error {

	kvi, err := cnpd.db.ListValues(l2driver.HE2HEIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l2driver.HE2HEIDs{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadHE2HEIDsIntoCache: adding HE to HE IDs: ", kv.GetKey(), entry)
		cnpd.reconcileBefore.he2heIDs[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) reconcileLoadSFCIDsIntoCache() error {

	kvi, err := cnpd.db.ListValues(l2driver.SFCIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l2driver.SFCIDs{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("reconcileLoadSFCIDsIntoCache: adding SFC IDs: ", kv.GetKey(), entry)
		cnpd.reconcileBefore.sfcIDs[kv.GetKey()] = *entry
	}
}

func (cnpd *sfcCtlrL2CNPDriver) sequencerInitFromReconcileCache() {

	// the sequencer is responsible fore choosing unique id's ... after pulling in all the data from
	// the db, the data is traversed and the max value is remembered

	maxVlanID := uint32(0)
	maxMemifID := uint32(0)
	maxIpID  := uint32(0)
	maxMacAddrID := uint32(0)


	// traverse the id caches recording max id's

	for _, heId := range cnpd.reconcileBefore.heIDs {
		if heId.LoopbackMacAddrId > maxMacAddrID {
			maxMacAddrID = heId.LoopbackMacAddrId
		}
	}
	for _, he2ee := range cnpd.reconcileBefore.he2eeIDs {
		if he2ee.VlanId > maxVlanID {
			maxVlanID = he2ee.VlanId
		}
	}
	for _, he2he := range cnpd.reconcileBefore.he2heIDs {
		if he2he.VlanId > maxVlanID {
			maxVlanID = he2he.VlanId
		}
	}
	for _, sfc := range cnpd.reconcileBefore.sfcIDs {
		if sfc.MacAddrId > maxMacAddrID {
			maxMacAddrID = sfc.MacAddrId
		}
		if sfc.MemifId > maxMemifID {
			maxMemifID = sfc.MemifId
		}
		if sfc.IpId > maxIpID {
			maxIpID = sfc.IpId
		}
	}

	// now prime the sequencer with the max values from the db to avoid collisions

	cnpd.seq.VLanID = maxVlanID
	cnpd.seq.MemIfID = maxMemifID
	cnpd.seq.MacInstanceID = maxMacAddrID
	cnpd.seq.IPInstanceID = maxIpID

	fmt.Println("sequencerInitFromReconcileCache: sequence IDs after loading id's: ", cnpd.seq)
}
