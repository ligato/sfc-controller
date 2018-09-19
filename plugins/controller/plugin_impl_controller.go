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

	"net/http"
	"sync"

	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/idapi"
	"github.com/ligato/sfc-controller/plugins/controller/idapi/ipam"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
	"github.com/namsral/flag"
	"github.com/unrolled/render"
)

// PluginID is plugin identifier (must be unique throughout the system)
const PluginID infra.PluginName = "Controller"

var (
	sfcConfigFile               string // cli flag - see RegisterFlags
	CleanDatastore              bool
	ContivKSREnabled            bool
	BypassModelTypeHttpHandlers bool
	log                         = logrus.DefaultLogger()
	ctlrPlugin                  *Plugin
)

// RegisterFlags add command line flags.
func RegisterFlags() {
	flag.StringVar(&sfcConfigFile, "sfc-config", "",
		"Name of a sfc config (yaml) file to load at startup")
	flag.BoolVar(&CleanDatastore, "clean", false,
		"Clean the controller datastore entries")
	flag.BoolVar(&ContivKSREnabled, "contiv-ksr", false,
		"Interact with contiv ksr to learn k8s config/state")
	flag.BoolVar(&BypassModelTypeHttpHandlers, "bypass-rest-for-model-objects", false,
		"Disable HTTP handling for controller objects")
}

// LogFlags dumps the command line flags
func LogFlags() {
	log.Debugf("LogFlags:")
	log.Debugf("\tsfcConfigFile:'%s'", sfcConfigFile)
	log.Debugf("\tclean:'%v'", CleanDatastore)
	log.Debugf("\tcontiv ksr:'%v'", ContivKSREnabled)
	log.Debugf("\tmodel REST disabled:'%v'", BypassModelTypeHttpHandlers)
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
	InterfaceStates       map[string]*controller.InterfaceStatus                  // key[entity/ifname]
	RenderedEntitesStates map[string]map[string]*controller.RenderedVppAgentEntry // key[modelType/name][vppkey]
	VppEntries            map[string]*vppagent.KVType                             // key[vppkey]
	MacAddrAllocator      *idapi.MacAddrAllocatorType
	MemifIDAllocator      *idapi.MemifAllocatorType
	VrfIDAllocator        *idapi.VrfAllocatorType
	IPAMPoolAllocators    map[string]*ipam.PoolAllocatorType // key[modelType/ipamPoolName]
	NetworkPodToNodeMap   map[string]*NetworkPodToNodeMap    // key[pod]
}

// Plugin contains the controllers information
type Plugin struct {
	Deps

	NetworkNodeMgr              NetworkNodeMgr
	IpamPoolMgr                 IPAMPoolMgr
	SysParametersMgr            SystemParametersMgr
	NetworkServiceMgr           NetworkServiceMgr
	NetworkNodeOverlayMgr       NetworkNodeOverlayMgr
	NetworkPodNodeMapMgr        NetworkPodToNodeMapMgr
	ramCache                    CacheType
	BypassModelTypeHttpHandlers bool // cli flag - see RegisterFlags
	CleanDatastore              bool // cli flag - see RegisterFlags
	ContivKSREnabled            bool // cli flag - see RegisterFlags
	ConfigMutex                 sync.Mutex
	DB                          keyval.ProtoBroker
}

// Deps groups the dependencies of the Plugin.
type Deps struct {
	infra.PluginDeps
	Etcd         *etcd.Plugin
	HTTPHandlers *rest.Plugin
	StatusCheck  *statuscheck.Plugin
}

