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

// The reconcile process "resyncs" the configuration stored in the database, and
// the provided config yaml file (if there is one).  It is possible to start
// with an empty datbase if desired by providing the -clean command line arg.
// The resync procedure loads the configuration into a reconcile "before"
// resync data structure then applies the configuration which causes new ETCD
// entries to be made, however, these will be put into the "after" resync data
// structure during reconcile.  Once all config has been processed, the before
// and after data structures are post-processed.
//
// This module drives the reconile for the plugins.
package core

import (
	"github.com/ligato/sfc-controller/controller/utils"
)

// track all the vpp agents in etcd
type ReconcileVppLabelsMapType map[string]struct{}

// initialize the map of all etcd vpp/agent labels
func (sfcCtrlPlugin *SfcControllerPluginHandler) ReconcileInit() error {

	sfcCtrlPlugin.ReconcileVppLabelsMap = make(ReconcileVppLabelsMapType)

	return nil
}

// init the reconcile procedure for all plguins
func (sfcCtrlPlugin *SfcControllerPluginHandler) ReconcileStart() error {

	log.Info("ReconcileStart: enter ...")
	defer log.Info("ReconcileStart: exit ...")

	for k, _ := range sfcCtrlPlugin.ReconcileVppLabelsMap {
		delete(sfcCtrlPlugin.ReconcileVppLabelsMap, k)
	}
	sfcCtrlPlugin.ReconcileLoadAllVppLabels()

	sfcCtrlPlugin.cnpDriverPlugin.ReconcileStart(sfcCtrlPlugin.ReconcileVppLabelsMap)

	return nil
}

// perform post processing of the reconcile procedure
func (sfcCtrlPlugin *SfcControllerPluginHandler) ReconcileEnd() error {

	log.Info("ReconcileEnd: begin ...")
	defer log.Info("ReconcileEnd: exit ...")

	sfcCtrlPlugin.cnpDriverPlugin.ReconcileEnd()

	return nil
}

// pull all vpp lavels from the etcd datastore
func (sfcCtrlPlugin *SfcControllerPluginHandler) ReconcileLoadAllVppLabels() {

	log.Info("ReconcileLoadAllVppLabels: begin ...")
	defer log.Info("ReconcileLoadAllVppLabels: exit ...")

	keyIter, err := sfcCtrlPlugin.db.ListKeys(utils.GetVppAgentPrefix())
	if err == nil {

		for {
			if key, _, done := keyIter.GetNext(); !done {
				label := utils.GetVppEtcdlabel(key)
				_, exists := sfcCtrlPlugin.ReconcileVppLabelsMap[label]
				if !exists {
					log.Info("ReconcileLoadAllVppLabels: adding label to reconcile label map: ", label)
					sfcCtrlPlugin.ReconcileVppLabelsMap[label] = struct{}{}
				}
				continue
			}
			break
		}

	}
}
