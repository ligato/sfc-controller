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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
)

type NetworkPodToNodeMapMgr struct {
	networkPodNodeCache map[string]*NetworkPodToNodeMap
}

func (mgr *NetworkPodToNodeMapMgr) ToArray() []*NetworkPodToNodeMap {
	var array []*NetworkPodToNodeMap
	for _, p2n := range mgr.networkPodNodeCache {
		array = append(array, p2n)
	}
	return array
}

func (mgr *NetworkPodToNodeMapMgr) Init() {
	mgr.InitRAMCache()
	mgr.LoadAllFromDatastoreIntoCache()
}

func (mgr *NetworkPodToNodeMapMgr) AfterInit() {
	go mgr.InitAndRunWatcher()
	if !ctlrPlugin.BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// NetworkPodToNodeMap holds all network node specific info
type NetworkPodToNodeMap struct {
	controller.NetworkPodToNodeMap
}

// InitRAMCache create a map for all the entities
func (mgr *NetworkPodToNodeMapMgr) InitRAMCache() {
	mgr.networkPodNodeCache = nil // delete old cache for re-init
	mgr.networkPodNodeCache = make(map[string]*NetworkPodToNodeMap)
}

// DumpCache logs all the entries in the map
func (mgr *NetworkPodToNodeMapMgr) DumpCache() {
	for _, p2n := range mgr.networkPodNodeCache {
		p2n.dumpToLog()
	}
}

func (p2n *NetworkPodToNodeMap) dumpToLog() {
	log.Infof("NetworkPodToNodeMap[%s] = %v", p2n.Pod, p2n)
}

// ConfigEqual return true if the entities are equal
func (p2n *NetworkPodToNodeMap) ConfigEqual(_p2n *NetworkPodToNodeMap) bool {
	if p2n.String() != _p2n.String() {
		return false
	}
	return true
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationCU(data interface{}) error {

	p2n := data.(*NetworkPodToNodeMap)

	if err := p2n.validate(); err != nil {
		return err
	}

	mgr.networkPodNodeCache[p2n.Pod] = p2n
	ctlrPlugin.ramCache.NetworkPodToNodeMap[p2n.Pod] = p2n

	if err := p2n.writeToDatastore(); err != nil {
		return err
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationR(name string) (*NetworkPodToNodeMap, bool) {
	p2n, exists := mgr.networkPodNodeCache[name]
	return p2n, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationD(data interface{}) error {

	name := data.(string)

	delete(mgr.networkPodNodeCache, name)

	// remove from the database
	database.DeleteFromDatastore(mgr.NameKey(name))

	return nil
}

// HandleCRUDOperationGetAll returns the map
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationGetAll() map[string]*NetworkPodToNodeMap {
	return mgr.networkPodNodeCache
}

func (p2n *NetworkPodToNodeMap) writeToDatastore() error {
	key := ctlrPlugin.NetworkPodNodeMapMgr.NameKey(p2n.Pod)
	return database.WriteToDatastore(key, p2n)
}

func (p2n *NetworkPodToNodeMap) deleteFromDatastore() {
	key := ctlrPlugin.NetworkPodNodeMapMgr.NameKey(p2n.Pod)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *NetworkPodToNodeMapMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastore: ...")
	mgr.loadAllFromDatastore(mgr.KeyPrefix(), ctlrPlugin.ramCache.NetworkPodToNodeMap)
	return mgr.loadAllFromDatastore(mgr.KeyPrefix(), mgr.networkPodNodeCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *NetworkPodToNodeMapMgr) loadAllFromDatastore(prefix string, p2nMap map[string]*NetworkPodToNodeMap) error {
	var p2n *NetworkPodToNodeMap
	return database.ReadIterate(prefix,
		func() proto.Message {
			p2n = &NetworkPodToNodeMap{}
			return p2n
		},
		func(data proto.Message) {
			p2nMap[p2n.Pod] = p2n
			//log.Debugf("loadAllFromDatastore: p2n=%v", p2n)
		})
}

const (
	networkPodName = "networkPodName"
)

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (mgr *NetworkPodToNodeMapMgr) InitHTTPHandlers() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET/POST %s", mgr.KeyPrefix())
	url := fmt.Sprintf(mgr.KeyPrefix()+"{%s}", networkPodName)
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(url, networkPodNodeMapHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(mgr.GetAllURL(), networkPodNodeMapGetAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/vnf-to-node-map/<networkPodName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/vnf-to-node-map/<networkPodName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/vnf-to-node-map/<networkPodName>
func networkPodNodeMapHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkPodNodeMapHandler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if p2n, exists := ctlrPlugin.NetworkPodNodeMapMgr.HandleCRUDOperationR(vars[networkPodName]); exists {
				formatter.JSON(w, http.StatusOK, p2n)
			} else {
				formatter.JSON(w, http.StatusNotFound, "not found: "+vars[networkPodName])
			}
		case "POST":
			networkPodNodeMapProcessPost(formatter, w, req)
		case "DELETE":
			networkPodNodeMapProcessDelete(formatter, w, req)
		}
	}
}

func networkPodNodeMapProcessPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var p2n NetworkPodToNodeMap
	err = json.Unmarshal(body, &p2n)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)
	if vars[networkPodName] != p2n.Pod {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if existing, exists := ctlrPlugin.NetworkPodNodeMapMgr.HandleCRUDOperationR(vars[networkPodName]); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if existing.ConfigEqual(&p2n) {
			log.Debugf("processPost: config equal no further processing required")
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
		log.Debugf("processPost: old: %v", existing)
		log.Debugf("processPost: new: %v", p2n)
	}

	if err := p2n.validate(); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkPodNodeMap, OperationalMsgOpCodeCreateUpdate, &p2n)

	formatter.JSON(w, http.StatusOK, "OK")
}

func networkPodNodeMapProcessDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkPodNodeMap,
		OperationalMsgOpCodeDelete, vars[networkPodName])

	formatter.JSON(w, http.StatusOK, "OK")
}

// networkPodNodeMapGetAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/vnf-to-node-map
func networkPodNodeMapGetAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkPodNodeMapGetAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]NetworkPodToNodeMap, 0)
			for _, n := range ctlrPlugin.NetworkPodNodeMapMgr.HandleCRUDOperationGetAll() {
				array = append(array, *n)
			}
			formatter.JSON(w, http.StatusOK, array)
		}
	}
}

