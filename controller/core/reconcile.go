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

// Package core: The reconcile process "resyncs" the configuration stored in
// the database, and the provided config yaml file (if there is one).  It is
// possible to start with an empty database if desired by providing the -clean
// command line arg. The resync procedure loads the configuration into a
// reconcile "before" resync data structure then applies the configuration
// which causes new ETCD entries to be made, however, these will be put into
// the "after" resync data structure during reconcile.  Once all config has
// been processed, the before and after data structures are post-processed.
// This module drives the resync for the plugins.
package core

// ReconcileStart: init the reconcile procedure for all plugins
func (plugin *SfcControllerPluginHandler) ReconcileStart() error {

	plugin.Log.Info("ReconcileStart: enter ...")
	defer plugin.Log.Info("ReconcileStart: exit ...")

	plugin.CNPDriver.ReconcileStart()

	return nil
}

// ReconcileEnd: perform post processing of the reconcile procedure
func (plugin *SfcControllerPluginHandler) ReconcileEnd() error {

	plugin.Log.Info("ReconcileEnd: begin ...")
	defer plugin.Log.Info("ReconcileEnd: exit ...")

	plugin.CNPDriver.ReconcileEnd()

	return nil
}
