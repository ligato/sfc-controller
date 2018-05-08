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

package controller

import (
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
	"sync"
)

// a cache of before and after entries per
// transaction type where a transaction could be a nb_api rest request,
// or a complete system resync which is initiated at startup after the
// db has been read in, or a loss of comms with etcd has occurred.
// Note: NO nb_api's are accepted during transaction processing as they
// are atomic operations and are mutex protected during each txn.
var beforeMap map[int]map[string]*vppagent.KVType
var afterMap map[int]map[string]*vppagent.KVType
var txnLevel int

var configMutex sync.Mutex

func RenderTxnGetAfterMap(key string) (*vppagent.KVType, bool) {
	vppKey, exists := afterMap[0][key]
	return vppKey, exists
}

// RenderTxnConfigStart initiates state for starting a config transaction
func RenderTxnConfigStart() {

	configMutex.Lock()

	log.Info("ConfigStart: starting ...")
	defer log.Info("ConfigStart: finished ...")

	beforeMap = nil
	afterMap = nil
	txnLevel = 0
	beforeMap = make(map[int]map[string]*vppagent.KVType)
	afterMap = make(map[int]map[string]*vppagent.KVType)
	initLevelMap(0)
}

// RenderTxnConfigEntityStart has ability to track resources for this particular entity
func RenderTxnConfigEntityStart() {
	log.Info("EntityStart: starting ...")
	defer log.Info("EntityStart: finished ...")
	txnLevel++
	initLevelMap(txnLevel)
}

// RenderTxnConfigEntityEnd has ability to track resources for this particular entity
func RenderTxnConfigEntityEnd() {
	log.Info("EntityEnd: starting ...")
	defer log.Info("EntityEnd: finished ...")
	log.Debugf("EntityEnd: before[%d]=%v", txnLevel, beforeMap[txnLevel])
	log.Debugf("EntityEnd: after[%d]=%v", txnLevel, afterMap[txnLevel])
	for key, entry := range afterMap[txnLevel] {
		afterMap[txnLevel-1][key] = entry
	}
	removeLevelMap(txnLevel)
	txnLevel--
	log.Debugf("EntityEnd: before[%d]=%v", txnLevel, beforeMap[txnLevel])
	log.Debugf("EntityEnd: after[%d]=%v", txnLevel, afterMap[txnLevel])
}

func initLevelMap(level int) {
	beforeMap[level] = nil
	kvMap := make(map[string]*vppagent.KVType)
	beforeMap[level] = kvMap
	kvMap = make(map[string]*vppagent.KVType)
	afterMap[level] = kvMap
}

func removeLevelMap(level int) {
	beforeMap[level] = nil
	afterMap[level] = nil
}

// RenderTxnConfigEntityRemoveEntries removes the rendered entries
func RenderTxnConfigEntityRemoveEntries() {

	// by removing the after entries, the end of transaction processing below will
	// be able to cleanup properly
	afterMap[txnLevel] = nil
	afterMap[txnLevel] = make(map[string]*vppagent.KVType)
}

