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

	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	controller "github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
	ipsec "github.com/ligato/vpp-agent/api/models/vpp/ipsec"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	"github.com/unrolled/render"
)

type NetworkServiceMgr struct {
	networkServiceCache map[string]*controller.NetworkService
}

func (mgr *NetworkServiceMgr) ToArray() []*controller.NetworkService {
	var array []*controller.NetworkService
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

// InitRAMCache create a map for all the entites
func (mgr *NetworkServiceMgr) InitRAMCache() {
	mgr.networkServiceCache = nil // delete old cache for re-init
	mgr.networkServiceCache = make(map[string]*controller.NetworkService)
}

// DumpCache logs all the entries in the map
func (mgr *NetworkServiceMgr) DumpCache() {
	for _, ns := range mgr.networkServiceCache {
		mgr.dumpToLog(ns)
	}
}

func (mgr *NetworkServiceMgr) dumpToLog(ns *controller.NetworkService) {
	log.Infof("NetworkService[%s] = %v", ns.Metadata.Name, ns)
}

// ConfigEqual return true if the entities are equal
func (mgr *NetworkServiceMgr) ConfigEqual(
	ns1 *controller.NetworkService,
	ns2 *controller.NetworkService) bool {
	if ns1.Metadata.String() != ns2.Metadata.String() {
		return false
	}
	if ns1.Spec.String() != ns2.Spec.String() {
		return false
	}
	// ignore .Status as just comparing config
	return true
}

// AppendStatusMsg adds the message to the status section
func (mgr *NetworkServiceMgr) AppendStatusMsg(ns *controller.NetworkService, msg string) {
	if ns.Status == nil {
		ns.Status = &controller.NetworkServiceStatus{}
	}
	ns.Status.Msg = append(ns.Status.Msg, msg)
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *NetworkServiceMgr) HandleCRUDOperationCU(data interface{}) error {

	_ns := data.(*controller.NetworkService)

	ns := &controller.NetworkService{}
	ns.Metadata = _ns.Metadata
	ns.Spec = _ns.Spec

	if _ns.Status != nil {
		log.Warnf("Network Service: %s status section: not empty for this config, ignoring %v",
			ns.Metadata.Name, _ns.Status)
	}

	if err := mgr.validate(ns); err != nil {
		mgr.AppendStatusMsg(ns, err.Error())
	}

	mgr.networkServiceCache[ns.Metadata.Name] = ns

	if err := mgr.writeToDatastore(ns); err != nil {
		return err
	}

	// inform ipam pool that a new ns might need a network service scope pool allocated
	ctlrPlugin.IpamPoolMgr.EntityCreate(ns.Metadata.Name, controller.IPAMPoolScopeNetworkService)

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *NetworkServiceMgr) HandleCRUDOperationR(name string) (*controller.NetworkService, bool) {
	ns, exists := mgr.networkServiceCache[name]
	return ns, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *NetworkServiceMgr) HandleCRUDOperationD(data interface{}) error {

	name := data.(string)

	if ns, exists := mgr.networkServiceCache[name]; !exists {
		return nil
	} else {
		mgr.renderDelete(ns)
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
func (mgr *NetworkServiceMgr) HandleCRUDOperationGetAll() map[string]*controller.NetworkService {
	return mgr.networkServiceCache
}

func (mgr *NetworkServiceMgr) writeToDatastore(ns *controller.NetworkService) error {
	key := ctlrPlugin.NetworkServiceMgr.NameKey(ns.Metadata.Name)
	return database.WriteToDatastore(key, ns)
}

func (mgr *NetworkServiceMgr) deleteFromDatastore(ns *controller.NetworkService) {
	key := ctlrPlugin.NetworkServiceMgr.NameKey(ns.Metadata.Name)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *NetworkServiceMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastore: ...")
	return mgr.loadAllFromDatastore(mgr.networkServiceCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *NetworkServiceMgr) loadAllFromDatastore(services map[string]*controller.NetworkService) error {
	var ns *controller.NetworkService
	return database.ReadIterate(mgr.KeyPrefix(),
		func() proto.Message {
			ns = &controller.NetworkService{}
			return ns
		},
		func(data proto.Message) {
			services[ns.Metadata.Name] = ns
			log.Debugf("loadAllFromDatastore: %v", ns)
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
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(url, mgr.networkServiceHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(mgr.GetAllURL(), networkServiceGetAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/network-service/<networkServiceName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/network-service/<networkServiceName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/network-service/<networkServiceName>
func (mgr *NetworkServiceMgr) networkServiceHandler(formatter *render.Render) http.HandlerFunc {

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
			mgr.networkServiceProcessPost(formatter, w, req)
		case "DELETE":
			networkServiceProcessDelete(formatter, w, req)
		}
	}
}

func (mgr *NetworkServiceMgr) networkServiceProcessPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var ns controller.NetworkService
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
		if mgr.ConfigEqual(existing, &ns) {
			log.Debugf("processPost: config equal no further processing required")
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
		log.Debugf("processPost: old: %v", existing)
		log.Debugf("processPost: new: %v", ns)
	}

	if err := mgr.validate(&ns); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkService, OperationalMsgOpCodeCreateUpdate, &ns)

	formatter.JSON(w, http.StatusOK, "OK")
}

func networkServiceProcessDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkService, OperationalMsgOpCodeDelete, vars[networkServiceName])

	formatter.JSON(w, http.StatusOK, "OK")
}

// networkServiceGetAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/network-services
func networkServiceGetAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("networkServiceGetAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]controller.NetworkService, 0)
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
		mgr.renderConfig(ns)
	}
}

func (mgr *NetworkServiceMgr) renderDelete(ns *controller.NetworkService) error {

	DeleteRenderedVppAgentEntries(ns.Status.RenderedVppAgentEntries)
	DeleteInterfaceEntries(ns.Status.Interfaces)

	return nil
}

func (mgr *NetworkServiceMgr) renderConfig(ns *controller.NetworkService) error {

	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	// first validate the config as it may have come in via a datastore
	// update from outside rest, startup yaml ... crd?
	if err := mgr.validate(ns); err != nil {
		return err
	}

	// initialize the network service status
	ns.Status = &controller.NetworkServiceStatus{}
	ns.Status.RenderedVppAgentEntries = make(map[string]*controller.RenderedVppAgentEntry, 0)
	ns.Status.Interfaces = make(map[string]*controller.InterfaceStatus, 0)

	defer mgr.renderComplete(ns)

	// there is a special case where loopbacks are not really part of connections but is
	// desirable to be created in the pod, so find them and render them, the connection
	// related interfaces (vnf-vswitch pairs), get rendered in renderConnectionSegments below.
	// The same goes for ethernet interfaces (eg: admin_status requested is enabled but it's currently disabled)
	for _, networkPod := range ns.Spec.NetworkPods {
		for _, iface := range networkPod.Spec.Interfaces {
			switch iface.IfType {
			case controller.IfTypeLoopBack:
				if err := mgr.RenderLoopbackInterface(ns, networkPod.Metadata.Name, iface); err != nil {
					return err
				}
			case controller.IfTypeEthernet:
				if err := mgr.RenderEthernetInterface(ns, networkPod.Metadata.Name, iface); err != nil {
					return err
				}
			}

		}
	}

	// render the interface pairs involved based on the connection type required
	for i, conn := range ns.Spec.Connections {
		log.Debugf("RenderNetworkService: %v network-service/conn: %v ", ns.Metadata.Name, conn)

		if err := mgr.renderConnectionSegments(ns, conn, uint32(i)); err != nil {
			return err
		}
	}

	log.Debugf("RenderNetworkService: network-service: %s, status: %v", ns.Metadata.Name, ns.Status)

	return nil
}

func (mgr *NetworkServiceMgr) renderComplete(ns *controller.NetworkService) error {

	if len(ns.Status.Msg) == 0 {
		mgr.AppendStatusMsg(ns, "OK")
		ns.Status.OperStatus = controller.OperStatusUp
	} else {
		RenderTxnConfigEntityRemoveEntries()
		ns.Status.RenderedVppAgentEntries = nil
		ns.Status.OperStatus = controller.OperStatusDown
	}

	// update the status info in the datastore
	if err := mgr.writeToDatastore(ns); err != nil {
		return err
	}

	log.Debugf("renderComplete: %v", ns)

	return nil
}

// RenderConnectionSegments traverses the connections and renders them
func (mgr *NetworkServiceMgr) renderConnectionSegments(
	ns *controller.NetworkService,
	conn *controller.Connection,
	i uint32) error {

	log.Debugf("RenderConnectionSegments: connType: %s", conn.ConnType)

	switch conn.ConnType {
	case controller.ConnTypeL2PP:
		if err := mgr.RenderConnL2PP(ns, conn, i); err != nil {
			return err
		}
	case controller.ConnTypeL2MP:
		if err := mgr.RenderConnL2MP(ns, conn, i); err != nil {
			return err
		}
	case controller.ConnTypeL3PP:
		if err := mgr.RenderConnL3PP(ns, conn, i); err != nil {
			return err
		}
	case controller.ConnTypeL3MP:
		if err := mgr.RenderConnL3MP(ns, conn, i); err != nil {
			return err
		}
	}

	return nil
}

func (mgr *NetworkServiceMgr) validate(ns *controller.NetworkService) error {

	log.Debugf("Validating NetworkService: %v", ns)

	if ns.Metadata == nil || ns.Metadata.Name == "" {
		errStr := "Validate error: network-service: missing metadata or name"
		log.Debugf(errStr)
		return fmt.Errorf(errStr)
	}

	if err := mgr.validateNetworkPods(ns); err != nil {
		log.Debugf(err.Error())
		return err
	}

	if ns.Spec.Connections != nil {
		if err := mgr.validateConnections(ns); err != nil {
			log.Debugf(err.Error())
			return err
		}
	}

	return nil
}

// validateConnections validates all the fields
func (mgr *NetworkServiceMgr) validateConnections(ns *controller.NetworkService) error {

	// traverse the connections and for each NetworkPod/interface, it must be in the
	// list of NetworkPod's

	for _, conn := range ns.Spec.Connections {

		switch conn.ConnType {
		case controller.ConnTypeL2PP:
			if len(conn.PodInterfaces)+len(conn.NodeInterfaces)+len(conn.NodeInterfaceLabels) != 2 {
				return fmt.Errorf("network-service: %s conn: l2 p2p must have 2 interfaces only",
					ns.Metadata.Name)
			}
		case controller.ConnTypeL2MP:
			if len(conn.PodInterfaces)+len(conn.NodeInterfaces) == 0 {
				return fmt.Errorf("network-service: %s conn: l2 p2mp must have at least one interface",
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
				}
				//else if conn.L2Bd.BdParms == nil {
				//	return fmt.Errorf("network-service: %s, conn: l2bd: %s has no DB parms nor refer to a template",
				//		ns.Metadata.Name, conn.L2Bd.Name)
				//}
			}
		case controller.ConnTypeL3PP:
			if len(conn.PodInterfaces)+len(conn.NodeInterfaces)+len(conn.NodeInterfaceLabels) != 2 {
				return fmt.Errorf("network-service: %s conn: l3 p2p must have 2 interfaces only",
					ns.Metadata.Name)
			}
		case controller.ConnTypeL3MP:
			if len(conn.PodInterfaces)+len(conn.NodeInterfaces) == 0 {
				return fmt.Errorf("network-service: %s conn: l3 p2mp must have at least one interface",
					ns.Metadata.Name)
			}
			//if conn.UseNodeL2Bd != "" || conn.L2Bd != nil {
			//	return fmt.Errorf("network-service: %s conn: cannot refer to a node bd OR provide l2bd parameters",
			//		ns.Metadata.Name)
			//}
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
				}
				//else if conn.L2Bd.BdParms == nil {
				//	return fmt.Errorf("network-service: %s, conn: l2bd: %s has no DB parms nor refer to a template",
				//		ns.Metadata.Name, conn.L2Bd.Name)
				//}
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
			if iface, _ := mgr.findNetworkPodAndInterfaceInList(ns, ConnPodName(connPodInterface),
				ConnInterfaceName(connPodInterface), ns.Spec.NetworkPods); iface == nil {
				return fmt.Errorf("network-service: %s conn: pod/port: %s not found in NetworkPod interfaces",
					ns.Metadata.Name, connPodInterface)
			}
		}

		if conn.ConnMethod != "" {
			switch conn.ConnMethod {
			case controller.ConnMethodDirect:
			case controller.ConnMethodVswitch:
			default:
				return fmt.Errorf("network-service: %s, connection has invalid conn method '%s'",
					ns.Metadata.Name, conn.ConnMethod)
			}
		}
	}

	return nil
}

// validatenetworkPods validates all the fields
func (mgr *NetworkServiceMgr) validateNetworkPods(ns *controller.NetworkService) error {

	// traverse the networkPods and for each NetworkPod/interface, it must be in the
	// list of NetworkPod's

	for _, networkPod := range ns.Spec.NetworkPods {

		if err := mgr.validateNetworkPod(ns, networkPod); err != nil {
			return err
		}
	}

	return nil
}

func (mgr *NetworkServiceMgr) findNetworkPodAndInterfaceInList(
	ns *controller.NetworkService,
	networkPodName string,
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

func (mgr *NetworkServiceMgr) validateNetworkPod(
	ns *controller.NetworkService,
	networkPod *controller.NetworkPod) error {

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
		case controller.IfTypeLoopBack:
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
			if iFace.MemifParms.RingSize != "" {
				if i, err := strconv.Atoi(iFace.MemifParms.RingSize); err != nil || i < 0 {
					return fmt.Errorf("network-service/if: %s/%s, invalid memif ring_size=%s",
						ns.Metadata.Name, iFace.Name, iFace.MemifParms.RingSize)
				}
			}
			if iFace.MemifParms.BufferSize != "" {
				if i, err := strconv.Atoi(iFace.MemifParms.BufferSize); err != nil || i < 0 {
					return fmt.Errorf("network-service/if: %s/%s, invalid memif buffer_size=%s",
						ns.Metadata.Name, iFace.Name, iFace.MemifParms.BufferSize)
				}
			}
			if iFace.MemifParms.RxQueues != "" {
				if i, err := strconv.Atoi(iFace.MemifParms.RxQueues); err != nil || i < 0 {
					return fmt.Errorf("network-service/if: %s/%s, invalid memif rx_queues=%s",
						ns.Metadata.Name, iFace.Name, iFace.MemifParms.RxQueues)
				}
			}
			if iFace.MemifParms.TxQueues != "" {
				if i, err := strconv.Atoi(iFace.MemifParms.TxQueues); err != nil || i < 0 {
					return fmt.Errorf("network-service/if: %s/%s, invalid memif tx_queues=%s",
						ns.Metadata.Name, iFace.Name, iFace.MemifParms.TxQueues)
				}
			}
		}
		if iFace.TapParms != nil {
			if iFace.TapParms.RxRingSize != "" {
				if i, err := strconv.Atoi(iFace.TapParms.RxRingSize); err != nil || i < 0 {
					return fmt.Errorf("network-service/if: %s/%s, invalid tap rx_ring_size=%s",
						ns.Metadata.Name, iFace.Name, iFace.TapParms.RxRingSize)
				}
			}
		}
		if iFace.TapParms != nil {
			if iFace.TapParms.TxRingSize != "" {
				if i, err := strconv.Atoi(iFace.TapParms.TxRingSize); err != nil || i < 0 {
					return fmt.Errorf("network-service/if: %s/%s, invalid tap tx_ring_size=%s",
						ns.Metadata.Name, iFace.Name, iFace.TapParms.TxRingSize)
				}
			}
		}
		if iFace.LinuxNamespace != "" {
			nsType, nsValue := LinuxNameSpaceTypeValue(iFace.LinuxNamespace)
			switch nsType {
			case controller.LinuxNamespaceNAME:
				if nsValue == "" {
					return fmt.Errorf("network-service/if: %s/%s, linux name space type=%s, INVALID value: =%s",
						ns.Metadata.Name, iFace.Name, nsType, nsValue)
				}
			case controller.LinuxNamespaceFILE:
				if nsValue == "" {
					return fmt.Errorf("network-service/if: %s/%s, linux name space type=%s, INVALID value: =%s",
						ns.Metadata.Name, iFace.Name, nsType, nsValue)
				}
			case controller.LinuxNamespaceMICROSERVICE:
				if nsValue == "" {
					return fmt.Errorf("network-service/if: %s/%s, linux name space type=%s, INVALID value: =%s",
						ns.Metadata.Name, iFace.Name, nsType, nsValue)
				}
			case controller.LinuxNamespacePID:
				if _, err := strconv.Atoi(nsValue); err != nil {
					return fmt.Errorf("network-service/if: %s/%s, linux name space type=%s, INVALID value: =%s",
						ns.Metadata.Name, iFace.Name, nsType, nsValue)
				}
			default:
				return fmt.Errorf("network-service/if: %s/%s, unsupported linux namespace type=%s",
					ns.Metadata.Name, iFace.Name, nsType)
			}
		}

		for _, ipsecTunnel := range iFace.IpsecTunnels {
			_, exists := ipsec.CryptoAlg_value[ipsecTunnel.CryptoAlg]
			if !exists {
				return fmt.Errorf("network-service/if: %s/%s, unsupported cryto algo for ipsec tunnel %s, val=%s",
					ns.Metadata.Name, iFace.Name, ipsecTunnel.Name, ipsecTunnel.CryptoAlg)
			}
			_, exists = ipsec.IntegAlg_value[ipsecTunnel.IntegAlg]
			if !exists {
				return fmt.Errorf("network-service/if: %s/%s, unsupported integ algo for ipsec tunnel %s, val=%s",
					ns.Metadata.Name, iFace.Name, ipsecTunnel.Name, ipsecTunnel.IntegAlg)
			}
		}
	}

	return nil
}

