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

import "time"

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

	InitAndRunWatcher()

	// rest handlers
	InitHTTPHandlers()

	// CRUD handlers
	HandleCRUDOperationCU(data interface{}) error
	HandleCRUDOperationD(data interface{}) error
}

func (s *Plugin) RegisterModelTypeManagers() {
	RegisterModelType(ModelTypeSysParameters, &s.SysParametersMgr)
	RegisterModelType(ModelTypeIPAMPool, &s.IpamPoolMgr)
	RegisterModelType(ModelTypeNetworkPodNodeMap, &s.NetworkPodNodeMapMgr)
	RegisterModelType(ModelTypeNetworkNodeOverlay, &s.NetworkNodeOverlayMgr)
	RegisterModelType(ModelTypeNetworkNode, &s.NetworkNodeMgr)
	RegisterModelType(ModelTypeNetworkService, &s.NetworkServiceMgr)
}

func findModelManager(modelName string) registeredManagersInterface {
	for _, entry := range RegisteredManagers {
		if entry.modelTypeName == modelName {
			return entry.mgr
		}
	}
	return nil
}

const (
	OperationalMsgOpCodeRender = iota + 1
	OperationalMsgOpCodeCreateUpdate
	OperationalMsgOpCodeDelete
)

type OperationalMsg struct {
	model string
	opCode int
	data interface{}
}
var OperationalMsgChannel = make(chan *OperationalMsg,0)

//ProcessOperationalMessages is a single threaded go routine processing all operations in sequence
func (s *Plugin) ProcessOperationalMessages() {

	then := time.Now()
	isRenderingEnabled := false

	for msg := range OperationalMsgChannel {

		log.Debugf("ProcessOperationalMessages: %v", msg)

		switch msg.opCode {
		case OperationalMsgOpCodeRender:
			log.Debugf("ProcessOperationalMessages: received rendering msg")
			// special msg for startup after all config from db/yaml have been processed
			isRenderingEnabled = true

		case OperationalMsgOpCodeCreateUpdate:
			findModelManager(msg.model).HandleCRUDOperationCU(msg.data)

		case OperationalMsgOpCodeDelete:
			findModelManager(msg.model).HandleCRUDOperationD(msg.data)

		}

		if isRenderingEnabled {
			log.Debugf("ProcessOperationalMessages: check if rendering required")
			if numInChan := len(OperationalMsgChannel); numInChan != 0 {
				// there are more messages in the channel so "skip" rendering until there are no more or
				// 1 second has elapsed
				now := time.Now()
				if now.Second()-then.Second() > 1 {
					then = now
					ctlrPlugin.AddOperationMsgToQueue("", OperationalMsgOpCodeRender, nil)
					log.Debugf("ProcessOperationalMessages: queueLen: %d, 1 second elapsed ... force a render",
						numInChan)
					RenderTxnConfigStart()
					s.RenderAll()
					RenderTxnConfigEnd()
				} else {
					log.Debugf("ProcessOperationalMessages: queueLen: %d, delaying render msg for at least 1 second",
						numInChan)
				}
			} else {
				log.Debugf("ProcessOperationalMessages: queue is empty ... rendering")
				RenderTxnConfigStart()
				s.RenderAll()
				RenderTxnConfigEnd()
			}
		}
	}
}

func (s *Plugin) AddOperationMsgToQueue(model string, opCode int, data interface{}) {
	msg := &OperationalMsg {
		model : model,
		opCode: opCode,
		data: data,
	}
	log.Debugf("AddOperationMsgToQueue: %v", msg)
	OperationalMsgChannel <- msg
}