// RenderTxnConfigEnd traverse new and old and updates etcd
func RenderTxnConfigEnd() error {

	// The transaction consists of all the types of objects that are created as
	// a result of processing the nodes, and services.  These are interfaces,
	// bridged domains, and static routes, etc.
	// When a transaction starts, we copy all these and ensure all existing vpp
	// entries are stored in the "before" cache. Then as the configuration is
	// processed, the "new" objects are added to the transaction "after" cache.
	// When the configuration is processed, a post processing of the before and
	// after caches is performed.
	// All entries in the before cache are processed one-by-one.  If the before
	// entry is not in the after cache, then clearly it is not needed and removed
	// from ETCD.  If it is in the after cache, then there are two cases.
	// If the entries match, it is removed from the after cache and ETCD is not
	// "touched".  If the entries are different, it remains in the after cache
	// and awaits transaction end "after" cache post processing.  Once all the
	// before entries have been processed, the after cache is processed.
	// If there are still entries in this cache, they are all written to ETCD.

	// The reason for this transactional approach is as follows: some ETCD entries
	// will be added and updated multiple times during processing of the
	// configuration and there is NO sense continually changing ETCD for an entry
	// until it is fully modified by the configuration processing.  This is why
	// an "after" cache is maintained.  Then post processing will ensure the
	// "final" values of an object are written only ONCE to the ETCD cache.
	// An example of this is bridge domains.  Initially for a host, a default
	// east-west BD is added to the system, then as interfaces are associated
	// with the BD, the BD is updated.  If we tried to continually update the
	// ETCD entry for this BD as we went along, we would improperly set the BD
	// to interim configs until it all the config is performed and the BD reaches
	// its final config.  This would have bad effects on the vpp-agents
	// as they would be forced to react to each BD change and data flow would
	// be affected.  The goal of the reconcile resync is to ONLY make changes
	// if there are new and/or obselete configs.  Existing configs should
	// reamin un-affected by the transaction process.

	defer configMutex.Unlock()

	log.Info("ConfigEnd: starting ...")
	defer log.Info("ConfigEnd: finished ...")

	// traverse the entries in the before cache
	for key := range beforeMap[0] {
		before := beforeMap[0][key]
		after, existsInAfterCache := afterMap[0][key]
		if !existsInAfterCache {
			exists, err := ctlrPlugin.db.Delete(key)          // remove from etcd
			delete(ctlrPlugin.ramConfigCache.VppEntries, key) // remove from sys ram cache
			log.Info("ConfigEnd: remove key from etcd and system cache: ", key, exists, err)
			log.Info("ConfigEnd: remove before entry: ", before)
			delete(beforeMap[0], key)
		} else {
			if before.Equal(after) {
				delete(afterMap[0], key) // ensure dont resend to etcd
			} else {
				log.Info("ConfigEnd: before != after ... before: ", before)
				log.Info("ConfigEnd: before != after ... after: ", after)
			}
		}
	}
	// now post process the after cache, write the remaining entries to etcd
	for key, after := range afterMap[0] {
		log.Info("ConfigEnd: add key to etcd: ", key, after)
		err := after.WriteToEtcd(ctlrPlugin.db)
		if err != nil {
			return err
		}
	}

	transferAfterVppKVEntriesToSystemCache()

	return nil
}

// RenderTxnAddVppEntriesToTxn caches the new entry in the transaction new/after map
func RenderTxnAddVppEntriesToTxn(
	renderedVppAgentEntries map[string]*controller.RenderedVppAgentEntry,
	renderingEntity string,
	vppKVs []*vppagent.KVType) {

	for _, vppKV := range vppKVs {
		RenderTxnAddVppEntryToTxn(renderedVppAgentEntries, renderingEntity, vppKV)
	}

}

// RenderTxnAddVppEntryToTxn caches the new entry in the transaction new/after map
func RenderTxnAddVppEntryToTxn(
	renderedVppAgentEntries map[string]*controller.RenderedVppAgentEntry,
	renderingEntity string,
	vppKV *vppagent.KVType) {

	_, exists := afterMap[txnLevel][vppKV.VppKey]
	if !exists {
		// initialize a new rendered vpp agent entry and append it to the array
		renderedVppEntry := &controller.RenderedVppAgentEntry{
			VppAgentKey:  vppKV.VppKey,
			VppAgentType: vppKV.VppEntryType,
		}
		//renderedVppAgentEntries[renderingEntity] = renderedVppEntry
		renderedVppAgentEntries[vppKV.VppKey] = renderedVppEntry
	}
	// add the new or existing kv entry to the config transaction after map
	afterMap[txnLevel][vppKV.VppKey] = vppKV

	log.Debugf("CfgTxnAddVppEntry: rendered map len: %d, kv:%v, ",
		len(renderedVppAgentEntries), vppKV)
}