func (mgr *NetworkServiceMgr) findInterfaceStatus(
	ns *controller.NetworkService,
	podInterfaceName string) (*controller.InterfaceStatus, bool) {

	if ns.Status != nil && ns.Status.Interfaces != nil {
		return RetrieveInterfaceStatusFromRamCache(ns.Status.Interfaces, podInterfaceName)
	}
	return nil, false

}

func (mgr *NetworkServiceMgr) FindInterfaceStatus(podInterfaceName string) *controller.InterfaceStatus {
	for _, ns := range mgr.networkServiceCache {
		if ifStatus, exists := mgr.findInterfaceStatus(ns, podInterfaceName); exists {
			return ifStatus
		}
	}
	return nil
}

func (mgr *NetworkServiceMgr) RenderL2BD(
	ns *controller.NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	nodeName string,
	l2bdIFs []*l2.BridgeDomain_Interface,
	netPodInterfaces []*controller.Interface) error {

	// if using an existing node bridge, we simply add the i/f's to the bridge
	if conn.UseNodeL2Bd != "" {

		var nodeL2BD *l2.BridgeDomain

		// find the l2db for this node ...
		nn, nodeL2BD := ctlrPlugin.NetworkNodeMgr.FindVppL2BDForNode(nodeName, conn.UseNodeL2Bd)
		if nodeL2BD == nil {
			msg := fmt.Sprintf("network-service: %s, referencing a missing node/l2bd: %s/%s",
				ns.Metadata.Name, nn.Metadata.Name, conn.UseNodeL2Bd)
			mgr.AppendStatusMsg(ns, msg)
			return fmt.Errorf(msg)
		}
		vppKV := vppagent.AppendInterfacesToL2BD(nodeName, nodeL2BD, l2bdIFs)
		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			vppKV)

	} else {

		bviAddress := ""

		var bdParms *controller.BDParms
		if conn.L2Bd != nil {

			bviAddress = conn.L2Bd.BviAddress

			// need to create a bridge for this conn
			if conn.L2Bd.L2BdTemplate != "" {
				bdParms = ctlrPlugin.SysParametersMgr.FindL2BDTemplate(conn.L2Bd.L2BdTemplate)
			} else if conn.L2Bd.BdParms != nil {
				bdParms = conn.L2Bd.BdParms
			} else {
				bdParms = ctlrPlugin.SysParametersMgr.GetDefaultSystemBDParms()
			}
		} else {
			bdParms = ctlrPlugin.SysParametersMgr.GetDefaultSystemBDParms()
		}

		bdName := fmt.Sprintf("BD_%s_C%d", ns.Metadata.Name, connIndex+1)

		// if there is a bvi address defined
		if bviAddress != "" {

			bviLoopIfNameName := "IFLOOP_" + bdName

			vppKV := vppagent.ConstructLoopbackInterface(
				ns.Metadata.Name,
				bviLoopIfNameName,
				[]string{bviAddress},
				"",
				ctlrPlugin.SysParametersMgr.ResolveMtu(0),
				controller.IfAdminStatusEnabled,
				ctlrPlugin.SysParametersMgr.ResolveRxMode(""))
			RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
				ModelTypeNetworkService+"/"+ns.Metadata.Name,
				vppKV)

			loopIface := &l2.BridgeDomain_Interface{
				Name:                    bviLoopIfNameName,
				BridgedVirtualInterface: true,
			}

			log.Debugf("RenderL2BD: npi=%v, l2bdifs=%v", netPodInterfaces, l2bdIFs)

			if conn.L2Bd.GenerateStaticArps {

				for _, networkPodInterface := range netPodInterfaces {

					podNameAndPort := networkPodInterface.Parent + "/" + networkPodInterface.Name
					ifStatus, found := ctlrPlugin.NetworkServiceMgr.findInterfaceStatus(ns, podNameAndPort)
					if !found {
						msg := fmt.Sprintf("network-service: %s, generating static arps, missing interface: %s/%s",
							ns.Metadata.Name, networkPodInterface.Name)
						mgr.AppendStatusMsg(ns, msg)
						return fmt.Errorf(msg)
					}

					log.Debugf("RenderL2BD: podNameAndPort=%s, npi=%v, ifStatus=%v",
						podNameAndPort, networkPodInterface, ifStatus)

					// for each interface/ip in the pod, create an arp entry
					for _, ipAddress := range ifStatus.IpAddresses {

						l3ArpEntry := &controller.L3ArpEntry{
							PhysAddress:       ifStatus.MacAddress,
							IpAddress:         ipAddress,
							OutgoingInterface: bviLoopIfNameName,
						}

						vppKV := vppagent.ConstructStaticArpEntry(nodeName, l3ArpEntry)
						RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
							ModelTypeNetworkService+"/"+ns.Metadata.Name,
							vppKV)
					}
				}
			}

			if conn.L2Bd.GenerateStaticL2Fibs {

				for i, l2bdif := range l2bdIFs {

					podNameAndPort := netPodInterfaces[i].Parent + "/" + netPodInterfaces[i].Name
					ifStatus, found := ctlrPlugin.NetworkServiceMgr.findInterfaceStatus(ns, podNameAndPort)
					if !found {
						msg := fmt.Sprintf("network-service: %s, generating static fibs, missing interface: %s/%s",
							ns.Metadata.Name, netPodInterfaces[1].Parent, netPodInterfaces[1].Name)
						mgr.AppendStatusMsg(ns, msg)
						return fmt.Errorf(msg)
					}

					l2FibEntry := &controller.L2FIBEntry{
						DestMacAddress: ifStatus.MacAddress,
						BdName:         bdName,
						OutgoingIf:     l2bdif.Name,
					}

					vppKV := vppagent.ConstructStaticFib(nodeName, l2FibEntry)
					RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
						ModelTypeNetworkService+"/"+ns.Metadata.Name,
						vppKV)
				}
			}

			l2bdIFs = append(l2bdIFs, loopIface)
		}

		vppKV := vppagent.ConstructL2BD(
			nodeName,
			bdName,
			l2bdIFs,
			bdParms)
		RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
			ModelTypeNetworkService+"/"+ns.Metadata.Name,
			vppKV)

	}
	return nil
}