// Init the controller, read the DB, reconcile/resync, render config to etcd
func (s *Plugin) Init() error {

	ctlrPlugin = s

	s.CleanDatastore = CleanDatastore
	s.ContivKSREnabled = ContivKSREnabled
	s.BypassModelTypeHttpHandlers = BypassModelTypeHttpHandlers

	log.Infof("Init: %s enter ...", PluginID)
	defer log.Infof("Init: %s exit ", PluginID)

	// Flag variables registered in init() are ready to use in InitPlugin()
	LogFlags()

	// Register providing status reports (push mode)
	s.StatusCheck.Register(PluginID, nil)
	s.StatusCheck.ReportStateChange(PluginID, statuscheck.Init, nil)

	ConfigMutexSet(&s.ConfigMutex)

	s.DB = s.Etcd.NewBroker(keyval.Root)
	database.InitDatabase(s.DB)

	s.RegisterModelTypeManagers()

	s.InitRAMCache()

	s.initMgrs()

	if err := s.PostProcessLoadedDatastore(); err != nil {
		os.Exit(1)
	}

	// the DB has been loaded and vpp entries known so now we can clean up the
	// DB and remove the vpp agent entries that the controller has managed/created
	if s.CleanDatastore {
		database.CleanDatastore(controller.SfcControllerConfigPrefix())
		s.CleanVppAgentEntriesFromEtcd()
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

	log.Infof("Dumping: controller cache: %v", s.ramCache)
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
	s.InitSystemHTTPHandler()

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
	
	s.afterInitMgrs()

	if s.ContivKSREnabled {
		go ctlrPlugin.NetworkPodNodeMapMgr.RunContivKSRNetworkPodToNodeMappingWatcher()
	}

	go s.ProcessOperationalMessages()
	s.AddOperationMsgToQueue("", OperationalMsgOpCodeRender, nil)

	s.StatusCheck.ReportStateChange(PluginID, statuscheck.OK, nil)

	return nil
}

// RenderAll calls only node and service as the rest are resources used by these
func (s *Plugin) RenderAll() {
	// nodes and services ONLY render VPP entries ... the rest are resources used by the renderers
	ctlrPlugin.NetworkNodeMgr.RenderAll()
	ctlrPlugin.NetworkServiceMgr.RenderAll()
}

// InitRAMCache creates the ram cache
func (s *Plugin) InitRAMCache() {

	s.ramCache.IPAMPoolAllocators = nil
	s.ramCache.IPAMPoolAllocators = make(map[string]*ipam.PoolAllocatorType)

	s.ramCache.VppEntries = nil
	s.ramCache.VppEntries = make(map[string]*vppagent.KVType)

	s.ramCache.RenderedEntitesStates = nil
	s.ramCache.RenderedEntitesStates = make(map[string]map[string]*controller.RenderedVppAgentEntry)

	s.ramCache.MacAddrAllocator = nil
	s.ramCache.MacAddrAllocator = idapi.NewMacAddrAllocator()

	s.ramCache.MemifIDAllocator = nil
	s.ramCache.MemifIDAllocator = idapi.NewMemifAllocator()

	s.ramCache.VrfIDAllocator = nil
	s.ramCache.VrfIDAllocator = idapi.NewVrfAllocator()

	s.ramCache.InterfaceStates = nil
	s.ramCache.InterfaceStates = make(map[string]*controller.InterfaceStatus)

	for _, entry := range RegisteredManagers {
		log.Infof("InitRAMCache: %s ...", entry.modelTypeName)
		entry.mgr.InitRAMCache()
	}

	// ksr state updates are stored here
	s.ramCache.NetworkPodToNodeMap = make(map[string]*NetworkPodToNodeMap, 0)
}

// Close performs close down procedures
func (s *Plugin) Close() error {
	return nil
}

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (s *Plugin) InitSystemHTTPHandler() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET %s", controller.SfcControllerPrefix())
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(controller.SfcControllerPrefix(), httpSystemGetAllYamlHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller
func httpSystemGetAllYamlHandler(formatter *render.Render) http.HandlerFunc {

	// This routine is a debug dump routine that dumps the config/state of the entire system in yaml format

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("httpSystemGetAllYamlHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			yaml, err := ctlrPlugin.SfcSystemCacheToYaml()
			if err != nil {
				formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})

			}
			formatter.Data(w, http.StatusOK, yaml)
		}
	}
}

