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
	"net"
	"net/http"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"go.ligato.io/cn-infra/v2/datasync"
	"go.ligato.io/cn-infra/v2/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/idapi/ipam"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
)

type IPAMPoolMgr struct {
	ipamPoolCache map[string]*controller.IPAMPool
}

func (mgr *IPAMPoolMgr) ToArray() []*controller.IPAMPool {
	var array []*controller.IPAMPool
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
	if !ctlrPlugin.BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// InitRAMCache create a map for all the entites
func (mgr *IPAMPoolMgr) InitRAMCache() {
	mgr.ipamPoolCache = nil // delete old cache for re-init
	mgr.ipamPoolCache = make(map[string]*controller.IPAMPool)
}

// DumpCache logs all the entries in the map
func (mgr *IPAMPoolMgr) DumpCache() {
	for _, ipamPool := range mgr.ipamPoolCache {
		mgr.dumpToLog(ipamPool)
	}
}

func (mgr *IPAMPoolMgr) dumpToLog(ip *controller.IPAMPool) {
	log.Infof("IPAMPool[%s] = %v", ip.Metadata.Name, ip)
}

// ConfigEqual return true if the entities are equal
func (mgr *IPAMPoolMgr) ConfigEqual(
	ipamPool1 *controller.IPAMPool,
	ipamPool2 *controller.IPAMPool) bool {
	if ipamPool1.Metadata.String() != ipamPool2.Metadata.String() {
		return false
	}
	if ipamPool1.Spec.String() != ipamPool2.Spec.String() {
		return false
	}
	// ignore Status as just comparing config
	return true
}

// FindAllocator returns a scoped allocator for this pool entity
func (mgr *IPAMPoolMgr) FindAllocator(poolName string, entityName string) (*ipam.PoolAllocatorType, error) {

	ipamPool, exists := mgr.ipamPoolCache[poolName]
	if !exists {
		return nil, fmt.Errorf("Cannot find ipam pool %s", poolName)
	}

	allocatorName := contructAllocatorName(ipamPool, entityName)

	ipamAllocator, exists := ctlrPlugin.ramCache.IPAMPoolAllocators[allocatorName]
	if !exists {
		return nil, fmt.Errorf("Cannot find allocator pool %s: allocator: %s",
			poolName, allocatorName)
	}

	return ipamAllocator, nil
}

// AllocateAddress returns a scoped ip address
func (mgr *IPAMPoolMgr) AllocateAddress(poolName string, nodeName string, vsName string) (string, uint32, error) {

	ipamPool, exists := mgr.ipamPoolCache[poolName]
	if !exists {
		return "", 0, fmt.Errorf("Cannot find ipam pool %s", poolName)
	}
	entityName := ""
	switch ipamPool.Spec.Scope {
	case controller.IPAMPoolScopeNode:
		entityName = nodeName
	case controller.IPAMPoolScopeNetworkService:
		entityName = vsName
	}
	allocatorPool, err := mgr.FindAllocator(poolName, entityName)
	if err != nil {
		return "", 0, fmt.Errorf("Cannot find ipam pool allocator for %s: %s", poolName, err)
	}
	ipAddress, ipNum, err := allocatorPool.AllocateIPAddress()
	if err != nil {
		return "", 0, fmt.Errorf("Cannot allocate address from ipam pool %s", poolName)
	}

	log.Debugf("AllocateAddress: allocatorPool: %v", allocatorPool)

	ipamPool.Status.Addresses[allocatorPool.Name] = allocatorPool.GetAllocatedAddressesStatus()

	return ipAddress, ipNum, nil
}

// SetAddress sets the address as used in the allocator ... startup case
func (mgr *IPAMPoolMgr) SetAddress(poolName string, nodeName string, vsName string, ipNum uint32) (string, error) {

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
	allocatorPool, err := mgr.FindAllocator(poolName, entityName)
	if err != nil {
		return "", fmt.Errorf("Cannot find ipam pool allocator for %s: %s", poolName, err)
	}
	ipAddrStr, err := allocatorPool.SetAddress(ipNum)
	if err != nil {
		return "", fmt.Errorf("Cannot allocate address from ipam pool %s", poolName)
	}

	log.Infof("AllocateAddress: allocatorPool: %v", allocatorPool)

	ipamPool.Status.Addresses[allocatorPool.Name] = allocatorPool.GetAllocatedAddressesStatus()

	return ipAddrStr, nil
}

// SetAddressIfInPool sets the address as used in the allocator ... if it is in a pool
func (mgr *IPAMPoolMgr) SetAddressIfInPool(poolName string, nodeName string, vsName string, ipAddress string) {

	ipamPool, exists := mgr.ipamPoolCache[poolName]
	if !exists {
		return
	}
	entityName := ""
	switch ipamPool.Spec.Scope {
	case controller.IPAMPoolScopeNode:
		entityName = nodeName
	case controller.IPAMPoolScopeNetworkService:
		entityName = vsName
	}
	allocatorPool, err := mgr.FindAllocator(poolName, entityName)
	if err != nil {
		return
	}
	allocatorPool.SetIPAddrIfInsidePool(ipAddress)

	ipamPool.Status.Addresses[allocatorPool.Name] = allocatorPool.GetAllocatedAddressesStatus()
}

func contructAllocatorName(ipamPool *controller.IPAMPool, entityName string) string {
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
				allocatorName := contructAllocatorName(ipamPool, entityName)
				ipamPoolAllocator = ipam.NewIPAMPoolAllocator(allocatorName,
					ipamPool.Spec.StartRange, ipamPool.Spec.EndRange, ipamPool.Spec.Network)
				ctlrPlugin.ramCache.IPAMPoolAllocators[allocatorName] = ipamPoolAllocator
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
				delete(ipamPool.Status.Addresses, allocatorName)
				delete(ctlrPlugin.ramCache.IPAMPoolAllocators, allocatorName)
			}
		}
	}

}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *IPAMPoolMgr) HandleCRUDOperationCU(data interface{}) error {

	_ipamPool := data.(*controller.IPAMPool)

	ipamPool := &controller.IPAMPool{}
	ipamPool.Metadata = _ipamPool.Metadata
	ipamPool.Spec = _ipamPool.Spec

	if _ipamPool.Status != nil {
		log.Warnf("IPAM Pool: %s status section: not empty for this config, ignoring %v",
			_ipamPool.Metadata.Name, _ipamPool.Status)
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

	ipamPool.Status = nil
	ipamPool.Status = &controller.IPAMPoolStatus{
		Addresses: make(map[string]string, 0),
	}

	if err := mgr.writeToDatastore(ipamPool); err != nil {
		return err
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *IPAMPoolMgr) HandleCRUDOperationR(name string) (*controller.IPAMPool, bool) {
	ipamPool, exists := mgr.ipamPoolCache[name]
	return ipamPool, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *IPAMPoolMgr) HandleCRUDOperationD(data interface{}) error {

	name := data.(string)

	ipamPool, exists := mgr.ipamPoolCache[name]
	if !exists {
		return nil
	}

	// remove from cache
	delete(mgr.ipamPoolCache, name)

	// remove from the database
	database.DeleteFromDatastore(mgr.NameKey(name))

	ctlrPlugin.IpamPoolMgr.EntityDelete(name, ipamPool.Spec.Scope)

	return nil
}

// HandleCRUDOperationGetAll returns the map
func (mgr *IPAMPoolMgr) HandleCRUDOperationGetAll() map[string]*controller.IPAMPool {
	return mgr.ipamPoolCache
}

func (mgr *IPAMPoolMgr) writeToDatastore(ip *controller.IPAMPool) error {
	key := ctlrPlugin.IpamPoolMgr.NameKey(ip.Metadata.Name)
	return database.WriteToDatastore(key, ip)
}

func (mgr *IPAMPoolMgr) deleteFromDatastore(ip *controller.IPAMPool) {
	key := ctlrPlugin.IpamPoolMgr.NameKey(ip.Metadata.Name)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *IPAMPoolMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastoreIntoCache: ...")
	return mgr.loadAllFromDatastore(mgr.ipamPoolCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *IPAMPoolMgr) loadAllFromDatastore(pools map[string]*controller.IPAMPool) error {
	var ip *controller.IPAMPool
	return database.ReadIterate(mgr.KeyPrefix(),
		func() proto.Message {
			ip = &controller.IPAMPool{}
			return ip
		},
		func(data proto.Message) {
			log.Debugf("loadAllFromDatastore: n=%v", ip)
			pools[ip.Metadata.Name] = ip
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
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(url, mgr.entityHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(mgr.GetAllURL(), getAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/ipam-pool/<entityName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/ipam-pool/<entityName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/ipam-pool/<entityName>
func (mgr *IPAMPoolMgr) entityHandler(formatter *render.Render) http.HandlerFunc {

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
			mgr.processPost(formatter, w, req)
		case "DELETE":
			processDelete(formatter, w, req)
		}
	}
}

func (mgr *IPAMPoolMgr) processPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var ipamPool controller.IPAMPool
	err = json.Unmarshal(body, &ipamPool)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)
	if vars[entityName] != ipamPool.Metadata.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if existing, exists := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationR(vars[entityName]); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if mgr.ConfigEqual(existing, &ipamPool) {
			log.Debugf("processPost: config equal no further processing required")
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
		log.Debugf("processPost: old: %v", existing)
		log.Debugf("processPost: new: %v", ipamPool)
	}

	if err := mgr.validate(&ipamPool); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeIPAMPool, OperationalMsgOpCodeCreateUpdate, &ipamPool)

	formatter.JSON(w, http.StatusOK, "OK")
}

func processDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeIPAMPool, OperationalMsgOpCodeDelete, vars[entityName])

	formatter.JSON(w, http.StatusOK, "OK")
}

// getAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/ipam-pools
func getAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("getAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]controller.IPAMPool, 0)
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

func (mgr *IPAMPoolMgr) validate(ip *controller.IPAMPool) error {

	log.Debugf("Validating IPAMPool: %v ...", ip)

	if ip.Metadata == nil || ip.Spec == nil {
		return fmt.Errorf("IPAM pool: Metadata: '%v' or Spec: '%v' config is missing",
			ip.Metadata, ip.Spec)
	}

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

	respChan := make(chan datasync.ProtoWatchResp, 0)
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
			case datasync.Delete:
				log.Infof("IPAMPoolWatcher: deleting key: %s ", resp.GetKey())
				ctlrPlugin.AddOperationMsgToQueue(ModelTypeIPAMPool, OperationalMsgOpCodeDelete, resp.GetKey())

			}
		}
	}
}