func (mgr *NetworkServiceMgr) RenderNetworkPodL2BD(
	ns *controller.NetworkService,
	conn *controller.Connection,
	connIndex uint32,
	podName string,
	l2bdIFs []*l2.BridgeDomain_Interface) error {

	if conn.UseNodeL2Bd != "" {
		msg := fmt.Sprintf("network-service/connIndex: %s/%d, cannot use a node/l2bd: %s for vnf=%s bridge",
			ns.Metadata.Name, connIndex+1, conn.UseNodeL2Bd, podName)
		mgr.AppendStatusMsg(ns, msg)
		return fmt.Errorf(msg)

	}

	var bdParms *controller.BDParms
	if conn.L2Bd != nil {
		// need to create a bridge for this conn
		if conn.L2Bd.L2BdTemplate != "" {
			bdParms = ctlrPlugin.SysParametersMgr.FindL2BDTemplate(conn.L2Bd.L2BdTemplate)
		} else {
			bdParms = conn.L2Bd.BdParms
		}
	} else {
		bdParms = ctlrPlugin.SysParametersMgr.GetDefaultSystemBDParms()
	}
	vppKV := vppagent.ConstructL2BD(
		podName,
		fmt.Sprintf("BD_%s_C%d", ns.Metadata.Name, connIndex+1),
		l2bdIFs,
		bdParms)
	RenderTxnAddVppEntryToTxn(ns.Status.RenderedVppAgentEntries,
		ModelTypeNetworkService+"/"+ns.Metadata.Name,
		vppKV)

	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *NetworkServiceMgr) InitAndRunWatcher() {

	log.Info("NetworkServiceWatcher: enter ...")
	defer log.Info("NetworkServiceWatcher: exit ...")

	respChan := make(chan datasync.ProtoWatchResp, 0)
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
			case datasync.Delete:
				log.Infof("NetworkServiceWatcher: deleting key: %s ", resp.GetKey())
				ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkService, OperationalMsgOpCodeDelete, resp.GetKey())
			}
		}
	}
}
