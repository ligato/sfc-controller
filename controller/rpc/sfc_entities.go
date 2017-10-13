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

// SFCEntityIdxRW thread-safe access to RAM Cache
type SFCEntityIdxRW interface {
	GetSFCEntity(sfcName string) (entity *controller.SfcEntity, found bool)
	PutSFCEntity(*controller.SfcEntity) error
	ListSFCEntities() []*controller.SfcEntity
	ValidateSFCEntity(*controller.SfcEntity) error
}

// Example curl invocations: for obtaining ALL host_entities
//   - GET:  curl -v http://localhost:9191/sfc_controller/api/v1/config/SFCs
//   - POST: not supported
func (plugin *SfcControllerRPC) sfcChainsHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		plugin.Log.Debugf("SFC Chains HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		var sfcArray = make([]controller.SfcEntity, 0)
		for _, sfc := range plugin.SFCNorthbound.ListSFCEntities() {
			sfcArray = append(sfcArray, *sfc)
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
func (plugin *SfcControllerRPC) sfcChainHandler(formatter *render.Render) http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		plugin.Log.Debugf("SFC Chain HTTP handler: Method %s, URL: %s", req.Method, req.URL)

		switch req.Method {
		case "GET":
			vars := mux.Vars(req)
			if sfc, exists := plugin.SFCNorthbound.GetSFCEntity(vars[entityName]); exists {
				formatter.JSON(w, http.StatusOK, sfc)
			} else {
				formatter.JSON(w, http.StatusNotFound, "sfc chain does not fouind:"+vars[entityName])
			}
			return
		case "POST":
			plugin.processSfcChainPost(formatter, w, req)
		}
	}
}

// wire the set/chain of containers to the vswitch and possibly external routers
func (plugin *SfcControllerRPC) processSfcChainPost(formatter *render.Render, w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		plugin.Log.Debugf("Can't read body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}
	var sfc controller.SfcEntity
	err = json.Unmarshal(body, &sfc)
	if err != nil {
		plugin.Log.Debugf("Can't parse body, error '%s'", err)
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	if err := plugin.SFCNorthbound.ValidateSFCEntity(&sfc); err != nil {
		formatter.JSON(w, http.StatusBadRequest, struct{ Error string }{err.Error()})
		return
	}

	vars := mux.Vars(req)

	if vars[entityName] != sfc.Name {
		formatter.JSON(w, http.StatusBadRequest, "json name does not matach url name")
		return
	}

	if err := plugin.SFCNorthbound.PutSFCEntity(&sfc); err != nil {
		formatter.JSON(w, http.StatusInternalServerError, struct{ Error string }{err.Error()})
		return
	}

	formatter.JSON(w, http.StatusOK, nil)
}
