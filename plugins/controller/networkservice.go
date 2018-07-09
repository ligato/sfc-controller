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
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
)

type NetworkServiceMgr struct {
	networkServiceCache map[string]*NetworkService
}

func (mgr *NetworkServiceMgr) ToArray() []*NetworkService {
	var array []*NetworkService
	for _, ns := range mgr.networkServiceCache {
		array = append(array, ns)
	}
	return array
}

func (mgr *NetworkServiceMgr) Init() {
	mgr.InitRAMCache()
	mgr.LoadAllFromDatastoreIntoCache()
}

func (mgr *NetworkServiceMgr) AfterInit() {
	go mgr.InitAndRunWatcher()
	if !ctlrPlugin.BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// NetworkService holds all network node specific info
type NetworkService struct {
	controller.NetworkService
}

// InitRAMCache create a map for all the entites
func (mgr *NetworkServiceMgr) InitRAMCache() {
	mgr.networkServiceCache = nil // delete old cache for re-init
	mgr.networkServiceCache = make(map[string]*NetworkService)
}

// DumpCache logs all the entries in the map
func (mgr *NetworkServiceMgr) DumpCache() {
	for _, ns := range mgr.networkServiceCache {
		ns.dumpToLog()
	}
}

func (ns *NetworkService) dumpToLog() {
	log.Infof("NetworkService[%s] = %v", ns.Metadata.Name, ns)
}

// ConfigEqual return true if the entities are equal
func (ns *NetworkService) ConfigEqual(ns2 *NetworkService) bool {
	if ns.Metadata.String() != ns2.Metadata.String() {
		return false
	}
	if ns.Spec.String() != ns2.Spec.String() {
		return false
	}
	// ignore .Status as just comparing config
	return true
}

// AppendStatusMsg adds the message to the status section
func (ns *NetworkService) AppendStatusMsg(msg string) {
	ns.Status.Msg = append(ns.Status.Msg, msg)
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *NetworkServiceMgr) HandleCRUDOperationCU(_ns *NetworkService, render bool) error {

	ns := &NetworkService{}
	ns.Metadata = _ns.Metadata
	ns.Spec = _ns.Spec

	if _ns.Status != nil {
		log.Warnf("Network Service: %s status section: not empty for this config, ignoring %v",
			ns.Metadata.Name, _ns.Status)
	}

	if err := ns.validate(); err != nil {
		return err
	}

	mgr.networkServiceCache[ns.Metadata.Name] = ns

	if err := ns.writeToDatastore(); err != nil {
		return err
	}

	// inform ipam pool that a new ns might need a network service scope pool allocated
	ctlrPlugin.IpamPoolMgr.EntityCreate(ns.Metadata.Name, controller.IPAMPoolScopeNetworkService)

	if render {
		ns.renderConfig()
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *NetworkServiceMgr) HandleCRUDOperationR(name string) (*NetworkService, bool) {
	ns, exists := mgr.networkServiceCache[name]
	return ns, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *NetworkServiceMgr) HandleCRUDOperationD(name string, render bool) error {

	if _, exists := mgr.networkServiceCache[name]; !exists {
		return nil
	}

	// remove from cache
	delete(mgr.networkServiceCache, name)

	// remove from the database
	database.DeleteFromDatastore(mgr.NameKey(name))

	// get rid of node scope ipam pool if there is one
	ctlrPlugin.IpamPoolMgr.EntityDelete(name, controller.IPAMPoolScopeNetworkService)

	return nil
}

// HandleCRUDOperationGetAll returns the map
func (mgr *NetworkServiceMgr) HandleCRUDOperationGetAll() map[string]*NetworkService {
	return mgr.networkServiceCache
}

func (ns *NetworkService) writeToDatastore() error {
	key := ctlrPlugin.NetworkServiceMgr.NameKey(ns.Metadata.Name)
	return database.WriteToDatastore(key, ns)
}

func (ns *NetworkService) deleteFromDatastore() {
	key := ctlrPlugin.NetworkServiceMgr.NameKey(ns.Metadata.Name)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *NetworkServiceMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastore: ...")
	return mgr.loadAllFromDatastore(mgr.networkServiceCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *NetworkServiceMgr) loadAllFromDatastore(services map[string]*NetworkService) error {
	var ns *NetworkService
	return database.ReadIterate(mgr.KeyPrefix(),
		func() proto.Message {
			ns = &NetworkService{}
			return ns
		},
		func(data proto.Message) {
			services[ns.Metadata.Name] = ns
			//log.Debugf("loadAllFromDatastore: %v", ns)
		})
}

const (
	networkServiceName = "networkServiceName"
)

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (mgr *NetworkServiceMgr) InitHTTPHandlers() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET/POST %s", mgr.KeyPrefix())
	url := fmt.Sprintf(mgr.KeyPrefix()+"{%s}", networkServiceName)
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(url, networkServiceHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPmux.RegisterHTTPHandler(mgr.GetAllURL(), networkServiceGetAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/network-service/<networkServiceName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/network-service/<networkServiceName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/network-service/<networkServiceName>
func networkServiceHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkServiceHandler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if n, exists := ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationR(vars[networkServiceName]); exists {
				formatter.JSON(w, http.StatusOK, n)
			} else {
				formatter.JSON(w, http.StatusNotFound, "not found: "+vars[networkServiceName])
			}
		case "POST":
			networkServiceProcessPost(formatter, w, req)
		case "DELETE":
			networkServiceProcessDelete(formatter, w, req)
		}
	}
}

func networkServiceProcessPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var ns NetworkService
	err = json.Unmarshal(body, &ns)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)
	if vars[networkServiceName] != ns.Metadata.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if existing, exists := ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationR(vars[networkServiceName]); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if existing.ConfigEqual(&ns) {
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
	}

	log.Debugf("processPost: POST: %v", ns)
	if err := ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationCU(&ns, true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

func networkServiceProcessDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	RenderTxnConfigStart()
	defer RenderTxnConfigEnd()

	vars := mux.Vars(req)
	if err := ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationD(vars[networkServiceName], true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, "OK")
}

// networkServiceGetAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/network-services
func networkServiceGetAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkServiceGetAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]NetworkService, 0)
			for _, ns := range ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationGetAll() {
				array = append(array, *ns)
			}
			formatter.JSON(w, http.StatusOK, array)
		}
	}
}