// KeyPrefix provides sfc controller's node key prefix
func (mgr *NetworkPodToNodeMapMgr) KeyPrefix() string {
	return controller.SfcControllerConfigPrefix() + "network-pod-to-node-map/"
}

// GetAllURL allows all to be retrieved
func (mgr *NetworkPodToNodeMapMgr) GetAllURL() string {
	return controller.SfcControllerConfigPrefix() + "network-pod-to-node-map"
}

// NameKey provides sfc controller's vnf name key prefix
func (mgr *NetworkPodToNodeMapMgr) NameKey(name string) string {
	return mgr.KeyPrefix() + name
}

// ContivKsrNetworkPodToNodePrefix: ContivKSR writes k8s updates using this prefix
func (mgr *NetworkPodToNodeMapMgr) ContivKsrNetworkPodToNodePrefix() string {
	return controller.SfcControllerContivKSRPrefix() + controller.SfcControllerStatusPrefix() +
		"network-pod-to-node-map/"
}

func (p2n *NetworkPodToNodeMap) validate() error {
	log.Debugf("Validating NetworkPodToNodeMap: %v ...", p2n)
	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *NetworkPodToNodeMapMgr) InitAndRunWatcher() {

	log.Info("NetworkPodToNodeMapWatcher: enter ...")
	defer log.Info("NetworkPodToNodeMapWatcher: exit ...")

	respChan := make(chan datasync.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("NetworkPodToNodeMapWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("NetworkPodToNodeMapWatcher: watching the key: %s", mgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Delete:
				log.Infof("NetworkPodToNodeMapWatcher: deleting key: %s ", resp.GetKey())
				ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkPodNodeMap, OperationalMsgOpCodeDelete, resp.GetKey())
			}
		}
	}
}

