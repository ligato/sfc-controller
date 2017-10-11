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

// The core plugin which drives the SFC Controller.  The core initializes the
// CNP dirver plugin based on command line args.  The database is initialized,
// and a resync is preformed based on what was already in the database.

package core

import (
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/sfc-controller/controller/cnpdriver"
	"github.com/ligato/sfc-controller/controller/extentitydriver"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/namsral/flag"
	"github.com/ligato/cn-infra/flavors/local"
)

var (
	cleanSfcDatastore bool // cli flag - see RegisterFlags
)

// Init is the Go init() function for the sfcCtrlPlugin. It should
// contain the boiler plate initialization code that is executed
// when the sfcCtrlPlugin is loaded into the Agent.
func init() {
	flag.BoolVar(&cleanSfcDatastore, "clean", false,
		"Clean the SFC datastore entries")

}

// ram cache of controller entities indexed by entity name
type SfcControllerCacheType struct {
	EEs  map[string]controller.ExternalEntity
	HEs  map[string]controller.HostEntity
	SFCs map[string]controller.SfcEntity
}

// SfcControllerPluginHandler glues together:
// CNP Driver, ext. entity driver, VNF Driver, ETCD, HTTP, SFC config
type SfcControllerPluginHandler struct {
	Deps

	ramConfigCache        SfcControllerCacheType
	controllerReady       bool
	db                    keyval.ProtoBroker
	ReconcileVppLabelsMap ReconcileVppLabelsMapType
}

// Deps are SfcControllerPluginHandler injected dependencies
type Deps struct {
	Etcd      keyval.KvProtoPlugin //inject
	local.PluginInfraDeps
	CNPDriver cnpdriver.SfcControllerCNPDriverAPI //inject
}

// Init the controller, read the db, reconcile/resync, render config to etcd
func (plugin *SfcControllerPluginHandler) Init() error {

	plugin.db = plugin.Etcd.NewBroker(keyval.Root)

	plugin.initRamCache()

	extentitydriver.SfcExternalEntityDriverInit()

	plugin.Log.Infof("Initializing plugin '%s'", plugin.PluginName)

	// Flag variables registered in init() are ready to use in InitPlugin()
	plugin.logFlags()

	// if -clean then remove the sfc controller datastore, reconcile will remove all extraneous i/f's, BD's etc
	if cleanSfcDatastore {
		plugin.DatastoreClean()
	}

	plugin.Log.Infof("CNP Driver: %s", plugin.CNPDriver.GetName())

	plugin.ReconcileInit()

	plugin.ReconcileStart()

	// If a startup yaml file is provided, then pull it into the ram cache and write it to the database
	// Note that there may already be already an existing database so the policy is that the config yaml
	// file will replace any conflicting entries in the database.
	if cfg, cfgFound, err := plugin.readConfigFromFile(); err != nil {
		plugin.Log.Error("error loading config: ", err)
		return err
	} else if cfgFound {
		if err := plugin.copyYamlConfigToRamCache(cfg); err != nil {
			plugin.Log.Error("error copying config to ram cache: ", err)
			return err
		}

		if err := plugin.validateRamCache(); err != nil {
			plugin.Log.Error("error validating ram cache: ", err)
			return err
		}

		if err := plugin.WriteRamCacheToEtcd(); err != nil {
			plugin.Log.Error("error writing ram config to etcd datastore: ", err)
			return err
		}
	} else { // read config database into ramCache
		if err := plugin.ReadEtcdDatastoreIntoRamCache(); err != nil {
			plugin.Log.Error("error reading etcd config into ram cache: ", err)
			return err
		}
	}

	if err := plugin.renderConfigFromRamCache(); err != nil {
		plugin.Log.Error("error copying config to ram cache: ", err)
		return err
	}

	plugin.ReconcileEnd()

	plugin.controllerReady = true

	return nil
}

// create the ram cache
func (plugin *SfcControllerPluginHandler) initRamCache() {
	plugin.ramConfigCache.EEs = make(map[string]controller.ExternalEntity)
	plugin.ramConfigCache.HEs = make(map[string]controller.HostEntity)
	plugin.ramConfigCache.SFCs = make(map[string]controller.SfcEntity)
}

// Dump the command line flags
func (plugin *SfcControllerPluginHandler) logFlags() {
	plugin.Log.Debugf("LogFlags:")
	plugin.Log.Debugf("\tsfcConfigFile:'%s'", plugin.PluginConfig.GetConfigName())
	plugin.Log.Debugf("\tcleanSfcDatastore:'%s'", cleanSfcDatastore)
}

// perform close down procedures
func (plugin *SfcControllerPluginHandler) Close() error {
	return safeclose.Close(extentitydriver.EEOperationChannel)
}