// KeyPrefix provides sfc controller's node key prefix
func (mgr *NetworkServiceMgr) KeyPrefix() string {
	return controller.SfcControllerConfigPrefix() + "network-service/"
}

// GetAllURL allows all to be retrieved
func (mgr *NetworkServiceMgr) GetAllURL() string {
	return controller.SfcControllerConfigPrefix() + "network-services"
}

// NameKey provides sfc controller's node name key prefix
func (mgr *NetworkServiceMgr) NameKey(name string) string {
	return mgr.KeyPrefix() + name
}

// RenderAll renders all entities in the cache
func (mgr *NetworkServiceMgr) RenderAll() {
	for _, ns := range mgr.networkServiceCache {
		ns.renderConfig()
	}
}

func (ns *NetworkService) renderConfig() error {

	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	// first validate the config as it may have come in via a datastore
	// update from outside rest, startup yaml ... crd?
	if err := ns.validate(); err != nil {
		return err
	}

	// remember the existing entries for this entity so we can compare afterwards to see what changed
	CopyRenderedVppAgentEntriesToBeforeCfgTxn(ModelTypeNetworkService + "/" + ns.Metadata.Name)

	// initialize the network service status
	ns.Status = &controller.NetworkServiceStatus{}
	ns.Status.RenderedVppAgentEntries = make(map[string]*controller.RenderedVppAgentEntry, 0)
	ns.Status.Interfaces = make(map[string]*controller.InterfaceStatus, 0)

	defer ns.renderComplete()


	for i, conn := range ns.Spec.Connections {
		log.Debugf("RenderNetworkService: network-service/conn: ", ns.Metadata.Name, conn)

		if err := ns.renderConnectionSegments(conn, uint32(i)); err != nil {
			return err
		}
	}

	log.Debugf("RenderNetworkService: network-service:%s, status:%v", ns.Metadata.Name, ns.Status)

	return nil
}

