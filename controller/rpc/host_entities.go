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

// HostEntityIdxRW thread-safe access to RAM Cache
type HostEntityIdxRW interface {
	GetHostEntity(hostEntityName string) (entity *controller.HostEntity, found bool)
	PutHostEntity(*controller.HostEntity) error
	ListHostEntities() []*controller.HostEntity
	ValidateHostEntity(*controller.HostEntity) error
}

// Example curl invocations: for obtaining ALL host_entities
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/HEs
func (plugin *SfcControllerRPC) hostEntitiesHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		plugin.Log.Debugf("Host Entities HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		var heArray = make([]controller.HostEntity, 0)
		for _, he := range plugin.SFCNorthbound.ListHostEntities() {
			heArray = append(heArray, *he)
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
func (plugin *SfcControllerRPC) hostEntityHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		plugin.Log.Debugf("Host Entity HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			vars := mux.Vars(req)
			if he, exists := plugin.SFCNorthbound.GetHostEntity(vars[entityName]); exists {
		formatter.JSON(w, http.StatusOK, he)
		} else {
		formatter.JSON(w, http.StatusNotFound, "host entity does not found:"+vars[entityName])
		}
			return
		case "POST":
			plugin.processHostEntityPost(formatter, w, req)
		}
	}
}

// create the host and wire to all other hosts? and external entities
func (plugin *SfcControllerRPC) processHostEntityPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		plugin.Log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}
	var he controller.HostEntity
	err = json.Unmarshal(body, &he)
	if err != nil {
		plugin.Log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if err := plugin.SFCNorthbound.ValidateHostEntity(&he); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)

	if vars[entityName] != he.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not match url name")
		return
	}
	if err := plugin.SFCNorthbound.PutHostEntity(&he); err != nil {
		formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, nil)
}
