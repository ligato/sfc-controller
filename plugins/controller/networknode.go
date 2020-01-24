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
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/ligato/sfc-controller/plugins/controller/vppagent"
	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	"github.com/unrolled/render"
)

type NetworkNodeMgr struct {
	networkNodeCache map[string]*controller.NetworkNode
}

func (mgr *NetworkNodeMgr) ToArray() []*controller.NetworkNode {
	var array []*controller.NetworkNode
	for _, nn := range mgr.networkNodeCache {
		array = append(array, nn)
	}
	return array
}

func (mgr *NetworkNodeMgr) Init() {
	mgr.InitRAMCache()
	mgr.LoadAllFromDatastoreIntoCache()
}

func (mgr *NetworkNodeMgr) AfterInit() {
	go mgr.InitAndRunWatcher()
	if !ctlrPlugin.BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// InitRAMCache create a map for all the entities
func (mgr *NetworkNodeMgr) InitRAMCache() {
	mgr.networkNodeCache = nil // delete old cache for re-init
	mgr.networkNodeCache = make(map[string]*controller.NetworkNode)
}

// DumpCache logs all the entries in the map
func (mgr *NetworkNodeMgr) DumpCache() {
	for _, nn := range mgr.networkNodeCache {
		mgr.dumpToLog(nn)
	}
}

func (mgr *NetworkNodeMgr) dumpToLog(nn *controller.NetworkNode) {
	log.Infof("NetworkNode[%s] = %v", nn.Metadata.Name, nn)
}

// ConfigEqual return true if the entities are equal
func (mgre *NetworkNodeMgr) ConfigEqual(
	n1 *controller.NetworkNode,
	n2 *controller.NetworkNode) bool {
	if n1.Metadata.String() != n2.Metadata.String() {
		return false
	}
	if n1.Spec.String() != n2.Spec.String() {
		return false
	}
	// ignore nn.Status as just comparing status
	return true
}

// AppendStatusMsg adds the message to the status section
func (mgr *NetworkNodeMgr) AppendStatusMsg(nn *controller.NetworkNode, msg string) {
	if nn.Status == nil {
		nn.Status = &controller.NetworkNodeStatus{}
	}
	nn.Status.Msg = append(nn.Status.Msg, msg)
}

// FindVxlanIPaddress looks up the vxlan ip address for this node in the if list
func (mgr *NetworkNodeMgr) FindVxlanIPaddress(nodeName string) (string, error) {

	nn, exists := mgr.HandleCRUDOperationR(nodeName)
	if !exists {
		return "", fmt.Errorf("node not found: %s", nodeName)
	}
	for _, iFace := range nn.Spec.Interfaces {
		switch iFace.IfType {
		case controller.IfTypeVxlanTunnel:
			for _, ipAddress := range iFace.IpAddresses {
				_, _, err := net.ParseCIDR(ipAddress)
				if err == nil {
					log.Debugf("FindVxlanIPaddress: node: %s, found vxlan ipaddr: %s",
						nodeName, ipAddress)
					return ipAddress, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no vxlan ip address found for node: %s", nodeName)
}

func (mgr *NetworkNodeMgr) FindInterfaceInNode(nodeName string, ifName string) (*controller.Interface, string) {

	nn, exists := mgr.networkNodeCache[nodeName]
	if !exists {
		return nil, ""
	}
	for _, iFace := range nn.Spec.Interfaces {
		if iFace.Name == ifName {
			return iFace, iFace.IfType
		}
	}

	return nil, ""
}

func (mgr *NetworkNodeMgr) FindInterfacesForThisLabelInNode(nodeName string,
	labels []string) ([]*controller.Interface, []string) {

	var interfaces []*controller.Interface
	var ifTypes []string

	nn, exists := mgr.networkNodeCache[nodeName]
	if !exists {
		log.Debugf("FindInterfacesForThisLabelInNode: node not found: %s", nodeName)
		return interfaces, ifTypes
	}
	for _, iFace := range nn.Spec.Interfaces {
		for _, ifaceLabel := range iFace.Labels {
			for _, label := range labels {
				if ifaceLabel == label {
					log.Debugf("FindInterfacesForThisLabelInNode: label matched: node/iface/label: %s/%s/%s",
						nodeName, iFace.Name, label)
					interfaces = append(interfaces, iFace)
					ifTypes = append(ifTypes, iFace.IfType)
					break
				}
			}
		}
	}

	return interfaces, ifTypes
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *NetworkNodeMgr) HandleCRUDOperationCU(data interface{}) error {

	_nn := data.(*controller.NetworkNode)

	nn := &controller.NetworkNode{}
	nn.Metadata = _nn.Metadata
	nn.Spec = _nn.Spec

	if _nn.Status != nil {
		log.Warnf("Network Node: %s status section: not empty for this config, ignoring %v",
			_nn.Metadata.Name, _nn.Status)
	}

	if err := mgr.validate(nn); err != nil {
		nn.Status = &controller.NetworkNodeStatus{}
		mgr.AppendStatusMsg(nn, err.Error())
	}

	mgr.networkNodeCache[_nn.Metadata.Name] = nn

	if err := mgr.writeToDatastore(nn); err != nil {
		return err
	}

	// inform ipam pool that a new node might need a node scope pool allocated
	ctlrPlugin.IpamPoolMgr.EntityCreate(_nn.Metadata.Name, controller.IPAMPoolScopeNode)

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *NetworkNodeMgr) HandleCRUDOperationR(name string) (*controller.NetworkNode, bool) {
	n, exists := mgr.networkNodeCache[name]
	return n, exists
}

// HandleCRUDOperationD removes from ram cache
func (mgr *NetworkNodeMgr) HandleCRUDOperationD(data interface{}) error {

	nodeName := data.(string)

	if nn, exists := mgr.networkNodeCache[nodeName]; !exists {
		return nil
	} else {
		mgr.renderDelete(nn)
	}

	// remove from cache
	delete(mgr.networkNodeCache, nodeName)

	// remove from the database
	database.DeleteFromDatastore(mgr.NameKey(nodeName))

	// get rid of allocated ipam pool for this node if there is one
	ctlrPlugin.IpamPoolMgr.EntityDelete(nodeName, controller.IPAMPoolScopeNode)

	return nil
}

// HandleCRUDOperationGetAll returns the map
func (mgr *NetworkNodeMgr) HandleCRUDOperationGetAll() map[string]*controller.NetworkNode {
	return mgr.networkNodeCache
}

func (mgr *NetworkNodeMgr) writeToDatastore(nn *controller.NetworkNode) error {
	key := ctlrPlugin.NetworkNodeMgr.NameKey(nn.Metadata.Name)
	return database.WriteToDatastore(key, nn)
}

func (mgr *NetworkNodeMgr) deleteFromDatastore(nn *controller.NetworkNode) {
	key := ctlrPlugin.NetworkNodeMgr.NameKey(nn.Metadata.Name)
	database.DeleteFromDatastore(key)
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *NetworkNodeMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastoreIntoCache: ...")
	return mgr.loadAllFromDatastore(mgr.networkNodeCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *NetworkNodeMgr) loadAllFromDatastore(nodes map[string]*controller.NetworkNode) error {
	var nn *controller.NetworkNode
	return database.ReadIterate(mgr.KeyPrefix(),
		func() proto.Message {
			nn = &controller.NetworkNode{}
			return nn
		},
		func(data proto.Message) {
			nodes[nn.Metadata.Name] = nn
			log.Debugf("loadAllFromDatastore: n=%v", nn)
		})
}

const (
	networkNodeName = "networkNodeName"
)

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (mgr *NetworkNodeMgr) InitHTTPHandlers() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET/POST %s", mgr.KeyPrefix())
	url := fmt.Sprintf(mgr.KeyPrefix()+"{%s}", networkNodeName)
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(url, mgr.httpNetworkNodeHandler, "GET", "POST", "DELETE")
	log.Infof("InitHTTPHandlers: registering GET %s", mgr.GetAllURL())
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(mgr.GetAllURL(), httpNetworkNodeGetAllHandler, "GET")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/network-node/<networkNodeName>
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/network-node/<networkNodeName>
// curl -X DELETE http://localhost:9191/sfc_controller/v2/config/network-node/<networkNodeName>
func (mgr *NetworkNodeMgr) httpNetworkNodeHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("httpNetworkNodeHandler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if n, exists := ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationR(vars[networkNodeName]); exists {
				formatter.JSON(w, http.StatusOK, n)
			} else {
				formatter.JSON(w, http.StatusNotFound, "not found: "+vars[networkNodeName])
			}
		case "POST":
			mgr.httpNetworkNodeProcessPost(formatter, w, req)
		case "DELETE":
			mgr.httpNetworkNodeProcessDelete(formatter, w, req)
		}
	}
}

func (mgr *NetworkNodeMgr) httpNetworkNodeProcessPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var nn controller.NetworkNode
	err = json.Unmarshal(body, &nn)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)
	if vars[networkNodeName] != nn.Metadata.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if existing, exists := ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationR(vars[networkNodeName]); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if err := mgr.validate(&nn); err == nil {
			if mgr.ConfigEqual(existing, &nn) {
				log.Debugf("processPost: config equal no further processing required")
				formatter.JSON(w, http.StatusOK, "OK")
				return
			}
		}
		log.Debugf("processPost: old: %v", existing)
		log.Debugf("processPost: new: %v", nn)
	}

	if err := mgr.validate(&nn); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkNode, OperationalMsgOpCodeCreateUpdate, &nn)

	formatter.JSON(w, http.StatusOK, "OK")
}