func (ns *NetworkService) renderComplete() error {

	if len(ns.Status.Msg) == 0 {
		ns.AppendStatusMsg("OK")
		ns.Status.OperStatus = controller.OperStatusUp
	} else {
		RenderTxnConfigEntityRemoveEntries()
		ns.Status.RenderedVppAgentEntries = nil
		ns.Status.OperStatus = controller.OperStatusDown
	}

	// update the status info in the datastore
	if err := ns.writeToDatastore(); err != nil {
		return err
	}

	log.Debugf("renderComplete: %v", ns)

	return nil
}

// RenderConnectionSegments traverses the connections and renders them
func (ns *NetworkService) renderConnectionSegments(
	conn *controller.Connection,
	i uint32) error {

	log.Debugf("RenderConnectionSegments: connType: %s", conn.ConnType)

	switch conn.ConnType {
	case controller.ConnTypeL2PP:
		if err := ns.RenderConnL2PP(conn, i); err != nil {
			return err
		}
	case controller.ConnTypeL2MP:
		if err := ns.RenderConnL2MP(conn, i); err != nil {
			return err
		}
	}

	return nil
}

func (ns *NetworkService) validate() error {

	log.Debugf("Validating NetworkService: %v", ns)

	if ns.Metadata == nil || ns.Metadata.Name == "" {
		return fmt.Errorf("network-service: missing metadata or name")
	}

	if err := ns.validateNetworkPods(); err != nil {
		return err
	}

	if ns.Spec.Connections != nil {
		if err := ns.validateConnections(); err != nil {
			return err
		}
	}

	return nil
}

// validateConnections validates all the fields
func (ns *NetworkService) validateConnections() error {

	// traverse the connections and for each NetworkPod/interface, it must be in the
	// list of NetworkPod's

	for _, conn := range ns.Spec.Connections {

		switch conn.ConnType {
		case controller.ConnTypeL2PP:
			if len(conn.PodInterfaces) + len(conn.NodeInterfaces) + len(conn.NodeInterfaceLabels) != 2 {
				return fmt.Errorf("network-service: %s conn: p2p must have 2 interfaces only",
					ns.Metadata.Name)
			}
		case controller.ConnTypeL2MP:
			if len(conn.PodInterfaces) + len(conn.NodeInterfaces) == 0 {
				return fmt.Errorf("network-service: %s conn: p2mp must have at least one interface",
					ns.Metadata.Name)
			}
			if conn.UseNodeL2Bd != "" && conn.L2Bd != nil {
				return fmt.Errorf("network-service: %s conn: cannot refer to a node bd AND provide l2bd parameters",
					ns.Metadata.Name)
			}
			if conn.L2Bd != nil {
				if conn.L2Bd.L2BdTemplate != "" && conn.L2Bd.BdParms != nil {
					return fmt.Errorf("network-service: %s conn: l2bd: %s  cannot refer to temmplate and provide l2bd parameters",
						ns.Metadata.Name, conn.L2Bd.Name)
				}
				if conn.L2Bd.L2BdTemplate != "" {
					if l2bdt := ctlrPlugin.SysParametersMgr.FindL2BDTemplate(conn.L2Bd.L2BdTemplate); l2bdt == nil {
						return fmt.Errorf("network-service: %s, conn: l2bd: %s  has invalid reference to non-existant l2bd template '%s'",
							ns.Metadata.Name, conn.L2Bd.Name, conn.L2Bd.L2BdTemplate)
					}
				} else if conn.L2Bd.BdParms == nil {
					return fmt.Errorf("network-service: %s, conn: l2bd: %s has no DB parms nor refer to a template",
						ns.Metadata.Name, conn.L2Bd.Name)
				}
			}
		default:
			return fmt.Errorf("network-service: %s, connection has invalid conn type '%s'",
				ns.Metadata.Name, conn.ConnType)
		}

		for _, connPodInterface := range conn.PodInterfaces {
			connPodName, connInterfaceName := ConnPodInterfaceNames(connPodInterface)
			if connPodName == "" || connInterfaceName == "" {
				return fmt.Errorf("network-service: %s conn: pod/port: %s must be of pod/interface format",
					ns.Metadata.Name, connPodInterface)
			}
			if iface, _ := ns.findNetworkPodAndInterfaceInList(ConnPodName(connPodInterface),
				ConnInterfaceName(connPodInterface), ns.Spec.NetworkPods); iface == nil {
				return fmt.Errorf("network-service: %s conn: pod/port: %s not found in NetworkPod interfaces",
					ns.Metadata.Name, connPodInterface)
			}
		}
	}

	return nil
}

