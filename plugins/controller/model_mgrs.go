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

const (
	ModelTypeSysParameters      = "system-parameters"
	ModelTypeIPAMPool           = "ipam-pool"
	ModelTypeNetworkPodNodeMap  = "network-pod-to-node-map"
	ModelTypeNetworkNodeOverlay = "network-node-overlay"
	ModelTypeNetworkNode        = "network-node"
	ModelTypeNetworkService     = "network-service"
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
	LoadAllFromDatastoreIntoCache() error

	RenderAll()

	InitAndRunWatcher()

	// rest handlers
	InitHTTPHandlers()
	//KeyPrefix()
	//GetAllURL()

	// model type specific crud handlers ... not part of interface as they return
	// model specifc parms but implement these for each new model type
	//HandleCRUDOperationCU()
	//HandleCRUDOperationR()
	//HandleCRUDOperationD()
	//HandleCRUDOperationGetAll()
}

func (s *Plugin) RegisterModelTypeManagers() {
	RegisterModelType(ModelTypeSysParameters, &s.SysParametersMgr)
	RegisterModelType(ModelTypeIPAMPool, &s.IpamPoolMgr)
	RegisterModelType(ModelTypeNetworkPodNodeMap, &s.NetworkPodNodeMapMgr)
	RegisterModelType(ModelTypeNetworkNodeOverlay, &s.NetworkNodeOverlayMgr)
	RegisterModelType(ModelTypeNetworkNode, &s.NetworkNodeMgr)
	RegisterModelType(ModelTypeNetworkService, &s.NetworkServiceMgr)
}
