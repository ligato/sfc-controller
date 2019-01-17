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

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"github.com/unrolled/render"
)

// SystemParametersMgr contains the system parameters for the system
type SystemParametersMgr struct {
	sysParmCache *controller.SystemParameters
}

// Init initializes the ram cache then pulls in the entries from the db
func (mgr *SystemParametersMgr) Init() {
	mgr.InitRAMCache()
	mgr.LoadAllFromDatastoreIntoCache()
}

// AfterInit sets up the watchers and http handlers
func (mgr *SystemParametersMgr) AfterInit() {
	go mgr.InitAndRunWatcher()
	if !ctlrPlugin.BypassModelTypeHttpHandlers {
		mgr.InitHTTPHandlers()
	}
}

// InitRAMCache create a map for all the entities
func (mgr *SystemParametersMgr) InitRAMCache() {
	mgr.sysParmCache = nil // delete old cache for re-init
	mgr.sysParmCache = &controller.SystemParameters{}
}

// DumpCache logs all the entries in the map
func (mgr *SystemParametersMgr) DumpCache() {
	mgr.dumpToLog(mgr.sysParmCache)
}

func (mgr *SystemParametersMgr) dumpToLog(sp *controller.SystemParameters) {
	log.Infof("SystemParameters = %v", sp)
}

// ConfigEqual return true if the entities are equal
func (mgr *SystemParametersMgr) ConfigEqual(
	sp1 *controller.SystemParameters,
	sp2 *controller.SystemParameters) bool {
	if sp1.String() != sp2.String() {
		return false
	}
	return true
}

// GetDefaultSystemBDParms return default if NO BDParms or sys template provided
func (mgr *SystemParametersMgr) GetDefaultSystemBDParms() *controller.BDParms {
	bdParms := &controller.BDParms{
		Flood:               true,
		UnknownUnicastFlood: true,
		Learn:               true,
		Forward:             true,
		ArpTermination:      false,
		MacAgeMinutes:       0,
	}
	return bdParms
}

// FindL2BDTemplate by name
func (mgr *SystemParametersMgr) FindL2BDTemplate(templateName string) *controller.BDParms {
	for _, l2bdt := range mgr.sysParmCache.L2BdTemplates {
		if templateName == l2bdt.Name {
			return l2bdt
		}
	}
	return nil
}

// HandleCRUDOperationCU add to ram cache and render
func (mgr *SystemParametersMgr) HandleCRUDOperationCU(data interface{}) error {

	sp := data.(*controller.SystemParameters)

	if err := mgr.validate(sp); err != nil {
		return err
	}

	mgr.sysParmCache = sp

	if err := mgr.writeToDatastore(sp); err != nil {
		return err
	}

	return nil
}

// HandleCRUDOperationR finds in ram cache
func (mgr *SystemParametersMgr) HandleCRUDOperationR() (*controller.SystemParameters, bool) {
	return mgr.sysParmCache, true
}

// HandleCRUDOperationD finds in ram cache
func (mgr *SystemParametersMgr) HandleCRUDOperationD(data interface{}) error {
	log.Debugf("HandleCRUDOperationD: resetting to defaults")
	mgr.sysParmCache = &controller.SystemParameters{}
	mgr.HandleCRUDOperationCU(mgr.sysParmCache)
	return nil
}

// HandleCRUDOperationGetAll returns the map
func (mgr *SystemParametersMgr) HandleCRUDOperationGetAll() *controller.SystemParameters {
	return mgr.sysParmCache
}

func (mgr *SystemParametersMgr) writeToDatastore(sp *controller.SystemParameters) error {
	return database.WriteToDatastore(ctlrPlugin.SysParametersMgr.KeyPrefix(), sp)
}

func (mgr *SystemParametersMgr) deleteFromDatastore(sp *controller.SystemParameters) {
	database.DeleteFromDatastore(ctlrPlugin.SysParametersMgr.KeyPrefix())
}

// LoadAllFromDatastoreIntoCache iterates over the etcd set
func (mgr *SystemParametersMgr) LoadAllFromDatastoreIntoCache() error {
	log.Debugf("LoadAllFromDatastoreIntoCache: ...")
	return mgr.loadAllFromDatastore(mgr.sysParmCache)
}

