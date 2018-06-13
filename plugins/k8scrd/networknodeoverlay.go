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

type CRDNetworkNodeOverlayMgr struct {
	networkNodeOverlayCache map[string]string
}

func (mgr *CRDNetworkNodeOverlayMgr) Init() {
}

func (mgr *CRDNetworkNodeOverlayMgr) AfterInit() {
	mgr.InitRAMCache()
	go mgr.InitAndRunWatcher()
}

// InitRAMCache create a map for all the entities
func (mgr *CRDNetworkNodeOverlayMgr) InitRAMCache() {
	//mgr.networkNodeOverlayCache = nil // delete old cache for re-init
	//mgr.networkNodeOverlayCache = make(map[string]*NetworkNodeOverlay)
}

// DumpCache logs all the entries in the map
func (mgr *CRDNetworkNodeOverlayMgr) DumpCache() {
	for _, nn := range mgr.networkNodeOverlayCache {
		log.Printf("NetworkNodeOverlay: %s", nn)
	}
}

func (mgr *CRDNetworkNodeOverlayMgr) CrdToSfcNetworkNodeOverlay(crdNNO crd.NetworkNodeOverlay) (sfcNNO controller.NetworkNodeOverlay, err error) {
	sfcNNO = controller.NetworkNodeOverlay{}
	sfcNNO.Metadata = &model.MetaDataType{}
	sfcNNO.Metadata.Name = crdNNO.Name
	sfcNNO.Metadata.Labels = crdNNO.Labels
	sfcNNO.NetworkNodeOverlay.Spec = &crdNNO.NetworkNodeOverlaySpec
	sfcNNO.NetworkNodeOverlay.Status = &crdNNO.NetworkNodeOverlayStatus
	return sfcNNO, nil
}

// HandleCRDSync syncs the SFC Controller with the incoming CRD
func (mgr *CRDNetworkNodeOverlayMgr) HandleCRDSync(crdNNO crd.NetworkNodeOverlay) {
	log.Info("CRDNetworkNodeOverlayMgr HandleCRDSync: enter ...")
	defer log.Info("CRDNetworkNodeOverlayMgr HandleCRDSync: exit ...")

	nno, err := mgr.CrdToSfcNetworkNodeOverlay(crdNNO)
	if err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	opStr := "created"
	if existing, exists := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationR(crdNNO.Name); exists {
		opStr = "updated"
		if existing.ConfigEqual(&nno) {
			log.Infof("crdNNO %s has not changed.", crdNNO.Name)
			return
		}
	}

	if err := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationCU(&nno, false); err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	log.Infof("NetworkNodeOverlay %s", opStr)
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *CRDNetworkNodeOverlayMgr) InitAndRunWatcher() {

	log.Info("CRD NetworkNodeOverlayWatcher: enter ...")
	defer log.Info("CRD NetworkNodeOverlayWatcher: exit ...")

	respChan := make(chan keyval.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(ctlrPlugin.NetworkNodeOverlayMgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("CRD NetworkNodeOverlayWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("CRD NetworkNodeOverlayWatcher: watching the key: %s", ctlrPlugin.NetworkNodeOverlayMgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:

			switch resp.GetChangeType() {
			case datasync.Put:
				dbEntry := &controller.NetworkNodeOverlay{}
				if err := resp.GetValue(dbEntry); err == nil {
					// config and status might have changed ...
					log.Infof("CRD NetworkNodeOverlayWatcher: PUT detected: NetworkNodeOverlay: %s",
						dbEntry)
				}

			case datasync.Delete:
				log.Infof("CRD NetworkNodeOverlayWatcher: deleting key: %s ", resp.GetKey())
				// tell k8s crd that resource has been removed
			}
		}
	}
}
