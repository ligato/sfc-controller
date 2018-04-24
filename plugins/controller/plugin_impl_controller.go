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

//go:generate protoc --proto_path=model --gogo_out=model model/controller.proto

package controller

import (
	"os"

	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/idapi"
	"github.com/ligato/sfc-controller/plugins/controller/idapi/ipam"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
	"github.com/namsral/flag"
)

// PluginID is plugin identifier (must be unique throughout the system)
const PluginID core.PluginName = "SfcController"

var (
	sfcConfigFile     string // cli flag - see RegisterFlags
	cleanSfcDatastore bool   // cli flag - see RegisterFlags
	contivKSREnabled  bool   // cli flag - see RegisterFlags
	log               = logrus.DefaultLogger()
	ctlrPlugin 		*Plugin

)

// RegisterFlags add command line flags.
func RegisterFlags() {
	flag.StringVar(&sfcConfigFile, "sfc-config", "",
		"Name of a sfc config (yaml) file to load at startup")
	flag.BoolVar(&cleanSfcDatastore, "clean", false,
		"Clean the SFC datastore entries")
	flag.BoolVar(&contivKSREnabled, "contiv-ksr", false,
		"Interact with contiv ksr to learn k8s config/state")
}

// LogFlags dumps the command line flags
func LogFlags() {
	log.Debugf("LogFlags:")
	log.Debugf("\tsfcConfigFile:'%s'", sfcConfigFile)
	log.Debugf("\tclean:'%v'", cleanSfcDatastore)
	log.Debugf("\tcontiv ksr:'%v'", contivKSREnabled)
}

func init() {
	// Logger must be initialized for each s individually.
	//log.SetLevel(logging.DebugLevel)
	log.SetLevel(logging.InfoLevel)

	RegisterFlags()
}

// CacheType is ram cache of controller entities
type CacheType struct {
	// state
	InterfaceStates    map[string]*controller.InterfaceStatus
	VppEntries         map[string]*vppagent.KVType
	MacAddrAllocator   *idapi.MacAddrAllocatorType
	MemifIDAllocator   *idapi.MemifAllocatorType
	IPAMPoolAllocators map[string]*ipam.PoolAllocatorType
}

// Plugin contains the controllers information
type Plugin struct {
	Etcd                  *etcdv3.Plugin
	HTTPmux               *rest.Plugin
	*local.FlavorLocal
	NetworkNodeMgr        NetworkNodeMgr
	IpamPoolMgr           IPAMPoolMgr
	SysParametersMgr      SystemParametersMgr
	NetworkServiceMgr     NetworkServiceMgr
	NetworkNodeOverlayMgr NetworkNodeOverlayMgr
	NetworkPodNodeMapMgr  NetworkPodToNodeMapMgr
	ramConfigCache        CacheType
	db                    keyval.ProtoBroker
}


// Init the controller, read the db, reconcile/resync, render config to etcd
func (s *Plugin) Init() error {

	ctlrPlugin = s

	log.Infof("Init: %s enter ...", PluginID)
	defer log.Infof("Init: %s exit ", PluginID)

	// Flag variables registered in init() are ready to use in InitPlugin()
	LogFlags()

	// Register providing status reports (push mode)
	s.StatusCheck.Register(PluginID, nil)
	s.StatusCheck.ReportStateChange(PluginID, statuscheck.Init, nil)

	s.db = s.Etcd.NewBroker(keyval.Root)
	database.InitDatabase(s.db)

	s.RegisterModelTypeManagers()

	s.initMgrs()

	//if err := s.LoadVppAgentEntriesFromState(); err != nil {
	//	os.Exit(1)
	//}

	// the db has been loaded and vpp entries kown so now we can clean the
	// db and the vpp agent that the controller has managed/created
	if cleanSfcDatastore {
		database.CleanDatastore(controller.SfcControllerConfigPrefix())
		//s.CleanVppAgentEntriesFromEtcd()
		s.InitRAMCache()
	}

	// If a startup yaml file is provided, then pull it into the ram cache and write it to the database
	// Note that there may already be an existing database so the policy is that the config yaml
	// file will replace any conflicting entries in the database.
	if sfcConfigFile != "" {

		if yamlConfig, err := s.SfcConfigYamlReadFromFile(sfcConfigFile); err != nil {
			log.Error("error loading config: ", err)
			os.Exit(1)
		} else if err := s.SfcConfigYamlProcessConfig(yamlConfig); err != nil {
			log.Error("error copying config: ", err)
			os.Exit(1)
		}
	}

	log.Infof("Dumping: controller cache: %v", s.ramConfigCache)
	for _, entry := range RegisteredManagers {
		log.Infof("Init: dumping %s ...", entry.modelTypeName)
		entry.mgr.DumpCache()
	}

	return nil
}

func (s *Plugin) initMgrs() {
	for _, entry := range RegisteredManagers {
		log.Infof("initMgrs: initing %s ...", entry.modelTypeName)
		entry.mgr.Init()
	}
}

func (s *Plugin) afterInitMgrs() {
	for _, entry := range RegisteredManagers {
		log.Infof("afterInitMgrs: after initing %s ...", entry.modelTypeName)
		entry.mgr.AfterInit()
	}
}

// AfterInit is called after all plugin are init-ed
func (s *Plugin) AfterInit() error {
	log.Info("AfterInit:", PluginID)

	// at this point, plugins are all loaded, all is read in from the database
	// so render the config ... note: resync will ensure etcd is not written to
	// unnecessarily

	RenderTxnConfigStart()
	s.RenderAll()
	RenderTxnConfigEnd()

	s.afterInitMgrs()

	if contivKSREnabled {
		go s.RunContivKSRVnfToNodeMappingWatcher()
	}

	s.StatusCheck.ReportStateChange(PluginID, statuscheck.OK, nil)

	return nil
}

// RenderAll calls each managers causing them to render
func (s *Plugin) RenderAll() {
	for _, entry := range RegisteredManagers {
		log.Infof("RenderAll: initial rendering %s ...", entry.modelTypeName)
		entry.mgr.RenderAll()
	}
}

// InitRAMCache creates the ram cache
func (s *Plugin) InitRAMCache() {

	for _, entry := range RegisteredManagers {
		log.Infof("InitRAMCache: %s ...", entry.modelTypeName)
		entry.mgr.InitRAMCache()
	}

	s.ramConfigCache.IPAMPoolAllocators = nil
	s.ramConfigCache.IPAMPoolAllocators = make(map[string]*ipam.PoolAllocatorType)

	//s.ramConfigCache.NetworkNodeOverlayVniAllocators = nil
	//s.ramConfigCache.VNFServiceMeshVniAllocators = make(map[string]*idapi.VxlanVniAllocatorType)
	//
	//s.ramConfigCache.VNFServiceMeshVxLanAddresses = nil
	//s.ramConfigCache.VNFServiceMeshVxLanAddresses = make(map[string]string)


	s.ramConfigCache.VppEntries = nil
	s.ramConfigCache.VppEntries = make(map[string]*vppagent.KVType)

	s.ramConfigCache.MacAddrAllocator = nil
	s.ramConfigCache.MacAddrAllocator = idapi.NewMacAddrAllocator()

	s.ramConfigCache.MemifIDAllocator = nil
	s.ramConfigCache.MemifIDAllocator = idapi.NewMemifAllocator()
}

// Close performs close down procedures
func (s *Plugin) Close() error {
	return nil
}
