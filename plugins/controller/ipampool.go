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
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/idapi/ipam"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
	"io/ioutil"
	"net"
	"net/http"
	"os"
)

type IPAMPoolMgr struct {
	ipamPoolCache map[string]*IPAMPool
}

func (mgr *IPAMPoolMgr) ToArray() []*IPAMPool {
	var array []*IPAMPool
	for _, ipamPool := range mgr.ipamPoolCache {
		array = append(array, ipamPool)
	}
	return array
}

func (mgr *IPAMPoolMgr) Init() {
	mgr.InitRAMCache()
	mgr.LoadAllFromDatastoreIntoCache()
}

func (mgr *IPAMPoolMgr) AfterInit() {
	go mgr.InitAndRunWatcher()
	if !BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// IPAMPool holds all network node specific info
type IPAMPool struct {
	controller.IPAMPool
}

// InitRAMCache create a map for all the entites
func (mgr *IPAMPoolMgr) InitRAMCache() {
	mgr.ipamPoolCache = nil // delete old cache for re-init
	mgr.ipamPoolCache = make(map[string]*IPAMPool)
}

// DumpCache logs all the entries in the map
func (mgr *IPAMPoolMgr) DumpCache() {
	for _, ipamPool := range mgr.ipamPoolCache {
		ipamPool.dumpToLog()
	}
}

func (ip *IPAMPool) dumpToLog() {
	log.Infof("IPAMPool[%s] = %v", ip.Metadata.Name, ip)
}

// ConfigEqual return true if the entities are equal
func (ipamPool *IPAMPool) ConfigEqual(ipamPool2 *IPAMPool) bool {
	if ipamPool.Metadata.String() != ipamPool2.Metadata.String() {
		return false
	}
	if ipamPool.Spec.String() != ipamPool2.Spec.String() {
		return false
	}
	// ignore Status as just comparing config
	return true
}

// AppendStatusMsg adds the message to the status section
func (ipamPool *IPAMPool) AppendStatusMsg(msg string) {
	ipamPool.Status.Msg = append(ipamPool.Status.Msg, msg)
}

// FindAllocator returns a scoped allocator for this pool entity
func (mgr *IPAMPoolMgr) FindAllocator(poolName string, entityName string) (*ipam.PoolAllocatorType, error) {

	ipamPool, exists := mgr.ipamPoolCache[poolName]
	if !exists {
		return nil, fmt.Errorf("Cannot find ipam pool %s", poolName)
	}

	allocatorName := contructAllocatorName(ipamPool, entityName)

	ipamAllocator, exists := ctlrPlugin.ramConfigCache.IPAMPoolAllocators[allocatorName]
	if !exists {
		return nil, fmt.Errorf("Cannot find allocator pool %s: allocator: %s",
			poolName, allocatorName)
	}

	return ipamAllocator, nil
}

// AllocateAddress returns a scoped ip address
func (mgr *IPAMPoolMgr) AllocateAddress(poolName string, nodeName string, vsName string) (string, error) {

	ipamPool, exists := mgr.ipamPoolCache[poolName]
	if !exists {
		return "", fmt.Errorf("Cannot find ipam pool %s", poolName)
	}
	entityName := ""
	switch ipamPool.Spec.Scope {
	case controller.IPAMPoolScopeNode:
		entityName = nodeName
	case controller.IPAMPoolScopeNetworkService:
		entityName = vsName
	}
	vxlanIpamPool, err := mgr.FindAllocator(poolName, entityName)
	if err != nil {
		return "", fmt.Errorf("Cannot find ipam pool %s: %s", poolName, err)
	}
	ipAddress, _, err := vxlanIpamPool.AllocateIPAddress()
	if err != nil {
		return "", fmt.Errorf("Cannot allocate address from ipamn pool %s", poolName)
	}
	return ipAddress, nil
}

func contructAllocatorName(ipamPool *IPAMPool, entityName string) string {
	switch ipamPool.Spec.Scope {
	case controller.IPAMPoolScopeSystem:
		return fmt.Sprintf("/%s/%s", ipamPool.Spec.Scope, ipamPool.Metadata.Name)
	case controller.IPAMPoolScopeNode:
		return fmt.Sprintf("/%s/%s/%s", ipamPool.Spec.Scope, ipamPool.Metadata.Name, entityName)
	case controller.IPAMPoolScopeNetworkService:
		return fmt.Sprintf("/%s/%s/%s", ipamPool.Spec.Scope, ipamPool.Metadata.Name, entityName)
	}
	return ""
}

// EntityCreate ensures a "scope" txnLevel pool allocator exists for this entity
func (mgr *IPAMPoolMgr) EntityCreate(entityName string, scope string) {

	for _, ipamPool := range mgr.ipamPoolCache {
		if ipamPool.Spec.Scope == scope {
			ipamPoolAllocator, err := mgr.FindAllocator(ipamPool.Metadata.Name, entityName)
			if err != nil {
				ipamPoolAllocator = ipam.NewIPAMPoolAllocator(ipamPool.Spec.Network,
					ipamPool.Spec.StartRange, ipamPool.Spec.EndRange, ipamPool.Spec.Network)
				allocatorName := contructAllocatorName(ipamPool, entityName)
				ctlrPlugin.ramConfigCache.IPAMPoolAllocators[allocatorName] = ipamPoolAllocator
			}
		}
	}

}

// EntityDelete removes a "scope" txnLevel pool allocator for this entity
func (mgr *IPAMPoolMgr) EntityDelete(entityName string, scope string) {

	for _, ipamPool := range mgr.ipamPoolCache {
		if ipamPool.Spec.Scope == scope || scope == controller.IPAMPoolScopeAny {
			ipamPoolAllocator, _ := mgr.FindAllocator(ipamPool.Metadata.Name, entityName)
			if ipamPoolAllocator != nil {
				allocatorName := contructAllocatorName(ipamPool, entityName)
				delete(ctlrPlugin.ramConfigCache.IPAMPoolAllocators, allocatorName)
			}
		}
	}

}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *IPAMPoolMgr) HandleCRUDOperationCU(_ipamPool *IPAMPool, render bool) error {

	ipamPool := &IPAMPool{}
	ipamPool.Metadata = _ipamPool.Metadata
	ipamPool.Spec = _ipamPool.Spec

	if _ipamPool.Status != nil {
		log.Warnf("IPAM Pool: %s status section: not empty for this config, ignoring %v",
			_ipamPool.Metadata.Name, _ipamPool.Status)
	}

	if err := ipamPool.validate(); err != nil {
		return err
	}

	mgr.ipamPoolCache[_ipamPool.Metadata.Name] = ipamPool

	switch ipamPool.Spec.Scope {
	case controller.IPAMPoolScopeSystem:
		mgr.EntityCreate("", ipamPool.Spec.Scope)
	case controller.IPAMPoolScopeNode:
		for _, nn := range ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationGetAll() {
			mgr.EntityCreate(nn.Metadata.Name, ipamPool.Spec.Scope)
		}
	case controller.IPAMPoolScopeNetworkService:
		for _, ns := range ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationGetAll() {
			mgr.EntityCreate(ns.Metadata.Name, ipamPool.Spec.Scope)
		}
	}

	if err := ipamPool.writeToDatastore(); err != nil {
		return err
	}

	if render {
		ipamPool.renderConfig()
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *IPAMPoolMgr) HandleCRUDOperationR(name string) (*IPAMPool, bool) {
	ipamPool, exists := mgr.ipamPoolCache[name]
	return ipamPool, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *IPAMPoolMgr) HandleCRUDOperationD(name string, render bool) error {

	ipamPool, exists := mgr.ipamPoolCache[name]
	if !exists {
		return nil
	}

	// remove from cache
	delete(mgr.ipamPoolCache, name)

	// remove from the database
	database.DeleteFromDatastore(mgr.NameKey(name))

	ctlrPlugin.IpamPoolMgr.EntityDelete(name, ipamPool.Spec.Scope)

	if render {
		log.Errorf("HandleCRUDOperationD: need to implement rerender ...")
	}

	return nil
}

// HandleCRUDOperationGetAll returns the map
func (mgr *IPAMPoolMgr) HandleCRUDOperationGetAll() map[string]*IPAMPool {
	return mgr.ipamPoolCache
}

func (ip *IPAMPool) writeToDatastore() error {
	key := ctlrPlugin.IpamPoolMgr.NameKey(ip.Metadata.Name)
	return database.WriteToDatastore(key, ip)
}

func (ip *IPAMPool) deleteFromDatastore() {
	key := ctlrPlugin.IpamPoolMgr.NameKey(ip.Metadata.Name)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *IPAMPoolMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastore: ...")
	return mgr.loadAllFromDatastore(mgr.ipamPoolCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *IPAMPoolMgr) loadAllFromDatastore(pools map[string]*IPAMPool) error {
	var ip *IPAMPool
	return database.ReadIterate(mgr.KeyPrefix(),
		func() proto.Message {
			ip = &IPAMPool{}
			return ip
		},
		func(data proto.Message) {
			pools[ip.Metadata.Name] = ip
			//log.Debugf("loadAllFromDatastore: n=%v", ip)
		})
}

const (
	entityName = "entityName"
)

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (mgr *IPAMPoolMgr) InitHTTPHandlers() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET/POST %s", mgr.KeyPrefix())
	url := fmt.Sprintf(mgr.KeyPrefix()+"{%s}", entityName)
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(url, entityHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(mgr.GetAllURL(), getAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/ipam-pool/<entityName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/ipam-pool/<entityName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/ipam-pool/<entityName>
func entityHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("entityHandler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if ip, exists := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationR(vars[entityName]); exists {
				formatter.JSON(w, http.StatusOK, ip)
			} else {
				formatter.JSON(w, http.StatusNotFound, "not found: "+vars[entityName])
			}
		case "POST":
			processPost(formatter, w, req)
		case "DELETE":
			processDelete(formatter, w, req)
		}
	}
}

func processPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var ip IPAMPool
	err = json.Unmarshal(body, &ip)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)
	if vars[entityName] != ip.Metadata.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if existing, exists := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationR(vars[entityName]); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if existing.ConfigEqual(&ip) {
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
	}

	log.Debugf("procesPost: POST: %v", ip)
	if err := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationCU(&ip, true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

func processDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

	vars := mux.Vars(req)
	if err := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationD(vars[entityName], true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

// getAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/ipam-pools
func getAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("getAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]IPAMPool, 0)
			for _, ip := range ctlrPlugin.IpamPoolMgr.HandleCRUDOperationGetAll() {
				array = append(array, *ip)
			}
			formatter.JSON(w, http.StatusOK, array)
		}
	}
}

