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

package rpc

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/unrolled/render"
	"io/ioutil"
	"net/http"
)

// ExternalEntityIdxRW thread-safe access to RAM Cache
type ExternalEntityIdxRW interface {
	GetExternalEntity(externalEntityName string) (entity *controller.ExternalEntity, found bool)
	DeleteExternalEntity(externalEntityName string) (found bool, err error)
	PutExternalEntity(*controller.ExternalEntity) error
	ListExternalEntities() []*controller.ExternalEntity
	ValidateExternalEntity(*controller.ExternalEntity) error
}

// Example curl invocations: for obtaining ALL external_entities
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/EEs
func (plugin *SfcControllerRPC) externalEntitiesHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		plugin.Log.Debugf("External Entities HTTP handler: Method %s, URL: %s, sfcPlugin",
			req.Method, req.URL)

		var eeArray = make([]controller.ExternalEntity, 0)
		for _, ee := range plugin.SFCNorthbound.ListExternalEntities() {
			eeArray = append(eeArray, *ee)
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
func (plugin *SfcControllerRPC) externalEntityHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		plugin.Log.Debugf("External Entity HTTP handler: Method %s, URL: %s", req.Method, req.URL)
		switch req.Method {
		case "GET":
			vars := mux.Vars(req)

			if ee, exists := plugin.SFCNorthbound.GetExternalEntity(vars[entityName]); exists {
				formatter.JSON(w, http.StatusOK, ee)
			} else {
				formatter.JSON(w, http.StatusNotFound, "external entity does not found: "+vars[entityName])
			}
			return
		case "DELETE":
			vars := mux.Vars(req)

			if exists, err := plugin.SFCNorthbound.DeleteExternalEntity(vars[entityName]); err != nil {
				formatter.JSON(w, http.StatusInternalServerError, "error deleting: "+vars[entityName]+" "+err.Error())
			} else if exists {
				formatter.JSON(w, http.StatusOK, "deleted: "+vars[entityName])
			} else {
				formatter.JSON(w, http.StatusNotFound, "external entity does not found: "+vars[entityName])
			}
		case "POST", "PUT":
			plugin.processExternalEntityPost(formatter, w, req)
		}
	}
}

// create the external entity and wire it into all the existing hosts
func (plugin *SfcControllerRPC) processExternalEntityPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		plugin.Log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}
	var ee controller.ExternalEntity
	err = json.Unmarshal(body, &ee)
	if err != nil {
		plugin.Log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if err := plugin.SFCNorthbound.ValidateExternalEntity(&ee); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)

	if vars[entityName] != ee.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if err := plugin.SFCNorthbound.PutExternalEntity(&ee); err != nil {
		formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, nil)
}