//SyncNetworkPodToNodeMap ensures the contiv, local config cache, and ram cache are in sync.  The config cache
// is populated by configuration (crd, rest, yaml), and also the ram cache is updated with the config info.  The
// ram cache is also modified (adds, deletes), when the contiv-ksr discovers where pods are hosted.
// The ram cache MUST be up to date as it is used by the renderer to know where pods are hosted so it can create
// the correct connectivity configuration in etcd.
func (mgr *NetworkPodToNodeMapMgr) SyncNetworkPodToNodeMap() bool {

	// load the contiv ksr pod to node map
	contivNetworkPodToNodeMapMap := make(map[string]*NetworkPodToNodeMap)
	mgr.loadAllFromDatastore(mgr.ContivKsrNetworkPodToNodePrefix(), contivNetworkPodToNodeMapMap)

    // copy the ram cache into a temp
	tempNetworkPodToNodeMapMap := make(map[string]*NetworkPodToNodeMap)
	for pod, entry := range ctlrPlugin.ramCache.NetworkPodToNodeMap {
		tempNetworkPodToNodeMapMap[pod] = entry
	}

	// clear the ram map, put the config cache into it, then put the contiv into it
	ctlrPlugin.ramCache.NetworkPodToNodeMap = nil
	ctlrPlugin.ramCache.NetworkPodToNodeMap = make(map[string]*NetworkPodToNodeMap)

	for pod, entry := range ctlrPlugin.NetworkPodNodeMapMgr.networkPodNodeCache {
		ctlrPlugin.ramCache.NetworkPodToNodeMap[pod] = entry
	}
	for pod, entry := range contivNetworkPodToNodeMapMap {
		ctlrPlugin.ramCache.NetworkPodToNodeMap[pod] = entry
	}

    // the ram cache has now the most accurate info, compare to temp and if different, we should render
	renderingRequired := false
	if len(tempNetworkPodToNodeMapMap) != len(ctlrPlugin.ramCache.NetworkPodToNodeMap) {
		renderingRequired = true
	} else {
		for _, entry := range tempNetworkPodToNodeMapMap {
			ramEntry, exists := ctlrPlugin.ramCache.NetworkPodToNodeMap[entry.Pod]
			if !exists || !ramEntry.ConfigEqual(entry) {
				log.Debugf("SyncNetworkPodToNodeMap: new config: %v", entry)
				renderingRequired = true
			}
		}
	}

	if renderingRequired {
		log.Debugf("SyncNetworkPodToNodeMap: rendering not required as no change in node-pod map")

		//ctlrPlugin.AddOperationMsgToQueue("", OperationalMsgOpCodeRender, nil)
	} else {
		log.Debugf("SyncNetworkPodToNodeMap: rendering not required as no change in node-pod map")
	}

	return renderingRequired
}

// RunContivKSRNetworkPodToNodeMappingWatcher enables etcd updates to be monitored
func (mgr *NetworkPodToNodeMapMgr) RunContivKSRNetworkPodToNodeMappingWatcher() {

	log.Info("ContivKSRNetworkPodToNodeMappingWatcher: enter ...")
	defer log.Info("ContivKSRNetworkPodToNodeMappingWatcher: exit ...")

	go func() {
		// back up timer ... paranoid about missing events ...
		// check every minute just in case
		ticker := time.NewTicker(1 * time.Minute)
		for _ = range ticker.C {
			ctlrPlugin.AddOperationMsgToQueue("", OperationalMsgOpCodeResyncContivNetworkPodMap, nil)
		}
	}()

	respChan := make(chan datasync.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.ContivKsrNetworkPodToNodePrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("ContivKSRNetworkPodToNodeMappingWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("ContivKSRNetworkPodToNodeMappingWatcher: watching the key: %s",
		mgr.ContivKsrNetworkPodToNodePrefix())

	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Put:
				ctlrPlugin.AddOperationMsgToQueue("", OperationalMsgOpCodeResyncContivNetworkPodMap, nil)
			case datasync.Delete:
				ctlrPlugin.AddOperationMsgToQueue("", OperationalMsgOpCodeResyncContivNetworkPodMap, nil)
			}
		}
	}
}