// KeyPrefix provides sfc controller's node key prefix
func (mgr *IPAMPoolMgr) KeyPrefix() string {
	return controller.SfcControllerConfigPrefix() + "ipam-pool/"
}

// GetAllURL allows all to be retrieved
func (mgr *IPAMPoolMgr) GetAllURL() string {
	return controller.SfcControllerConfigPrefix() + "ipam-pools"
}

// NameKey provides sfc controller's entity name key prefix
func (mgr *IPAMPoolMgr) NameKey(name string) string {
	return mgr.KeyPrefix() + name
}

func (ip *IPAMPool) renderConfig() error {
	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	// first validate the config as it may have come in via a dartastore
	// update from outside rest, startup yaml ... crd?
	if err := ip.validate(); err != nil {
		return err
	}

	return nil
}

// RenderAll renders all entites in the cache
func (mgr *IPAMPoolMgr) RenderAll() {
	for _, ip := range mgr.ipamPoolCache {
		ip.renderConfig()
	}
}

func (ip *IPAMPool) validate() error {

	log.Debugf("Validating IPAMPool: %v ...", ip)

	switch ip.Spec.Scope {
	case controller.IPAMPoolScopeSystem:
	case controller.IPAMPoolScopeNode:
	case controller.IPAMPoolScopeNetworkService:
	default:
		return fmt.Errorf("IPAM pool: %s scope '%s' not recognized",
			ip.Metadata.Name, ip.Spec.Scope)
	}

	_, _, err := net.ParseCIDR(ip.Spec.Network)
	//ip, network, err := net.ParseCIDR(ipamPool.Network)
	if err != nil {
		return fmt.Errorf("ipam pool '%s', %v, expected format i.p.v.4/xx, or ip::v6/xx",
			ip.Metadata.Name, err)
	}
	// if ipamPool.StartRange != 0 {
	// 	ip = net.ParseIP(ipamPool.StartAddress)
	// 	if ip == nil {
	// 		return fmt.Errorf("ipam_pool '%s', %s not a valid ip address",
	// 			ipamPool.Name, ipamPool.StartAddress)
	// 	}
	// 	if !network.Contains(ip) {
	// 		return fmt.Errorf("ipam_pool '%s', %s not contained in network: %s",
	// 			ipamPool.Name, ipamPool.StartAddress, ipamPool.Network)
	// 	}
	// }
	// if ipamPool.EndRange != 0 {
	// 	ip = net.ParseIP(ipamPool.EndAddress)
	// 	if ip == nil {
	// 		return fmt.Errorf("ipam_pool '%s', %s not a valid ip address",
	// 			ipamPool.Name, ipamPool.EndAddress)
	// 	}
	// 	if !network.Contains(ip) {
	// 		return fmt.Errorf("ipam_pool '%s', %s not contained in network: %s",
	// 			ipamPool.Name, ipamPool.EndAddress, ipamPool.Network)
	// 	}
	// }

	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *IPAMPoolMgr) InitAndRunWatcher() {

	log.Info("IPAMPoolWatcher: enter ...")
	defer log.Info("IPAMPoolWatcher: exit ...")

	//go func() {
	//	// back up timer ... paranoid about missing events ...
	//	// check every minute just in case
	//	ticker := time.NewTicker(1 * time.Minute)
	//	for _ = range ticker.C {
	//		tempIPAMPoolMap := make(map[string]*IPAMPool)
	//		mgr.loadAllFromDatastore(tempIPAMPoolMap)
	//		renderingRequired := false
	//		for _, dbEntry := range tempIPAMPoolMap {
	//			ramEntry, exists := mgr.HandleCRUDOperationR(dbEntry.Metadata.Name)
	//			if !exists || !ramEntry.ConfigEqual(dbEntry) {
	//				log.Debugf("IPAMPoolWatcher: timer new config: %v", dbEntry)
	//				renderingRequired = true
	//				mgr.HandleCRUDOperationCU(dbEntry, false) // render at the end
	//			}
	//		}
	//		// if any of the entities required rendering, do it now
	//		if renderingRequired {
	//			RenderTxnConfigStart()
	//			ctlrPlugin.RenderAll()
	//			RenderTxnConfigEnd()
	//		}
	//		tempIPAMPoolMap = nil
	//	}
	//}()

	respChan := make(chan keyval.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("IPAMPoolWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("IPAMPoolWatcher: watching the key: %s", mgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Put:
				dbNode := &IPAMPool{}
				if err := resp.GetValue(dbNode); err == nil {
					ramNode, exists := mgr.HandleCRUDOperationR(dbNode.Metadata.Name)
					if !exists || !ramNode.ConfigEqual(dbNode) {
						log.Infof("IPAMPoolWatcher: watch config key: %s, value:%v",
							resp.GetKey(), dbNode)
						RenderTxnConfigStart()
						mgr.HandleCRUDOperationCU(dbNode, true)
						RenderTxnConfigEnd()
					}
				}

			case datasync.Delete:
				log.Infof("IPAMPoolWatcher: deleting key: %s ", resp.GetKey())
				RenderTxnConfigStart()
				mgr.HandleCRUDOperationD(resp.GetKey(), true)
				RenderTxnConfigEnd()
			}
		}
	}
}