// validatenetworkPods validates all the fields
func (ns *NetworkService) validateNetworkPods() error {

	// traverse the networkPods and for each NetworkPod/interface, it must be in the
	// list of NetworkPod's

	for _, networkPod := range ns.Spec.NetworkPods {

		if err := ns.validateNetworkPod(networkPod); err != nil {
			return err
		}
	}

	return nil
}

func (ns *NetworkService) findNetworkPodAndInterfaceInList(networkPodName string,
	ifName string,
	networkPods []*controller.NetworkPod) (*controller.Interface, string) {

	for _, networkPod := range networkPods {
		for _, iFace := range networkPod.Spec.Interfaces {
			if networkPod.Metadata.Name == networkPodName && iFace.Name == ifName {
				return iFace, networkPod.Spec.PodType
			}
		}
	}
	return nil, ""
}

func (ns *NetworkService) validateNetworkPod(networkPod *controller.NetworkPod) error {

	if networkPod.Metadata == nil || networkPod.Metadata.Name == "" {
		return fmt.Errorf("ValidatePod: network-service: %s, network-pod is missing metadata or name",
			ns.Metadata.Name)
	}
	if networkPod.Spec == nil {
		return fmt.Errorf("ValidatePod: network-service: %s, network-pod: %s is missing spec",
			ns.Metadata.Name, networkPod.Metadata.Name)
	}
	switch networkPod.Spec.PodType {
	case controller.NetworkPodTypeExternal:
	case controller.NetworkPodTypeVPPContainer:
	case controller.NetworkPodTypeNonVPPContainer:
	default:
		return fmt.Errorf("ValidatePod: network-service: %s, network-pod: %s invalid pod-type %s",
			ns.Metadata.Name, networkPod.Metadata.Name, networkPod.Spec.PodType)
	}

	if len(networkPod.Spec.Interfaces) == 0 {
		return fmt.Errorf("network-service/pod: %s/%s no interfaces defined",
			ns.Metadata.Name, networkPod.Metadata.Name)
	}

	for _, iFace := range networkPod.Spec.Interfaces {
		switch iFace.IfType {
		case controller.IfTypeMemif:
		case controller.IfTypeEthernet:
		case controller.IfTypeVeth:
		case controller.IfTypeTap:
		default:
			return fmt.Errorf("network-service/pod: %s/%s has invalid if type '%s'",
				ns.Metadata.Name, iFace.Name, iFace.IfType)
		}
		for _, ipAddress := range iFace.IpAddresses {
			ip, network, err := net.ParseCIDR(ipAddress)
			if err != nil {
				return fmt.Errorf("network-service/if: %s/%s '%s', expected format i.p.v.4/xx, or ip::v6/xx",
					ns.Metadata.Name, iFace.Name, err)
			}
			log.Debugf("ValidatePod: ip: %s, network: %s", ip, network)
		}
		if iFace.MemifParms != nil {
			if iFace.MemifParms.Mode != "" {
				switch iFace.MemifParms.Mode {
				case controller.IfMemifModeEhernet:
				case controller.IfMemifModeIP:
				case controller.IfMemifModePuntInject:
				default:
					return fmt.Errorf("network-service/if: %s/%s, unsupported memif mode=%s",
						ns.Metadata.Name, iFace.Name, iFace.MemifParms.Mode)
				}
			}
			if iFace.MemifParms.InterPodConn != "" {
				switch iFace.MemifParms.InterPodConn {
				case controller.IfMemifInterPodConnTypeDirect:
				case controller.IfMemifInterPodConnTypeVswitch:
				default:
					return fmt.Errorf("network-service/if: %s/%s, unsupported memif inter-vnf connection type=%s",
						ns.Metadata.Name, iFace.Name, iFace.MemifParms.InterPodConn)
				}
			}
		}
	}

	return nil
}

