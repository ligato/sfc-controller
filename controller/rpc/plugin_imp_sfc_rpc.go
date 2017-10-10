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
	"fmt"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/flavors/local"
)

const (
	entityName = "entityName"
)

// SfcControllerRPC encapsulates HTTP/REST RPCs of SFC Controller
// for GET/POST on top of:
// - Host Entities
// - SFC Elements
// - External Entities
type SfcControllerRPC struct {
	Deps
}

// Deps contains all injected dependencies of SfcControllerRPC
type Deps struct {
	HTTP          rest.HTTPHandlers //inject
	local.PluginLogDeps             //inject
	SFCNorthbound SFCNorthboundRW   //inject
}

// SFCAPI allows to Get/Put External Entity, SFC Entity, Host Entity
type SFCNorthboundRW interface {
	ExternalEntityIdxRW
	SFCEntityIdxRW
	HostEntityIdxRW
}

// Init registers the handler funcs for GET and POST, TODO PUT/DELETE
func (plugin *SfcControllerRPC) Init() {
	plugin.Log.Infof("InitHttpHandlers: registering controller URLs")

	url := fmt.Sprintf(controller.ExternalEntityKeyPrefix()+"{%s}", entityName)
	plugin.HTTP.RegisterHTTPHandler(url, plugin.externalEntityHandler, "GET", "POST")
	plugin.HTTP.RegisterHTTPHandler(controller.ExternalEntitiesHttpPrefix(),
		plugin.externalEntitiesHandler, "GET")

	url = fmt.Sprintf(controller.HostEntityKeyPrefix()+"{%s}", entityName)
	plugin.HTTP.RegisterHTTPHandler(url, plugin.hostEntityHandler, "GET", "POST")
	plugin.HTTP.RegisterHTTPHandler(controller.HostEntitiesHttpPrefix(), plugin.hostEntitiesHandler, "GET")

	url = fmt.Sprintf(controller.SfcEntityKeyPrefix()+"{%s}", entityName)
	plugin.HTTP.RegisterHTTPHandler(url, plugin.sfcChainHandler, "GET", "POST")
	plugin.HTTP.RegisterHTTPHandler(controller.SfcEntityHttpPrefix(), plugin.sfcChainsHandler, "GET")
}