func (mgr *NetworkNodeMgr) httpNetworkNodeProcessDelete(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkNode, OperationalMsgOpCodeDelete, vars[networkNodeName])

	formatter.JSON(w, http.StatusOK, "OK")
}

// httpNetworkNodeGetAllHandler GET: curl -v http://localhost:9191/sfc-controller/v2/config/network-nodes
func httpNetworkNodeGetAllHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("httpNetworkNodeGetAllHandler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			var array = make([]controller.NetworkNode, 0)
			for _, nn := range ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationGetAll() {
				array = append(array, *nn)
			}
			formatter.JSON(w, http.StatusOK, array)
		}
	}
}

// KeyPrefix provides sfc controller's node key prefix
func (mgr *NetworkNodeMgr) KeyPrefix() string {
	return controller.SfcControllerConfigPrefix() + "network-node/"
}

// GetAllURL allows all to be retrieved
func (mgr *NetworkNodeMgr) GetAllURL() string {
	return controller.SfcControllerConfigPrefix() + "network-nodes"
}

// NameKey provides sfc controller's node name key prefix
func (mgr *NetworkNodeMgr) NameKey(name string) string {
	return mgr.KeyPrefix() + name
}

func (mgr *NetworkNodeMgr) renderDelete(nn *controller.NetworkNode) error {

	DeleteRenderedVppAgentEntries(nn.Status.RenderedVppAgentEntries)
	DeleteInterfaceEntries(nn.Status.Interfaces)

	return nil
}

