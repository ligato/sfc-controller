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
	"github.com/ligato/sfc-controller/plugins/controller/database"
)

// a cache of before and after entries per
// transaction type where a transaction could be a nb_api rest request,
// or a complete system resync which is initiated at startup after the
// DB has been read in, or a loss of comms with etcd has occurred.
// Note: NO nb_api's are accepted during transaction processing as they
// are atomic operations and are mutex protected during each txn.
var beforeMap map[int]map[string]*vppagent.KVType
var afterMap map[int]map[string]*vppagent.KVType
var txnLevel int

var configMutex *sync.Mutex = nil

func ConfigMutexSet(_configMutex *sync.Mutex) {
	configMutex = _configMutex
}

func RenderTxnGetAfterMap(key string) (*vppagent.KVType, bool) {
	vppKey, exists := afterMap[0][key]
	return vppKey, exists
}

// RenderTxnConfigStart initiates state for starting a config transaction
func RenderTxnConfigStart() {

	configMutex.Lock()

	log.Infof("ConfigStart: starting ...len vppEntries=%d", len(ctlrPlugin.ramCache.VppEntries))

	beforeMap = nil
	afterMap = nil
	txnLevel = 0
	beforeMap = make(map[int]map[string]*vppagent.KVType)
	afterMap = make(map[int]map[string]*vppagent.KVType)
	initLevelMap(0)

	for key, vppKV := range ctlrPlugin.ramCache.VppEntries {
		if key != vppKV.VppKey {
			log.Fatal("RenderTxnConfigStart: whoa")
		}
		beforeMap[0][vppKV.VppKey] = vppKV
	}

	log.Infof("ConfigStart: finished ...len before[0]: %d", len(beforeMap[0]))
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

	// disguard these entries for this transaction, RenderTxnConfigEnd will clean
	// up by looking at the saved entries before this transaction took place
	// and remove the old/before entries
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

	defer configMutex.Unlock()

	log.Infof("ConfigEnd: starting ...len before[0]/after[0]=%d/%d",
		len(beforeMap[0]), len(afterMap[0]))


	// traverse the entries in the before cache
	for key := range beforeMap[0] {
		before := beforeMap[0][key]
		after, existsInAfterCache := afterMap[0][key]
		if !existsInAfterCache {
			exists, err := ctlrPlugin.DB.Delete(key)    // remove from etcd
			delete(ctlrPlugin.ramCache.VppEntries, key) // remove from sys ram cache
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
		err := after.WriteToEtcd(ctlrPlugin.DB)
		if err != nil {
			return err
		}
	}

	for _, vppKV := range afterMap[0] {
		ctlrPlugin.ramCache.VppEntries[vppKV.VppKey] = vppKV
	}

	log.Infof("ConfigEnd: finished ...len before[0]/after[0]/vpp_entries: %d/%d/%d",
		len(beforeMap[0]), len(afterMap[0]), len(ctlrPlugin.ramCache.VppEntries))

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

	log.Debugf("CfgTxnAddVppEntry: rendered map len: %d, txnLevel: %d kv:%v, ",
		len(renderedVppAgentEntries), txnLevel, vppKV)
}

// DeleteRenderedVppAgentEntries deletes the existing rendered set
func DeleteRenderedVppAgentEntries(renderedVppAgentEntries map[string]*controller.RenderedVppAgentEntry) {

	log.Debugf("DeleteRenderedVppAgentEntriesFromBeforeCfgTxn: renderedEntities=%v",
		renderedVppAgentEntries)

	for _, vppAgentEntry := range renderedVppAgentEntries {
		log.Debugf("DeleteRenderedVppAgentEntriesFromBeforeCfgTxn: entry=%v", vppAgentEntry)
		delete(ctlrPlugin.ramCache.VppEntries, vppAgentEntry.VppAgentKey)
		database.DeleteFromDatastore(vppAgentEntry.VppAgentKey)
	}
}
