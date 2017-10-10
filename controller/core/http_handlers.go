// Copyright (c) 2017 Cisco and/or its affiliates.
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

// The HTTP REST interface and implementation.  The model for the controller in
// ligato/sfc-controller/controller/model drives the REST interface.  The
// model is described in the protobuf file.  Each of the entites like hosts,
// external routers, and SFC's can be configrued (CRUDed) via REST calls.

package core

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/unrolled/render"
	"io/ioutil"
	"net/http"
)

const (
	entityName = "entityName"
)

var ()

var sfcplg *SfcControllerPluginHandler

// InitHttpHandlers: register the handler funcs for GET and POST, TODO PUT/DELETE
func (sfcCtrlPlugin *SfcControllerPluginHandler) InitHttpHandlers() {

	sfcplg = sfcCtrlPlugin

	log.Infof("InitHttpHandlers: registering controller URLs. sfcplug:", sfcplg)

	url := fmt.Sprintf(controller.ExternalEntityKeyPrefix()+"{%s}", entityName)
	sfcCtrlPlugin.HTTPmux.RegisterHTTPHandler(url, externalEntityHandler, "GET", "POST")
	sfcCtrlPlugin.HTTPmux.RegisterHTTPHandler(controller.ExternalEntitiesHttpPrefix(),
		externalEntitiesHandler, "GET")

	url = fmt.Sprintf(controller.HostEntityKeyPrefix()+"{%s}", entityName)
	sfcCtrlPlugin.HTTPmux.RegisterHTTPHandler(url, hostEntityHandler, "GET", "POST")
	sfcCtrlPlugin.HTTPmux.RegisterHTTPHandler(controller.HostEntitiesHttpPrefix(), hostEntitiesHandler, "GET")

	url = fmt.Sprintf(controller.SfcEntityKeyPrefix()+"{%s}", entityName)
	sfcCtrlPlugin.HTTPmux.RegisterHTTPHandler(url, sfcChainHandler, "GET", "POST")
	sfcCtrlPlugin.HTTPmux.RegisterHTTPHandler(controller.SfcEntityHttpPrefix(), sfcChainsHandler, "GET")
}

// Example curl invocations: for obtaining ALL external_entities
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/EEs
func externalEntitiesHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("External Entities HTTP handler: Method %s, URL: %s, sfcPlugin", req.Method, req.URL, sfcplg)

		var eeArray = make([]controller.ExternalEntity, 0)
		for _, ee := range sfcplg.ramConfigCache.EEs {
			eeArray = append(eeArray, ee)
		}
		switch req.Method {
		case "GET":
			formatter.JSON(w, http.StatusOK, eeArray)
			return
		}
	}
}

// Example curl invocations: for obtaining a provided external entity
//   - GET:  curl -X GET http://localhost:9191/sfc_controller/api/v1/EE/<entityName>
//   - POST: curl -v -X POST -d '{"counter":30}' http://localhost:9191/example/test
func externalEntityHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("External Entity HTTP handler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if ee, exists := sfcplg.ramConfigCache.EEs[vars[entityName]]; exists {
				formatter.JSON(w, http.StatusOK, ee)
			} else {
				formatter.JSON(w, http.StatusNotFound, "external entity does not found:"+vars[entityName])
			}
			return
		case "POST":
			processExternalEntityPost(formatter, w, req)
		}
	}
}

// create the external entity and wire it into all the existing hosts
func processExternalEntityPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}
	var ee controller.ExternalEntity
	err = json.Unmarshal(body, &ee)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if err := sfcplg.validateEE(&ee); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)

	if vars[entityName] != ee.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	sfcplg.ramConfigCache.EEs[vars[entityName]] = ee

	if err := sfcplg.DatastoreExternalEntityPut(&ee); err != nil {
		formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})
		return
	}

	if err := sfcplg.renderExternalEntity(&ee, true, true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, nil)
}

// Example curl invocations: for obtaining ALL host_entities
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/HEs
func hostEntitiesHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("Host Entities HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		var heArray = make([]controller.HostEntity, 0)
		for _, he := range sfcplg.ramConfigCache.HEs {
			heArray = append(heArray, he)
		}
		switch req.Method {
		case "GET":
			formatter.JSON(w, http.StatusOK, heArray)
			return
		}
	}
}

// Example curl invocations: for obtaining a provided host_entity
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/HEs/<hostName>
//   - POST: curl -v -X POST -d '{"counter":30}' http://localhost:9191/example/test
func hostEntityHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("Host Entity HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			vars := mux.Vars(req)
			if he, exists := sfcplg.ramConfigCache.HEs[vars[entityName]]; exists {
				formatter.JSON(w, http.StatusOK, he)
			} else {
				formatter.JSON(w, http.StatusNotFound, "host entity does not found:"+vars[entityName])
			}
			return
		case "POST":
			processHostEntityPost(formatter, w, req)
		}
	}
}

// create the host and wire to all other hosts? and external entities
func processHostEntityPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}
	var he controller.HostEntity
	err = json.Unmarshal(body, &he)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if err := sfcplg.validateHE(&he); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)

	if vars[entityName] != he.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}
	sfcplg.ramConfigCache.HEs[vars[entityName]] = he

	if err := sfcplg.DatastoreHostEntityPut(&he); err != nil {
		formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})
		return
	}

	if err := sfcplg.renderHostEntity(&he, true, true); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, nil)
}

// Example curl invocations: for obtaining ALL host_entities
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/SFCs
//   - POST: not supported
func sfcChainsHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("SFC Chains HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		var sfcArray = make([]controller.SfcEntity, 0)
		for _, sfc := range sfcplg.ramConfigCache.SFCs {
			sfcArray = append(sfcArray, sfc)
		}
		switch req.Method {
		case "GET":
			formatter.JSON(w, http.StatusOK, sfcArray)
			return
		}
	}
}

// Example curl invocations: for obtaining a provided host_entity
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/SFCs/<chainName>
//   - POST: curl -v -X POST -d '{"counter":30}' http://localhost:9191/example/test
func sfcChainHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		log.Debugf("SFC Chain HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			vars := mux.Vars(req)
			if sfc, exists := sfcplg.ramConfigCache.SFCs[vars[entityName]]; exists {
				formatter.JSON(w, http.StatusOK, sfc)
			} else {
				formatter.JSON(w, http.StatusNotFound, "sfc chain does not fouind:"+vars[entityName])
			}
			return
		case "POST":
			processSfcChainPost(formatter, w, req)
		}
	}
}

// wire the set/chain of containers to the vswitch and possibly external routers
func processSfcChainPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}
	var sfc controller.SfcEntity
	err = json.Unmarshal(body, &sfc)
	if err != nil {
		log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if err := sfcplg.validateSFC(&sfc); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)

	if vars[entityName] != sfc.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	sfcplg.ramConfigCache.SFCs[vars[entityName]] = sfc

	if err := sfcplg.DatastoreSfcEntityPut(&sfc); err != nil {
		formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})
		return
	}

	if err := sfcplg.renderServiceFunctionEntity(&sfc); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, nil)
}