func (mgr *NetworkNodeMgr) renderConfig(nn *controller.NetworkNode) error {

	RenderTxnConfigEntityStart()
	defer RenderTxnConfigEntityEnd()

	// first validate the config as it may have come in via a DB
	// update from outside rest, startup yaml ... crd?
	if err := mgr.validate(nn); err != nil {
		return err
	}

	log.Debugf("renderConfig: before nn.Status=%v", nn.Status)

	nn.Status = &controller.NetworkNodeStatus{}
	nn.Status.RenderedVppAgentEntries = make(map[string]*controller.RenderedVppAgentEntry, 0)
	nn.Status.Interfaces = make(map[string]*controller.InterfaceStatus, 0)

	defer mgr.renderComplete(nn)

	// create interfaces in the vswitch
	if nn.Spec.Interfaces != nil {
		if err := mgr.renderNodeInterfaces(nn); err != nil {
			return err
		}
	}

	// create l2bds
	if nn.Spec.L2Bds != nil {
		if err := mgr.renderNodeL2BDs(nn); err != nil {
			return err
		}
	}
	log.Debugf("renderConfig: after nn.Status=%v", nn.Status)

	return nil
}

// RenderAll renders all entities in the cache
func (mgr *NetworkNodeMgr) RenderAll() {
	for _, nn := range mgr.networkNodeCache {
		mgr.renderConfig(nn)
	}
}

func vppKeyL2BDName(nodeName, l2bdName string) string {
	return "L2BD_" + nodeName + "_" + l2bdName
}

