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
	"os"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/sfc-controller/plugins/controller"
	crd "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/apis/sfccontroller/v1alpha1"
	model "github.com/ligato/sfc-controller/plugins/controller/model"
)

type CRDNetworkNodeMgr struct {
	networkNodeCache map[string]string
}

func (mgr *CRDNetworkNodeMgr) Init() {
}

func (mgr *CRDNetworkNodeMgr) AfterInit() {
	mgr.InitRAMCache()
	go mgr.InitAndRunWatcher()
}

// InitRAMCache create a map for all the entities
func (mgr *CRDNetworkNodeMgr) InitRAMCache() {
	//mgr.networkNodeCache = nil // delete old cache for re-init
	//mgr.networkNodeCache = make(map[string]*NetworkNode)
}

// DumpCache logs all the entries in the map
func (mgr *CRDNetworkNodeMgr) DumpCache() {
	for _, nn := range mgr.networkNodeCache {
		log.Printf("NetworkNode: %s", nn)
	}
}

func (mgr *CRDNetworkNodeMgr) CrdToSfcNetworkNode(crdNN crd.NetworkNode) (sfcNN controller.NetworkNode, err error) {
	sfcNN = controller.NetworkNode{}
	sfcNN.Metadata = &model.MetaDataType{}
	sfcNN.Metadata.Name = crdNN.Name
	sfcNN.Metadata.Labels = crdNN.Labels
	sfcNN.NetworkNode.Spec = &crdNN.NetworkNodeSpec
	sfcNN.NetworkNode.Status = &crdNN.NetworkNodeStatus
	return sfcNN, nil
}

// HandleCRDSync syncs the SFC Controller with the incoming CRD
func (mgr *CRDNetworkNodeMgr) HandleCRDSync(crdNN crd.NetworkNode) {
	log.Info("CRDNetworkNodeMgr HandleCRDSync: enter ...")
	defer log.Info("CRDNetworkNodeMgr HandleCRDSync: exit ...")

	nn, err := mgr.CrdToSfcNetworkNode(crdNN)
	if err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	opStr := "created"
	if existing, exists := ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationR(crdNN.Name); exists {
		opStr = "updated"
		if existing.ConfigEqual(&nn) {
			log.Infof("crdNN %s has not changed.", crdNN.Name)
			return
		}
	}

	if err := ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationCU(&nn, false); err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	log.Infof("NetworkNode %s", opStr)
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *CRDNetworkNodeMgr) InitAndRunWatcher() {

	log.Info("CRD NetworkNodeWatcher: enter ...")
	defer log.Info("CRD NetworkNodeWatcher: exit ...")

	respChan := make(chan keyval.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(ctlrPlugin.NetworkNodeMgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("CRD NetworkNodeWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("CRD NetworkNodeWatcher: watching the key: %s", ctlrPlugin.NetworkNodeMgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:

			switch resp.GetChangeType() {
			case datasync.Put:
				dbEntry := &controller.NetworkNode{}
				if err := resp.GetValue(dbEntry); err == nil {
					// config and status might have changed ...
					log.Infof("CRD NetworkNodeWatcher: PUT detected: NetworkNode: %s",
						dbEntry)
				}

			case datasync.Delete:
				log.Infof("CRD NetworkNodeWatcher: deleting key: %s ", resp.GetKey())
				// tell k8s crd that resource has been removed
			}
		}
	}
}
