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
	"k8s.io/apimachinery/pkg/util/runtime"
	"fmt"
	"k8s.io/client-go/tools/cache"
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

func (mgr *CRDNetworkNodeMgr) CrdToSfcNetworkNode(crdNN crd.NetworkNode) (sfcNN model.NetworkNode, err error) {
	sfcNN = model.NetworkNode{}
	sfcNN.Metadata = &model.MetaDataType{}
	sfcNN.Metadata.Name = crdNN.Name
	sfcNN.Metadata.Labels = crdNN.Labels
	sfcNN.Spec = &crdNN.NetworkNodeSpec
	sfcNN.Status = &crdNN.NetworkNodeStatus
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
		if ctlrPlugin.NetworkNodeMgr.ConfigEqual(existing, &nn) {
			log.Infof("crdNN %s has not changed.", crdNN.Name)
			return
		}
	}

	ctlrPlugin.AddOperationMsgToQueue(controller.ModelTypeNetworkNode,
		controller.OperationalMsgOpCodeCreateUpdate, &nn)

	log.Infof("NetworkNode %s", opStr)
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *CRDNetworkNodeMgr) InitAndRunWatcher() {

	log.Info("CRD NetworkNodeWatcher: enter ...")
	defer log.Info("CRD NetworkNodeWatcher: exit ...")

	respChan := make(chan datasync.ProtoWatchResp, 0)
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
				dbEntry := &model.NetworkNode{}
				if err := resp.GetValue(dbEntry); err == nil {
					// status might have changed ...
					log.Infof("CRD NetworkNodeWatcher: PUT detected: NetworkNode: %s",
						dbEntry)
					mgr.updateStatus(*dbEntry)
				}

			case datasync.Delete:
				log.Infof("CRD NetworkNodeWatcher: deleting key: %s ", resp.GetKey())
				// tell k8s crd that resource has been removed
			}
		}
	}
}

// updates the CRD status in Kubernetes with the current status from the sfc-controller
func (mgr *CRDNetworkNodeMgr) updateStatus(sfcNetworkNode model.NetworkNode) error {
	// Fetch crdNetworkNode from K8s cache
	// The name in sfc is the namespace/name, which is the "namespace key". Split it out.
	key := sfcNetworkNode.Metadata.Name
	log.Infof("NetworkNode key: %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	if namespace == "" {
		namespace = "default"
	}
	crdNetworkNode, errGet := k8scrdPlugin.CrdController.networkNodesLister.NetworkNodes(namespace).Get(name)
	if errGet != nil {
		log.Errorf("Could not get '%s' with namespace '%s", name, namespace)
		return errGet
	}

	// set status from sfc controller
	if sfcNetworkNode.Status != nil {
		// NEVER modify objects from the store. It's a read-only, local cache.
		// You can use DeepCopy() to make a deep copy of original object and modify this copy
		// Or create a copy manually for better performance
		crdNetworkNodeCopy := crdNetworkNode.DeepCopy()

		crdNetworkNodeCopy.NetworkNodeStatus = *sfcNetworkNode.Status
		log.Infof("NetworkNode Status Msg: %s", crdNetworkNodeCopy.NetworkNodeStatus.Msg)

		// Until #38113 is merged, we must use Update instead of UpdateStatus to
		// update the Status block of the NetworkNode resource. UpdateStatus will not
		// allow changes to the Spec of the resource, which is ideal for ensuring
		// nothing other than resource status has been updated.
		_, errUpdate := k8scrdPlugin.CrdController.sfcclientset.SfccontrollerV1alpha1().NetworkNodes(crdNetworkNodeCopy.Namespace).Update(crdNetworkNodeCopy)
		return errUpdate
	}
	return nil
}