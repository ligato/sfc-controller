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

package k8scrd

import (
	"github.com/ligato/sfc-controller/plugins/controller"
)

type registeredMgrEntryType struct {
	modelTypeName string
	mgr           registeredManagersInterface
}

// RegisteredManagers contains all mgrs of models from the proto
var RegisteredManagers []*registeredMgrEntryType

func RegisterModelType(modelTypeName string, mgr registeredManagersInterface) {
	entry := &registeredMgrEntryType{
		modelTypeName: modelTypeName,
		mgr:           mgr,
	}
	RegisteredManagers = append(RegisteredManagers, entry)
}

type registeredManagersInterface interface {
	Init()
	AfterInit()
	InitRAMCache()
	DumpCache()

	InitAndRunWatcher()

}

func (s *Plugin) RegisterModelTypeManagers() {
	//RegisterModelType(controller.ModelTypeSysParameters, &s.sysParametersMgr)
	//RegisterModelType(controller.ModelTypeIPAMPool, &s.ipamPoolMgr)
	//RegisterModelType(controller.ModelTypeNetworkPodNodeMap, &s.networkPodNodeMapMgr)
	//RegisterModelType(controller.ModelTypeNetworkNodeOverlay, &s.networkNodeOverlayMgr)
	RegisterModelType(controller.ModelTypeNetworkNode, &s.NetworkNodeMgr)
	//RegisterModelType(controller.ModelTypeNetworkService, &s.networkServiceMgr)
}