// CopyRenderedVppAgentEntriesToBeforeCfgTxn cache the existing set before new keys are rendered
func CopyRenderedVppAgentEntriesToBeforeCfgTxn(
	vppAgentEntries map[string]*controller.RenderedVppAgentEntry) {

	for _, vppAgentEntry := range vppAgentEntries {
		log.Debugf("CopyRendered...BeforeCfgTxn: entry=%v", vppAgentEntry)
		if vppKV, exists := ctlrPlugin.ramConfigCache.VppEntries[vppAgentEntry.VppAgentKey]; !exists {
			log.Warnf("CopyRendered...BeforeCfgTxn: ouch ... missing vpp cache entry: %s",
				vppAgentEntry.VppAgentKey)
			vppKV = &vppagent.KVType{
				VppKey:       vppAgentEntry.VppAgentKey,
				VppEntryType: vppAgentEntry.VppAgentType,
			}
			beforeMap[0][vppAgentEntry.VppAgentKey] = vppKV
		} else {
			beforeMap[0][vppAgentEntry.VppAgentKey] = vppKV
		}
	}
	log.Debugf("CopyRenderedVppAgentEntriesToBeforeCfgTxn: beforeMap: %v", beforeMap)
}

// transferAfterVppKVEntriesToSystemCache updates the system cache with the new vpp agent entries
func transferAfterVppKVEntriesToSystemCache() {

	for _, vppKV := range afterMap[0] {
		ctlrPlugin.ramConfigCache.VppEntries[vppKV.VppKey] = vppKV
	}
}

// // LoadVppAgentEntriesFromState uses key/type from state to lad vpp entries from etcd
// func LoadVppAgentEntriesFromState() error {

// 	log.Debugf("LoadVppAgentEntriesFromState: processing vnf services state: num: %d",
// 		len(s.ramConfigCache.VNFServicesState))
// 	for _, vs := range s.ramConfigCache.VNFServicesState {
// 		log.Debugf("LoadVppAgentEntriesFromState: processing vnf service state: %s", vs.Name)
// 		if err := s.LoadVppAgentEntriesFromRenderedVppAgentEntries(vs.RenderedVppAgentEntries); err != nil {
// 			return err
// 		}
// 	}
// 	log.Debugf("LoadVppAgentEntriesFromState: processing nodes state: num: %d",
// 		len(s.ramConfigCache.NodeState))
// 	for _, ns := range s.ramConfigCache.NodeState {
// 		log.Debugf("LoadVppAgentEntriesFromState: processing node state: %s", ns.Name)
// 		if err := s.LoadVppAgentEntriesFromRenderedVppAgentEntries(ns.RenderedVppAgentEntries); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// // LoadVppAgentEntriesFromRenderedVppAgentEntries load from etcd
// func LoadVppAgentEntriesFromRenderedVppAgentEntries(
// 	vppAgentEntries []*controller.RenderedVppAgentEntry) error {

// 	log.Debugf("LoadVppAgentEntriesFromRenderedVppAgentEntries: num: %d, %v",
// 		len(vppAgentEntries), vppAgentEntries)
// 	for _, vppAgentEntry := range vppAgentEntries {

// 		vppKVEntry := vppagent.NewKVEntry(vppAgentEntry.VppAgentKey, vppAgentEntry.VppAgentType)
// 		found, err := vppKVEntry.ReadFromEtcd(s.db)
// 		if err != nil {
// 			return err
// 		}
// 		if found {
// 			s.ramConfigCache.VppEntries[vppKVEntry.VppKey] = vppKVEntry
// 		}
// 	}

// 	return nil
// }

// // CleanVppAgentEntriesFromEtcd load from etcd
// func CleanVppAgentEntriesFromEtcd() {
// 	log.Debugf("CleanVppAgentEntriesFromEtcd: removing all vpp keys managed by the controller")
// 	for _, kvEntry := range s.ramConfigCache.VppEntries {
// 		s.DeleteFromDatastore(kvEntry.VppKey)
// 	}
// }

// When a rendered high txnLevel entity is processed, it is possible it is part
// of a set of high txnLevel entities.  If a particular entity "fails", then
// we want to cleanup just the currrent high txnLevel entity.  So, as each high
// txnLevel entity is processed and finished, the current set can be moved to the
// new grooup of "sort of" commited entites.  But if the current is ended
// prematurely, then we can "backout" the current set without affecting the
// other "pseudo-commited" set.  Then once we really want to post process the
// final set, we can do our mark and end sweep processing.

// cfgtxn.overallStart, cfg.innerStart, ... nexting of inner starts, pushes
// the kv entites into the next map[1][]kv