// PostProcessLoadedDatastore uses key/type from state to load vpp entries from etcd
func (s *Plugin) PostProcessLoadedDatastore() error {

	log.Debugf("PostProcessLoadedDatastore: processing ipam pool state: num: %d",
		len(s.IpamPoolMgr.ipamPoolCache))
	for _, ipamPool := range s.IpamPoolMgr.ipamPoolCache {
		if ipamPool.Status == nil {
			ipamPool.Status = &controller.IPAMPoolStatus{
				Addresses: make(map[string]string, 0),
			}
		} else {
			if ipamPool.Status.Addresses == nil {
				ipamPool.Status.Addresses = make(map[string]string, 0)
			}
		}
	}
	ctlrPlugin.IpamPoolMgr.EntityCreate("", controller.IPAMPoolScopeSystem)

	log.Debugf("PostProcessLoadedDatastore: processing nodes state: num: %d",
		len(s.NetworkNodeMgr.networkNodeCache))
	for _, nn := range s.NetworkNodeMgr.networkNodeCache {

		ctlrPlugin.IpamPoolMgr.EntityCreate(nn.Metadata.Name, controller.IPAMPoolScopeNode)

		if nn.Status != nil && len(nn.Status.RenderedVppAgentEntries) != 0 {

			log.Debugf("PostProcessLoadedDatastore: processing node state: %s", nn.Metadata.Name)
			if err := s.LoadVppAgentEntriesFromRenderedVppAgentEntries(
				ModelTypeNetworkNode+"/"+nn.Metadata.Name,
				nn.Status.RenderedVppAgentEntries); err != nil {
				return err
			}
		}
		if nn.Status != nil && len(nn.Status.Interfaces) != 0 {

			log.Debugf("PostProcessLoadedDatastore: processing node interfaces: %s", nn.Metadata.Name)

			for _, ifStatus := range nn.Status.Interfaces {

				log.Debugf("PostProcessLoadedDatastore: ifStatus: %v", ifStatus)

				ctlrPlugin.ramCache.InterfaceStates[ifStatus.Name] = ifStatus
				if ifStatus.MemifID > ctlrPlugin.ramCache.MemifIDAllocator.MemifID {
					ctlrPlugin.ramCache.MemifIDAllocator.MemifID = ifStatus.MemifID
				}
				if ifStatus.MacAddrID > ctlrPlugin.ramCache.MacAddrAllocator.MacAddrID {
					ctlrPlugin.ramCache.MacAddrAllocator.MacAddrID = ifStatus.MacAddrID
				}
				UpdateRamCacheAllocatorsForInterfaceStatus(ifStatus, nn.Metadata.Name)
			}
		}
	}

	log.Debugf("PostProcessLoadedDatastore: processing network services state: num: %d",
		len(s.NetworkServiceMgr.networkServiceCache))
	for _, ns := range s.NetworkServiceMgr.networkServiceCache {

		ctlrPlugin.IpamPoolMgr.EntityCreate(ns.Metadata.Name, controller.IPAMPoolScopeNetworkService)

		if ns.Status != nil && len(ns.Status.RenderedVppAgentEntries) != 0 {

			log.Debugf("PostProcessLoadedDatastore: processing network service state: %s", ns.Metadata.Name)
			if err := s.LoadVppAgentEntriesFromRenderedVppAgentEntries(
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				ns.Status.RenderedVppAgentEntries); err != nil {
				return err
			}
		}
		if ns.Status != nil && len(ns.Status.Interfaces) != 0 {

			log.Debugf("PostProcessLoadedDatastore: processing netwrok service interfaces: %s", ns.Metadata.Name)

			for _, ifStatus := range ns.Status.Interfaces {

				log.Debugf("PostProcessLoadedDatastore: ifStatus: %v", ifStatus)

				ctlrPlugin.ramCache.InterfaceStates[ifStatus.Name] = ifStatus
				if ifStatus.MemifID > ctlrPlugin.ramCache.MemifIDAllocator.MemifID {
					ctlrPlugin.ramCache.MemifIDAllocator.MemifID = ifStatus.MemifID
				}
				if ifStatus.MacAddrID > ctlrPlugin.ramCache.MacAddrAllocator.MacAddrID {
					ctlrPlugin.ramCache.MacAddrAllocator.MacAddrID = ifStatus.MacAddrID
				}
				UpdateRamCacheAllocatorsForInterfaceStatus(ifStatus, ns.Metadata.Name)
			}
		}

	}

	return nil
}

// LoadVppAgentEntriesFromRenderedVppAgentEntries load from etcd
func (s *Plugin) LoadVppAgentEntriesFromRenderedVppAgentEntries(
	entityName string,
	vppAgentEntries map[string]*controller.RenderedVppAgentEntry) error {

	log.Debugf("LoadVppAgentEntriesFromRenderedVppAgentEntries: entity: %s, num: %d, %v",
		entityName, len(vppAgentEntries), vppAgentEntries)
	for _, vppAgentEntry := range vppAgentEntries {

		vppKVEntry := vppagent.NewKVEntry(vppAgentEntry.VppAgentKey, vppAgentEntry.VppAgentType)
		found, err := vppKVEntry.ReadFromEtcd(s.DB)
		if err != nil {
			return err
		}
		if found {
			// add ref to VppEntries indexed by the vppKey
			s.ramCache.VppEntries[vppKVEntry.VppKey] = vppKVEntry

			// add ref to the rendering entity
			if renderedEntities, exists := s.ramCache.RenderedEntitesStates[entityName]; !exists {
				renderedEntities = make(map[string]*controller.RenderedVppAgentEntry, 0)
				s.ramCache.RenderedEntitesStates[entityName] = renderedEntities

			}
			s.ramCache.RenderedEntitesStates[entityName][vppAgentEntry.VppAgentKey] = vppAgentEntry
		}
	}

	return nil
}

// CleanVppAgentEntriesFromEtcd load from etcd
func (s *Plugin) CleanVppAgentEntriesFromEtcd() {
	log.Debugf("CleanVppAgentEntriesFromEtcd: removing all vpp keys managed by the controller")
	for _, kvEntry := range s.ramCache.VppEntries {
		database.DeleteFromDatastore(kvEntry.VppKey)
	}
}
