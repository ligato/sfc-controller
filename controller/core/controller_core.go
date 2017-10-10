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
	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/sfc-controller/controller/cnpdriver"
	"github.com/ligato/sfc-controller/controller/extentitydriver"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/namsral/flag"
)

// Plugin identifier (must be unique throughout the system)
const PluginID core.PluginName = "SfcController"

var (
	sfcConfigFile     string // cli flag - see RegisterFlags
	cleanSfcDatastore bool   // cli flag - see RegisterFlags
	log               = logroot.StandardLogger()
)

// Add command line flags here.
func RegisterFlags() {
	flag.StringVar(&sfcConfigFile, "sfc-config", "",
		"Name of a sfc config (yaml) file to load at startup")
	flag.BoolVar(&cleanSfcDatastore, "clean", false,
		"Clean the SFC datastore entries")
}

// Dump the command line flags
func LogFlags() {
	log.Debugf("LogFlags:")
	log.Debugf("\tsfcConfigFile:'%s'", sfcConfigFile)
}

// Init is the Go init() function for the sfcCtrlPlugin. It should
// contain the boiler plate initialization code that is executed
// when the sfcCtrlPlugin is loaded into the Agent.
func init() {
	// Logger must be initialized for each sfcCtrlPlugin individually.
	log.SetLevel(logging.DebugLevel)
	//TODO with Lukas pluginapi.RegisterLogger(PluginID, log.StandardLogger())

	RegisterFlags()

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

	yamlConfig            *YamlConfig
	ramConfigCache        SfcControllerCacheType
	controllerReady       bool
	db                    keyval.ProtoBroker
	ReconcileVppLabelsMap ReconcileVppLabelsMapType
}

// Deps are SfcControllerPluginHandler injected dependencies
type Deps struct {
	Etcd      *etcdv3.Plugin                      //inject
	HTTPmux   *rest.Plugin                        //inject
	CNPDriver cnpdriver.SfcControllerCNPDriverAPI //inject
}

// Init the controller, read the db, reconcile/resync, render config to etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) Init() error {

	sfcCtrlPlugin.db = sfcCtrlPlugin.Etcd.NewBroker(keyval.Root)

	sfcCtrlPlugin.initRamCache()

	extentitydriver.SfcExternalEntityDriverInit()

	var err error

	log.Infof("Initializing sfcCtrlPlugin '%s'", PluginID)

	// Flag variables registered in init() are ready to use in InitPlugin()
	LogFlags()

	// if -clean then remove the sfc controller datastore, reconcile will remove all extraneous i/f's, BD's etc
	if cleanSfcDatastore {
		sfcCtrlPlugin.DatastoreClean()
	}

	// register northbound controller API's
	sfcCtrlPlugin.InitHttpHandlers()

	log.Infof("CNP Driver: %s", sfcCtrlPlugin.CNPDriver.GetName())

	sfcCtrlPlugin.ReconcileInit()

	sfcCtrlPlugin.ReconcileStart()

	// read config database into ramCache
	if err := sfcCtrlPlugin.ReadEtcdDatastoreIntoRamCache(); err != nil {
		log.Error("error reading etcd config into ram cache: ", err)
		return err
	}

	// If a startup yaml file is provided, then pull it into the ram cache and write it to the database
	// Note that there may already be already an existing database so the policy is that the config yaml
	// file will replace any conflicting entries in the database.
	if sfcConfigFile != "" {

		if err := sfcCtrlPlugin.readConfigFromFile(sfcConfigFile); err != nil {
			log.Error("error loading config: ", err)
			return err
		}

		if err := sfcCtrlPlugin.copyYamlConfigToRamCache(); err != nil {
			log.Error("error copying config to ram cache: ", err)
			return err
		}

		if err := sfcCtrlPlugin.validateRamCache(); err != nil {
			log.Error("error validating ram cache: ", err)
			return err
		}

		if err := sfcCtrlPlugin.WriteRamCacheToEtcd(); err != nil {
			log.Error("error writing ram config to etcd datastore: ", err)
			return err
		}

	}

	if err = sfcCtrlPlugin.renderConfigFromRamCache(); err != nil {
		log.Error("error copying config to ram cache: ", err)
		return err
	}

	sfcCtrlPlugin.ReconcileEnd()

	sfcCtrlPlugin.controllerReady = true

	return nil
}

// create the ram cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) initRamCache() {
	sfcCtrlPlugin.ramConfigCache.EEs = make(map[string]controller.ExternalEntity)
	sfcCtrlPlugin.ramConfigCache.HEs = make(map[string]controller.HostEntity)
	sfcCtrlPlugin.ramConfigCache.SFCs = make(map[string]controller.SfcEntity)
}

// perform close down procedures
func (sfcCtrlPlugin *SfcControllerPluginHandler) Close() error {
	return safeclose.Close(extentitydriver.EEOperationChannel)
}