func (ns *NetworkService) findInterfaceStatus(podInterfaceName string) (*controller.InterfaceStatus, bool) {

	return RetrieveInterfaceStatusFromRamCache(ns.Status.Interfaces, podInterfaceName)
}

func (mgr *NetworkServiceMgr) FindInterfaceStatus(podInterfaceName string) *controller.InterfaceStatus {
	for _, ns := range mgr.networkServiceCache {
		if ifStatus, exists := ns.findInterfaceStatus(podInterfaceName); exists {
			return ifStatus
		}
	}
	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *NetworkServiceMgr) InitAndRunWatcher() {

	log.Info("NetworkServiceWatcher: enter ...")
	defer log.Info("NetworkServiceWatcher: exit ...")

	//go func() {
	//	// back up timer ... paranoid about missing events ...
	//	// check every minute just in case
	//	ticker := time.NewTicker(1 * time.Minute)
	//	for _ = range ticker.C {
	//		tempNetworkServiceMap := make(map[string]*NetworkService)
	//		mgr.loadAllFromDatastore(tempNetworkServiceMap)
	//		renderingRequired := false
	//		for _, dbNode := range tempNetworkServiceMap {
	//			ramNode, exists := mgr.HandleCRUDOperationR(dbNode.Metadata.Name)
	//			if !exists || !ramNode.ConfigEqual(dbNode) {
	//				log.Debugf("NetworkServiceWatcher: timer new config: %v", dbNode)
	//				renderingRequired = true
	//				mgr.HandleCRUDOperationCU(dbNode, false) // render at the end
	//			}
	//		}
	//		// if any of the entities required rendering, do it now
	//		if renderingRequired {
	//			RenderTxnConfigStart()
	//			ctlrPlugin.RenderAll()
	//			RenderTxnConfigEnd()
	//		}
	//		tempNetworkServiceMap = nil
	//	}
	//}()

	respChan := make(chan keyval.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("NetworkServiceWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("NetworkServiceWatcher: watching the key: %s", mgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Put:
				dbNode := &NetworkService{}
				if err := resp.GetValue(dbNode); err == nil {
					ramNode, exists := mgr.HandleCRUDOperationR(dbNode.Metadata.Name)
					if !exists || !ramNode.ConfigEqual(dbNode) {
						log.Infof("NetworkServiceWatcher: watch config key: %s, value:%v",
							resp.GetKey(), dbNode)
						RenderTxnConfigStart()
						mgr.HandleCRUDOperationCU(dbNode, true)
						RenderTxnConfigEnd()
					}
				}

			case datasync.Delete:
				log.Infof("NetworkServiceWatcher: deleting key: %s ", resp.GetKey())
				RenderTxnConfigStart()
				mgr.HandleCRUDOperationD(resp.GetKey(), true)
				RenderTxnConfigEnd()
			}
		}
	}
}
