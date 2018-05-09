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
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
	"time"
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
	mgr.InitHTTPHandlers()
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

// HandleContivKSRStatusUpdate add to ram cache and render
func (mgr *NetworkPodToNodeMapMgr) HandleContivKSRStatusUpdate(p2n *NetworkPodToNodeMap, render bool) error {

	if err := p2n.validate(); err != nil {
		return err
	}

	ctlrPlugin.ramConfigCache.NetworkPodToNodeMap[p2n.Pod] = p2n


	if render {
		p2n.renderConfig()
	}

	return nil
}

// HandleContivKSRStatusDelete add to ram cache and render
func (mgr *NetworkPodToNodeMapMgr) HandleContivKSRStatusDelete(podName string, render bool) error {

	delete(ctlrPlugin.ramConfigCache.NetworkPodToNodeMap, podName)

	if render {
		mgr.RenderAll()
	}

	return nil
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationCU(p2n *NetworkPodToNodeMap, render bool) error {

	if err := p2n.validate(); err != nil {
		return err
	}

	mgr.networkPodNodeCache[p2n.Pod] = p2n
	ctlrPlugin.ramConfigCache.NetworkPodToNodeMap[p2n.Pod] = p2n

	if err := p2n.writeToDatastore(); err != nil {
		return err
	}

	if render {
		p2n.renderConfig()
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationR(name string) (*NetworkPodToNodeMap, bool) {
	p2n, exists := mgr.networkPodNodeCache[name]
	return p2n, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *NetworkPodToNodeMapMgr) HandleCRUDOperationD(name string, render bool) error {

	delete(mgr.networkPodNodeCache, name)
	if render {
		mgr.RenderAll()
	}

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
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(url, networkPodNodeMapHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(mgr.GetAllURL(), networkPodNodeMapGetAllHandler, "GET")
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

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

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
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
	}

	log.Debugf("processPost: POST: %v", p2n)
	if err := ctlrPlugin.NetworkPodNodeMapMgr.HandleCRUDOperationCU(&p2n, true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

func networkPodNodeMapProcessDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

	vars := mux.Vars(req)
	if err := ctlrPlugin.NetworkPodNodeMapMgr.HandleCRUDOperationD(vars[networkPodName], true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

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
		"vnf-to-node/"
}

func (p2n *NetworkPodToNodeMap) renderConfig() error {
	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	// first validate the config as it may have come in via a datastore
	// update from outside rest, startup yaml ... crd?
	if err := p2n.validate(); err != nil {
		return err
	}

	ctlrPlugin.NetworkServiceMgr.RenderAll()

	return nil
}

// RenderAll renders all entites in the cache
func (mgr *NetworkPodToNodeMapMgr) RenderAll() {
	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	ctlrPlugin.NetworkServiceMgr.RenderAll()
}

func (p2n *NetworkPodToNodeMap) validate() error {
	log.Debugf("Validating NetworkPodToNodeMap: %v ...", p2n)
	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *NetworkPodToNodeMapMgr) InitAndRunWatcher() {

	log.Info("NetworkPodToNodeMapWatcher: enter ...")
	defer log.Info("NetworkPodToNodeMapWatcher: exit ...")

	go func() {
		// back up timer ... paranoid about missing events ...
		// check every minute just in case
		ticker := time.NewTicker(1 * time.Minute)
		for _ = range ticker.C {
			tempNetworkPodToNodeMapMap := make(map[string]*NetworkPodToNodeMap)
			mgr.loadAllFromDatastore(mgr.KeyPrefix(), tempNetworkPodToNodeMapMap)
			renderingRequired := false
			for _, dbEntry := range tempNetworkPodToNodeMapMap {
				ramEntry, exists := mgr.HandleCRUDOperationR(dbEntry.Pod)
				if !exists || !ramEntry.ConfigEqual(dbEntry) {
					log.Debugf("NetworkPodToNodeMapWatcher: timer new config: %v", dbEntry)
					renderingRequired = true
					mgr.HandleCRUDOperationCU(dbEntry, false) // render at the end
				}
			}
			// if any of the entities required rendering, do it now
			if renderingRequired {
				RenderTxnConfigStart()
				ctlrPlugin.RenderAll()
				RenderTxnConfigEnd()
			}
			tempNetworkPodToNodeMapMap = nil
		}
	}()

	respChan := make(chan keyval.ProtoWatchResp, 0)
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
			case datasync.Put:
				dbEntry := &NetworkPodToNodeMap{}
				if err := resp.GetValue(dbEntry); err == nil {
					ramEntry, exists := mgr.HandleCRUDOperationR(dbEntry.Pod)
					if !exists || !ramEntry.ConfigEqual(dbEntry) {
						log.Infof("NetworkPodToNodeMapWatcher: watch config key: %s, value:%v",
							resp.GetKey(), dbEntry)
						RenderTxnConfigStart()
						mgr.HandleCRUDOperationCU(dbEntry, true)
						RenderTxnConfigEnd()
					}
				}

			case datasync.Delete:
				log.Infof("NetworkPodToNodeMapWatcher: deleting key: %s ", resp.GetKey())
				RenderTxnConfigStart()
				mgr.HandleCRUDOperationD(resp.GetKey(), true)
				RenderTxnConfigEnd()
			}
		}
	}
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
			tempNetworkPodToNodeMapMap := make(map[string]*NetworkPodToNodeMap)
			mgr.loadAllFromDatastore(mgr.ContivKsrNetworkPodToNodePrefix(), tempNetworkPodToNodeMapMap)
			renderingRequired := false
			for _, dbEntry := range tempNetworkPodToNodeMapMap {
				ramEntry, exists := ctlrPlugin.ramConfigCache.NetworkPodToNodeMap[dbEntry.Pod]
				if !exists || !ramEntry.ConfigEqual(dbEntry) {
					log.Debugf("ContivKSRNetworkPodToNodeMappingWatcher: timer new config: %v", dbEntry)
					renderingRequired = true
					mgr.HandleContivKSRStatusUpdate(dbEntry, false) // render at the end
				}
			}
			// if any of the entities required rendering, do it now
			if renderingRequired {
				RenderTxnConfigStart()
				ctlrPlugin.RenderAll()
				RenderTxnConfigEnd()
			}
			tempNetworkPodToNodeMapMap = nil
		}
	}()

	respChan := make(chan keyval.ProtoWatchResp, 0)
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
				p2n := &NetworkPodToNodeMap{}
				if err := resp.GetValue(p2n); err == nil {
					log.Infof("ContivKSRNetworkPodToNodeMappingWatcher: key: %s, value:%v", resp.GetKey(), p2n)
					RenderTxnConfigStart()
					mgr.HandleContivKSRStatusUpdate(p2n, true)
					RenderTxnConfigEnd()
				}

			case datasync.Delete:
				log.Infof("ContivKSRNetworkPodToNodeMappingWatcher: deleting key: %s ", resp.GetKey())
				RenderTxnConfigStart()
				mgr.HandleContivKSRStatusDelete(resp.GetKey(), true)
				RenderTxnConfigEnd()
			}
		}
	}
}