func (mgr *NetworkNodeMgr) renderNodeL2BDs(nn *controller.NetworkNode) error {

	var bdParms *controller.BDParms

	for _, l2bd := range nn.Spec.L2Bds {

		if l2bd.L2BdTemplate != "" {
			bdParms = ctlrPlugin.SysParametersMgr.FindL2BDTemplate(l2bd.L2BdTemplate)
		} else {
			if l2bd.BdParms == nil {
				bdParms = ctlrPlugin.SysParametersMgr.GetDefaultSystemBDParms()
			} else {
				bdParms = l2bd.BdParms
			}
		}

		var iFaces []*l2.BridgeDomain_Interface

		// if there is a bvi address defined
		if l2bd.BviAddress != "" {

			ifName := "IFLOOP_"+vppKeyL2BDName(nn.Metadata.Name, l2bd.Name)

			vppKV := vppagent.ConstructLoopbackInterface(
				nn.Metadata.Name,
				ifName,
				[]string{l2bd.BviAddress},
				"",
				ctlrPlugin.SysParametersMgr.ResolveMtu(0),
				controller.IfAdminStatusEnabled,
				ctlrPlugin.SysParametersMgr.ResolveRxMode(""))
			RenderTxnAddVppEntryToTxn(nn.Status.RenderedVppAgentEntries,
				ModelTypeNetworkNode+"/"+nn.Metadata.Name,
				vppKV)
			loopIface := &l2.BridgeDomain_Interface{
				Name: ifName,
				BridgedVirtualInterface: true,
			}

			iFaces = append(iFaces, loopIface)
		}

		vppKV := vppagent.ConstructL2BD(
			nn.Metadata.Name,
			vppKeyL2BDName(nn.Metadata.Name, l2bd.Name),
			iFaces,
			bdParms)
		RenderTxnAddVppEntryToTxn(nn.Status.RenderedVppAgentEntries,
			ModelTypeNetworkNode+"/"+nn.Metadata.Name,
			vppKV)

		log.Infof("renderNodeL2BDs: vswitch: %s, vppKV: %v", nn.Metadata.Name, vppKV)
	}

	return nil
}

func (mgr *NetworkNodeMgr) renderNodeInterfaces(nn *controller.NetworkNode) error {

	var vppKV *vppagent.KVType

	for _, iFace := range nn.Spec.Interfaces {
		switch iFace.IfType {
		case controller.IfTypeEthernet:
			if !iFace.BypassRenderer {

				ifStatus, err := InitInterfaceStatus(nn.Metadata.Name, nn.Metadata.Name, iFace)
				if err != nil {
					RemoveInterfaceStatus(nn.Status.Interfaces, iFace.Parent, iFace.Name)
					msg := fmt.Sprintf("node interface: %s/%s, %s", iFace.Parent, iFace.Name, err)
					mgr.AppendStatusMsg(nn, msg)
					return err
				}
				PersistInterfaceStatus(nn.Status.Interfaces, ifStatus, iFace.Parent, iFace.Name)

				vppKV = vppagent.ConstructEthernetInterface(
					nn.Metadata.Name,
					iFace.Name,
					ifStatus.IpAddresses,
					ifStatus.MacAddress,
					ctlrPlugin.SysParametersMgr.ResolveMtu(iFace.Mtu),
					iFace.AdminStatus,
					ctlrPlugin.SysParametersMgr.ResolveRxMode(iFace.RxMode))

				RenderTxnAddVppEntryToTxn(nn.Status.RenderedVppAgentEntries,
					ModelTypeNetworkNode+"/"+nn.Metadata.Name,
					vppKV)
			}
		}
		log.Infof("renderNodeInterfaces: vswitch: %s, ifType: %s, vppKV: %v",
			nn.Metadata.Name, iFace.IfType, vppKV)
	}

	return nil
}

func (mgr *NetworkNodeMgr) renderComplete(nn *controller.NetworkNode) error {

	if len(nn.Status.Msg) == 0 {
		mgr.AppendStatusMsg(nn,"OK")
		nn.Status.Status = controller.OperStatusUp
	} else {
		RenderTxnConfigEntityRemoveEntries()
		nn.Status.RenderedVppAgentEntries = nil
		nn.Status.Status = controller.OperStatusDown
	}

	// update the status info in the datastore
	if err := mgr.writeToDatastore(nn); err != nil {
		return err
	}

	log.Debugf("renderComplete: %v", nn)

	return nil
}

// FindL2BDForNode by name
func (mgr *NetworkNodeMgr) FindL2BDForNode(nodeName string, l2bdName string) *controller.L2BD {

	if nn, exists := mgr.networkNodeCache[nodeName]; !exists {
		return nil
	} else {
		for _, l2bd := range nn.Spec.L2Bds {
			if l2bd.Name == l2bdName {
				return l2bd
			}
		}
	}
	return nil
}

