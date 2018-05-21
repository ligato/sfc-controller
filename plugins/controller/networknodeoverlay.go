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
	"github.com/ligato/sfc-controller/plugins/controller/idapi"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
)

type NetworkNodeOverlayMgr struct {
	networkNodeOverlayCache map[string]*NetworkNodeOverlay
	vniAllocators           map[string]*idapi.VxlanVniAllocatorType
	vxLanAddresses          map[string]string
}

func (mgr *NetworkNodeOverlayMgr) ToArray() []*NetworkNodeOverlay {
	var array []*NetworkNodeOverlay
	for _, nno := range mgr.networkNodeOverlayCache {
		array = append(array, nno)
	}
	return array
}

func (mgr *NetworkNodeOverlayMgr) Init() {
	mgr.InitRAMCache()
	mgr.LoadAllFromDatastoreIntoCache()
}

func (mgr *NetworkNodeOverlayMgr) AfterInit() {
	go mgr.InitAndRunWatcher()
	if !ctlrPlugin.BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// NetworkNodeOverlay holds all network node specific info
type NetworkNodeOverlay struct {
	controller.NetworkNodeOverlay
}

// InitRAMCache create a map for all the entities
func (mgr *NetworkNodeOverlayMgr) InitRAMCache() {
	mgr.networkNodeOverlayCache = nil // delete old cache for re-init
	mgr.networkNodeOverlayCache = make(map[string]*NetworkNodeOverlay)
	mgr.vniAllocators = nil
	mgr.vniAllocators = make(map[string]*idapi.VxlanVniAllocatorType)
	mgr.vxLanAddresses = nil
	mgr.vxLanAddresses = make(map[string]string)
}

// DumpCache logs all the entries in the map
func (mgr *NetworkNodeOverlayMgr) DumpCache() {
	for _, nno := range mgr.networkNodeOverlayCache {
		nno.dumpToLog()
	}
}

func (nno *NetworkNodeOverlay) dumpToLog() {
	log.Infof("networkNodeOverlay[%s] = %v", nno.Metadata.Name, nno)
}

// ConfigEqual return true if the entities are equal
func (nno *NetworkNodeOverlay) ConfigEqual(n2 *NetworkNodeOverlay) bool {
	if nno.Metadata.String() != n2.Metadata.String() {
		return false
	}
	if nno.Spec.String() != n2.Spec.String() {
		return false
	}
	// ignore snno.Status as just comparing config
	return true
}

// AppendStatusMsg adds the message to the status section
func (nno *NetworkNodeOverlay) AppendStatusMsg(msg string) {
	nno.Status.Msg = append(nno.Status.Msg, msg)
}

// AllocateVxlanAddress allocates a free address from the pool
func (mgr *NetworkNodeOverlayMgr) AllocateVxlanAddress(poolName string, nodeName string, nodeLabel string) (string, error) {

	if poolName == "" { // no pool specified ... try the hard coded vxlan endpoints in the node i/f list
		ipAddress, err := ctlrPlugin.NetworkNodeMgr.FindVxlanIPaddress(nodeName)
		if err == nil {
			return ipAddress, err
		}
		if nodeLabel != "" {
			nodeInterfaces, nodeIfTypes := ctlrPlugin.NetworkNodeMgr.FindInterfacesForThisLabelInNode(nodeName , []string{nodeLabel})
			if len(nodeInterfaces) != 1 {
				return "", fmt.Errorf("One interface must have a label: %s", nodeLabel)
			}
			if nodeIfTypes[0] != controller.IfTypeEthernet || len(nodeInterfaces[0].IpAddresses) != 1 {
				return "", fmt.Errorf("An ethernet interface with an ip_address is required for this label: %s", nodeLabel)
			}
			return nodeInterfaces[0].IpAddresses[0], nil
		}
		return ipAddress, err
	}

	if vxlanIPAddress, exists := mgr.vxLanAddresses[nodeName]; exists {
		return vxlanIPAddress, nil
	}
	vxlanIpamPool, err := ctlrPlugin.IpamPoolMgr.FindAllocator(poolName, "") // system txnLevel pool for vxlans
	if err != nil {
		return "", fmt.Errorf("Cannot find system vxlan pool %s: %s", poolName, err)
	}
	vxlanIPAddress, _, err := vxlanIpamPool.AllocateIPAddress()
	if err != nil {
		return "", fmt.Errorf("Cannot allocate address from node overlay vxlan pool %s", poolName)
	}

	mgr.vxLanAddresses[nodeName] = vxlanIPAddress

	return vxlanIPAddress, nil
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *NetworkNodeOverlayMgr) HandleCRUDOperationCU(_nno *NetworkNodeOverlay, render bool) error {

	nno := &NetworkNodeOverlay{}
	nno.Metadata = _nno.Metadata
	nno.Spec = _nno.Spec

	if _nno.Status != nil {
		log.Warnf("Network Node Overlay: %s status section: not empty for this config, ignoring %v",
			_nno.Metadata.Name, _nno.Status)
	}

	if err := nno.validate(); err != nil {
		return err
	}

	mgr.networkNodeOverlayCache[_nno.Metadata.Name] = nno

	if nno.Spec.VxlanMeshParms != nil {
		if _, exists := mgr.vniAllocators[nno.Metadata.Name]; !exists {
			mgr.vniAllocators[nno.Metadata.Name] =
				idapi.NewVxlanVniAllocator(nno.Spec.VxlanMeshParms.VniRangeStart,
					nno.Spec.VxlanMeshParms.VniRangeEnd)
		} else {
			// what if the range has been updated ... check new and old values
			// if its the same do nothing otherwise ...
		}
	}

	if err := nno.writeToDatastore(); err != nil {
		return err
	}

	if render {
		nno.renderConfig()
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *NetworkNodeOverlayMgr) HandleCRUDOperationR(name string) (*NetworkNodeOverlay, bool) {
	nno, exists := mgr.networkNodeOverlayCache[name]
	return nno, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *NetworkNodeOverlayMgr) HandleCRUDOperationD(name string, render bool) error {
	return fmt.Errorf("delete not implemented %s", name)
}

// HandleCRUDOperationGetAll returns the map
func (mgr *NetworkNodeOverlayMgr) HandleCRUDOperationGetAll() map[string]*NetworkNodeOverlay {
	return mgr.networkNodeOverlayCache
}

func (nno *NetworkNodeOverlay) writeToDatastore() error {
	key := ctlrPlugin.NetworkNodeOverlayMgr.NameKey(nno.Metadata.Name)
	return database.WriteToDatastore(key, nno)
}

func (nno *NetworkNodeOverlay) deleteFromDatastore() {
	key := ctlrPlugin.NetworkNodeOverlayMgr.NameKey(nno.Metadata.Name)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *NetworkNodeOverlayMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastore: ...")
	return mgr.loadAllFromDatastore(mgr.networkNodeOverlayCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *NetworkNodeOverlayMgr) loadAllFromDatastore(nodes map[string]*NetworkNodeOverlay) error {
	var nno *NetworkNodeOverlay
	return database.ReadIterate(mgr.KeyPrefix(),
		func() proto.Message {
			nno = &NetworkNodeOverlay{}
			return nno
		},
		func(data proto.Message) {
			nodes[nno.Metadata.Name] = nno
			//log.Debugf("loadAllFromDatastore: n=%v", nno)
		})
}

const (
	networkNodeOverlayName = "networkNodeOverlayName"
)

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (mgr *NetworkNodeOverlayMgr) InitHTTPHandlers() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET/POST %s", mgr.KeyPrefix())
	url := fmt.Sprintf(mgr.KeyPrefix()+"{%s}", networkNodeOverlayName)
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(url, networkNodeOverlayHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(mgr.GetAllURL(), networkNodeOverlayGetAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/network-service-mesh/<networkNodeOverlayName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/network-service-mesh/<networkNodeOverlayName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/network-service-mesh/<networkNodeOverlayName>
func networkNodeOverlayHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkNodeOverlayHandler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if nno, exists := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationR(vars[networkNodeOverlayName]); exists {
				formatter.JSON(w, http.StatusOK, nno)
			} else {
				formatter.JSON(w, http.StatusNotFound, "not found: "+vars[networkNodeOverlayName])
			}
		case "POST":
			networkNodeOverlayProcessPost(formatter, w, req)
		case "DELETE":
			networkNodeOverlayProcessDelete(formatter, w, req)
		}
	}
}

func networkNodeOverlayProcessPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var nno NetworkNodeOverlay
	err = json.Unmarshal(body, &nno)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)
	if vars[networkNodeOverlayName] != nno.Metadata.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if existing, exists := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationR(vars[networkNodeOverlayName]); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if existing.ConfigEqual(&nno) {
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
	}

	log.Debugf("processPost: POST: %v", nno)
	if err := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationCU(&nno, true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

func networkNodeOverlayProcessDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	//sfcplg.ConfigTransactionStart()
	//defer sfcplg.ConfigTransactionEnd()

	vars := mux.Vars(req)
	if err := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationD(vars[networkNodeOverlayName], true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

// networkNodeOverlayGetAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/network-service-meshes
func networkNodeOverlayGetAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkNodeOverlayGetAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]NetworkNodeOverlay, 0)
			for _, nno := range ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationGetAll() {
				array = append(array, *nno)
			}
			formatter.JSON(w, http.StatusOK, array)
		}
	}
}