// loadAllFromDatastore iterates over the etcd set
func (mgr *SystemParametersMgr) loadAllFromDatastore(sp *controller.SystemParameters) error {
	return database.ReadFromDatastore(ctlrPlugin.SysParametersMgr.KeyPrefix(), sp)
}

// InitHTTPHandlers registers the handler funcs for CRUD operations
func (mgr *SystemParametersMgr) InitHTTPHandlers() {

	log.Infof("InitHTTPHandlers: registering ...")

	log.Infof("InitHTTPHandlers: registering GET/POST %s", mgr.KeyPrefix())
	ctlrPlugin.HTTPHandlers.RegisterHTTPHandler(mgr.KeyPrefix(), mgr.systemParametersHandler, "GET", "POST")
}

// curl -X GET http://localhost:9191/sfc_controller/v2/config/system-parameters
// curl -X POST -d '{json payload}' http://localhost:9191/sfc_controller/v2/config/system-parameters
func (mgr *SystemParametersMgr) systemParametersHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("systemParametersHandler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			if sp, exists := ctlrPlugin.SysParametersMgr.HandleCRUDOperationR(); exists {
				formatter.JSON(w, http.StatusOK, sp)
			} else {
				formatter.JSON(w, http.StatusNotFound, "not found")
			}
		case "POST":
			mgr.systemParametersProcessPost(formatter, w, req)
		}
	}
}

func (mgr *SystemParametersMgr) systemParametersProcessPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	var sp controller.SystemParameters
	err = json.Unmarshal(body, &sp)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if existing, exists := ctlrPlugin.SysParametersMgr.HandleCRUDOperationR(); exists {
		// if nothing has changed, simply return OK and waste no cycles
		if mgr.ConfigEqual(existing, &sp) {
			log.Debugf("processPost: config equal no further processing required")
			formatter.JSON(w, http.StatusOK, "OK")
			return
		}
		log.Debugf("processPost: old: %v", existing)
		log.Debugf("processPost: new: %v", sp)
	}

	if err := mgr.validate(&sp); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	ctlrPlugin.AddOperationMsgToQueue(ModelTypeSysParameters, OperationalMsgOpCodeCreateUpdate, &sp)

	formatter.JSON(w, http.StatusOK, "OK")
}

// KeyPrefix provides sfc controller's node key prefix
func (mgr *SystemParametersMgr) KeyPrefix() string {
	return controller.SfcControllerConfigPrefix() + "system-parameters"
}

func (mgr *SystemParametersMgr) validate(sp *controller.SystemParameters) error {
	log.Debugf("Validating SystemParameters: %v ...", sp)

	if sp.Mtu == 0 {
		sp.Mtu = 1500
	}
	if sp.DefaultStaticRoutePreference == 0 {
		sp.DefaultStaticRoutePreference = 5
	}
	if sp.RxMode != "" {
		switch sp.RxMode {
		case controller.RxModeAdaptive:
		case controller.RxModeInterrupt:
		case controller.RxModePolling:
		default:
			return fmt.Errorf("SysParm: Invalid rxMode setting %s", sp.RxMode)
		}
	}

	return nil
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *SystemParametersMgr) InitAndRunWatcher() {

	log.Info("SystemParametersWatcher: enter ...")
	defer log.Info("SystemParametersWatcher: exit ...")

	respChan := make(chan datasync.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(mgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("SystemParametersWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("SystemParametersWatcher: watching the key: %s", mgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:
			switch resp.GetChangeType() {
			case datasync.Delete:
				log.Infof("SystemParametersWatcher: deleting key: %s ", resp.GetKey())
				ctlrPlugin.AddOperationMsgToQueue(ModelTypeSysParameters, OperationalMsgOpCodeDelete, resp.GetKey())
			}
		}
	}
}

// ResolveMtu uses this input parm or the system default
func (mgr *SystemParametersMgr) ResolveMtu(mtu uint32) uint32 {

	if mtu == 0 {
		mtu = mgr.sysParmCache.Mtu
	}
	return mtu
}

// ResolveRxMode uses this input parm or the system default
func (mgr *SystemParametersMgr) ResolveRxMode(rxMode string) string {

	if rxMode == "" {
		rxMode = mgr.sysParmCache.RxMode
	}
	return rxMode
}