// FindVppL2BDForNode by name
func (mgr *NetworkNodeMgr) FindVppL2BDForNode(nodeName string, l2bdName string) (*controller.NetworkNode,
	*l2.BridgeDomain) {

	var vppKey *vppagent.KVType
	var exists bool

	if l2bd := mgr.FindL2BDForNode(nodeName, l2bdName); l2bd == nil {
		return nil, nil
	}

	// first look in the txn cache then the system cache
	key := vppagent.L2BridgeDomainKey(nodeName, vppKeyL2BDName(nodeName, l2bdName))
	if vppKey, exists = RenderTxnGetAfterMap(key); !exists {
		if vppKey, exists = ctlrPlugin.ramCache.VppEntries[key]; !exists {
			return nil, nil
		}
	}

	return mgr.networkNodeCache[nodeName], vppKey.L2BD
}

func findInterfaceLabel(labels []string, label string) bool {
	for _, l := range labels {
		if l == label {
			return true
		}
	}
	return false
}

// RenderVxlanLoopbackInterfaceAndStaticRoutes renders static routes for the vxlan
func (mgr *NetworkNodeMgr) RenderVxlanLoopbackInterfaceAndStaticRoutes(
	renderingEntity string,
	fromNode string,
	toNode string,
	vrfID uint32,
	fromVxlanAddress string,
	toVxlanAddress string,
	createLoopbackInterface bool,
	createLoopbackStaticRoutes bool,
	networkNodeInterfaceLabel string) map[string]*controller.RenderedVppAgentEntry {

	//var renderedEntries map[string]*controller.RenderedVppAgentEntry
	var renderedEntries = make(map[string]*controller.RenderedVppAgentEntry)

	// strip the /xx off of the address
	fromVxlanAddress = vppagent.StripSlashAndSubnetIPAddress(fromVxlanAddress)

	// keep the /xx for the route
	//toVxlanAddress = vppagent.StripSlashAndSubnetIPAddress(toVxlanAddress)

	// depending on the number of ethernet/label:vxlan interfaces on the source node and
	// the number of ethernet/label:vxlan inerfaces on the dest node, a set of static
	// routes will be created

	// for now assume 1 address per node and soon there will ba a v4 and a v6 ?

	n1 := mgr.networkNodeCache[fromNode]

	if createLoopbackInterface {
		// make sure there is a loopback i/f entry for this vxlan endpoint
		vppKV := vppagent.ConstructLoopbackInterface(n1.Metadata.Name,
			"IF_VXLAN_LOOPBACK_"+fromNode,
			[]string{fromVxlanAddress},
			"",
			ctlrPlugin.SysParametersMgr.sysParmCache.Mtu,
			controller.IfAdminStatusEnabled,
			ctlrPlugin.SysParametersMgr.sysParmCache.RxMode)
		RenderTxnAddVppEntryToTxn(renderedEntries, renderingEntity, vppKV)
	}

	n2 := mgr.networkNodeCache[toNode]

	if createLoopbackStaticRoutes {
		for _, node1Iface := range n1.Spec.Interfaces {
			if node1Iface.IfType != controller.IfTypeEthernet ||
				!(findInterfaceLabel(node1Iface.Labels, networkNodeInterfaceLabel) ||
					len(n1.Spec.Interfaces) == 1) { // if only one ethernet if, it does not need the label
				continue
			}
			for _, node2Iface := range n2.Spec.Interfaces {
				if node2Iface.IfType != controller.IfTypeEthernet ||
					!(findInterfaceLabel(node2Iface.Labels, networkNodeInterfaceLabel) ||
						len(n2.Spec.Interfaces) == 1) { // if only one ethernet if, it does not need the label
					continue
				}

				l3sr := &controller.L3VRFRoute{
					VrfId:             vrfID,
					Description:       fmt.Sprintf("L3VRF_VXLAN Node:%s to Node:%s", fromNode, toNode),
					DstIpAddr:         toVxlanAddress, // dest node vxlan address/xx
					NextHopAddr:       node2Iface.IpAddresses[0],
					OutgoingInterface: node1Iface.Name,
					Weight:            ctlrPlugin.SysParametersMgr.sysParmCache.DefaultStaticRouteWeight,
					Preference:        ctlrPlugin.SysParametersMgr.sysParmCache.DefaultStaticRoutePreference,
				}
				vppKV := vppagent.ConstructStaticRoute(n1.Metadata.Name, l3sr)
				RenderTxnAddVppEntryToTxn(renderedEntries, renderingEntity, vppKV)
			}
		}
	}
	return renderedEntries
}