// KeyPrefix provides sfc controller's node key prefix
func (mgr *NetworkNodeOverlayMgr) KeyPrefix() string {
	return controller.SfcControllerConfigPrefix() + "network-node-overlay/"
}

// GetAllURL allows all to be retrieved
func (mgr *NetworkNodeOverlayMgr) GetAllURL() string {
	return controller.SfcControllerConfigPrefix() + "network-node-overlays"
}

// NameKey provides sfc controller's node name key prefix
func (mgr *NetworkNodeOverlayMgr) NameKey(name string) string {
	return mgr.KeyPrefix() + name
}

func (nno *NetworkNodeOverlay) renderConfig() error {
	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	// first validate the config as it may have come in via a dartastore
	// update from outside rest, startup yaml ... crd?
	if err := nno.validate(); err != nil {
		return err
	}

	ctlrPlugin.RenderAll()

	return nil
}

// RenderAll renders all entites in the cache
func (mgr *NetworkNodeOverlayMgr) RenderAll() {
	for _, nno := range mgr.networkNodeOverlayCache {
		nno.renderConfig()
	}
}

func (nno *NetworkNodeOverlay) validate() error {

	log.Debugf("Validating networkNodeOverlay: %v ...", nno)

	switch nno.Spec.ServiceMeshType {
	case controller.NetworkNodeOverlayTypeMesh:
	case controller.NetworkNodeOverlayTypeHubAndSpoke:
	default:
		return fmt.Errorf("Network Node Overlay: %s service mesh type '%s' not recognized",
			nno.Metadata.Name, nno.Spec.ServiceMeshType)
	}

	switch nno.Spec.ConnectionType {
	case controller.NetworkNodeOverlayConnectionTypeVxlan:
		switch nno.Spec.ServiceMeshType {
		case controller.NetworkNodeOverlayTypeMesh:
			if nno.Spec.VxlanMeshParms == nil {
				return fmt.Errorf("Network Node Overlay: %s vxlan mesh parameters not specified",
					nno.Metadata.Name)
			}
			if nno.Spec.VxlanMeshParms.VniRangeStart > nno.Spec.VxlanMeshParms.VniRangeEnd ||
				nno.Spec.VxlanMeshParms.VniRangeStart == 0 || nno.Spec.VxlanMeshParms.VniRangeEnd == 0 {
				return fmt.Errorf("Network Node Overlay: %s vxlan vni range invalid", nno.Metadata.Name)
			}
		case controller.NetworkNodeOverlayTypeHubAndSpoke:
			if nno.Spec.VxlanHubAndSpokeParms == nil {
				return fmt.Errorf("Network Node Overlay: %s vxlan hub and spoke parameters not specified",
					nno.Metadata.Name)
			}
			if nno.Spec.VxlanHubAndSpokeParms.Vni == 0 {
				return fmt.Errorf("Network Node Overlay: %s vxlan vni invalid",
					nno.Metadata.Name)
			}
		default:
			return fmt.Errorf("Network Node Overlay: %s service mesh type %s not supported for connection type '%s'",
				nno.Metadata.Name,
				nno.Spec.ServiceMeshType,
				nno.Spec.ConnectionType)
		}
	default:
		return fmt.Errorf("Network Node Overlay: %s connection type '%s' not recognized",
			nno.Metadata.Name, nno.Spec.ConnectionType)
	}

	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *NetworkNodeOverlayMgr) InitAndRunWatcher() {

	log.Info("networkNodeOverlayWatcher: enter ...")
	defer log.Info("networkNodeOverlayWatcher: exit ...")

	//go func() {
	//	// back up timer ... paranoid about missing events ...
	//	// check every minute just in case
	//	ticker := time.NewTicker(1 * time.Minute)
	//	for _ = range ticker.C {
	//		tempnetworkNodeOverlayMap := make(map[string]*NetworkNodeOverlay)
	//		mgr.loadAllFromDatastore(tempnetworkNodeOverlayMap)
	//		renderingRequired := false
	//		for _, dbEntry := range tempnetworkNodeOverlayMap {
	//			ramEntry, exists := mgr.HandleCRUDOperationR(dbEntry.Metadata.Name)
	//			if !exists || !ramEntry.ConfigEqual(dbEntry) {
	//				log.Debugf("networkNodeOverlayWatcher: timer new config: %v", dbEntry)
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
	//		tempnetworkNodeOverlayMap = nil
	//	}
	//}()

	respChan := make(chan keyval.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("networkNodeOverlayWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("networkNodeOverlayWatcher: watching the key: %s", mgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Put:
				dbNode := &NetworkNodeOverlay{}
				if err := resp.GetValue(dbNode); err == nil {
					ramNode, exists := mgr.HandleCRUDOperationR(dbNode.Metadata.Name)
					if !exists || !ramNode.ConfigEqual(dbNode) {
						log.Infof("networkNodeOverlayWatcher: watch config key: %s, value:%v",
							resp.GetKey(), dbNode)
						RenderTxnConfigStart()
						mgr.HandleCRUDOperationCU(dbNode, true)
						RenderTxnConfigEnd()
					}
				}

			case datasync.Delete:
				log.Infof("networkNodeOverlayWatcher: deleting key: %s ", resp.GetKey())
				RenderTxnConfigStart()
				mgr.HandleCRUDOperationD(resp.GetKey(), true)
				RenderTxnConfigEnd()
			}
		}
	}
}