func (mgr *NetworkNodeMgr) validate(nn *controller.NetworkNode) error {
	log.Debugf("Validating NetworkNode: %v ...", nn)

	//if nn.Spec.Interfaces != nil && nn.Vswitches != nil {
	//	return fmt.Errorf("node: %s can only model 1 vswitch with interfaces, or multiple with vswitches, but not both",
	//		nn.Metadata.Name)
	//} else if nn.Spec.HasMultipleVswitches && nn.Interfaces != nil {
	//	return fmt.Errorf("node: %s indicating multiple vswitches but using the interfaces array, use vnfs instead",
	//		nn.Metadata.Name)
	//} else if !nn.Spec.HasMultipleVswitches && nn.Vswitches != nil {
	//	return fmt.Errorf("node: %s indicating single vswitch but using the vnf's array, use interfaces instead",
	//		nn.Metadata.Name)
	//}

	if nn.Spec.Interfaces != nil {
		if err := mgr.nodeValidateInterfaces(nn, nn.Metadata.Name, nn.Spec.Interfaces); err != nil {
			return err
		}
	}

	//if nn.Spec.Vswitches != nil {
	//	for _, vnf := range nn.Vswitches {
	//		if vnf.Name == "" {
	//			return fmt.Errorf("node: %s has missing vnf name", nn.Metadata.Name)
	//		}
	//		switch vnf.VnfType {
	//		case controller.VNFTypeVPPVswitch:
	//		case controller.VNFTypeExternal:
	//		default:
	//			return fmt.Errorf("vnf: %s has invalid vnf type '%s'",
	//				vnf.Name, vnf.VnfType)
	//		}
	//
	//		if err := nodeValidateInterfaces(vnf.Name, vnf.Interfaces); err != nil {
	//			return err
	//		}
	//	}
	//}

	for _, l2bd := range nn.Spec.L2Bds {

		if l2bd.L2BdTemplate != "" && l2bd.BdParms != nil {
			return fmt.Errorf("node: %s, l2bd: %s  cannot refer to temmplate and provide l2bd parameters",
				nn.Metadata.Name, l2bd.Name)
		}
		if l2bd.L2BdTemplate != "" {
			if l2bdt := ctlrPlugin.SysParametersMgr.FindL2BDTemplate(l2bd.L2BdTemplate); l2bdt == nil {
				return fmt.Errorf("node: %s, l2bd: %s  has invalid reference to non-existant l2bd template '%s'",
					nn.Metadata.Name, l2bd.Name, l2bd.L2BdTemplate)
			}
		}
	}

	return nil
}

func (mgr *NetworkNodeMgr) nodeValidateInterfaces(nn *controller.NetworkNode,
	nodeName string, iFaces []*controller.Interface) error {

	for _, iFace := range iFaces {
		switch iFace.IfType {
		case controller.IfTypeEthernet:
		case controller.IfTypeVxlanTunnel:
		default:
			return fmt.Errorf("node/if: %s/%s has invalid if type '%s'",
				nodeName, iFace.Name, iFace.IfType)
		}
		iFace.Parent = nn.Metadata.Name
		for _, ipAddress := range iFace.IpAddresses {
			ip, network, err := net.ParseCIDR(ipAddress)
			if err != nil {
				return fmt.Errorf("node/if: %s/%s '%s', expected format i.p.v.4/xx, or ip::v6/xx",
					nodeName, iFace.Name, err)
			}
			log.Debugf("nodeValidateInterfaces: name: %s/%s, ip: %s, network: %s",
				nn.Metadata.Name, iFace.Name, ip, network)
		}
	}

	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *NetworkNodeMgr) InitAndRunWatcher() {

	log.Info("NetworkNodeWatcher: enter ...")
	defer log.Info("NetworkNodeWatcher: exit ...")

	respChan := make(chan datasync.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("NetworkNodeWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("NetworkNodeWatcher: watching the key: %s", mgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:

			switch resp.GetChangeType() {
			case datasync.Delete:
				log.Infof("NetworkNodeWatcher: deleting key: %s ", resp.GetKey())
				ctlrPlugin.AddOperationMsgToQueue(ModelTypeNetworkNode, OperationalMsgOpCodeDelete, resp.GetKey())
			}
		}
	}
}
