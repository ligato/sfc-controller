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
	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/util/runtime"
	"fmt"
)

type CRDNetworkServiceMgr struct {
	NetworkServiceCache map[string]string
}

func (mgr *CRDNetworkServiceMgr) Init() {
}

func (mgr *CRDNetworkServiceMgr) AfterInit() {
	mgr.InitRAMCache()
	go mgr.InitAndRunWatcher()
}

// InitRAMCache create a map for all the entities
func (mgr *CRDNetworkServiceMgr) InitRAMCache() {
	//mgr.NetworkServiceCache = nil // delete old cache for re-init
	//mgr.NetworkServiceCache = make(map[string]*NetworkService)
}

// DumpCache logs all the entries in the map
func (mgr *CRDNetworkServiceMgr) DumpCache() {
	for _, nn := range mgr.NetworkServiceCache {
		log.Printf("NetworkService: %s", nn)
	}
}

func (mgr *CRDNetworkServiceMgr) CrdToSfcNetworkService(crdNS crd.NetworkService) (sfcNS model.NetworkService, err error) {
	sfcNS = model.NetworkService{}
	sfcNS.Metadata = &model.MetaDataType{}
	sfcNS.Metadata.Name = crdNS.Name
	sfcNS.Metadata.Labels = crdNS.Labels
	sfcNS.Spec = &crdNS.NetworkServiceSpec
	sfcNS.Status = &crdNS.NetworkServiceStatus
	return sfcNS, nil
}

// HandleCRDSync syncs the SFC Controller with the incoming CRD
func (mgr *CRDNetworkServiceMgr) HandleCRDSync(crdNS crd.NetworkService) {
	log.Info("CRDNetworkServiceMgr HandleCRDSync: enter ...")
	defer log.Info("CRDNetworkServiceMgr HandleCRDSync: exit ...")

	ns, err := mgr.CrdToSfcNetworkService(crdNS)
	if err != nil {
		log.Errorf("%s", err.Error())
		return
	}

	opStr := "created"
	if existing, exists := ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationR(crdNS.Name); exists {
		opStr = "updated"
		if ctlrPlugin.NetworkServiceMgr.ConfigEqual(existing, &ns) {
			log.Infof("crdNN %s has not changed.", crdNS.Name)
			return
		}
	}

	ctlrPlugin.AddOperationMsgToQueue(controller.ModelTypeNetworkService,
		controller.OperationalMsgOpCodeCreateUpdate, &ns)

	log.Infof("NetworkService %s", opStr)
}

// InitAndRunWatcher enables etcd updates to be monitored
func (mgr *CRDNetworkServiceMgr) InitAndRunWatcher() {

	log.Info("CRD NetworkServiceWatcher: enter ...")
	defer log.Info("CRD NetworkServiceWatcher: exit ...")

	respChan := make(chan datasync.ProtoWatchResp, 0)
	watcher := ctlrPlugin.Etcd.NewWatcher(ctlrPlugin.NetworkServiceMgr.KeyPrefix())
	err := watcher.Watch(keyval.ToChanProto(respChan), make(chan string), "")
	if err != nil {
		log.Errorf("CRD NetworkServiceWatcher: cannot watch: %s", err)
		os.Exit(1)
	}
	log.Debugf("CRD NetworkServiceWatcher: watching the key: %s", ctlrPlugin.NetworkServiceMgr.KeyPrefix())

	for {
		select {
		case resp := <-respChan:

			switch resp.GetChangeType() {
			case datasync.Put:
				dbEntry := &model.NetworkService{}
				if err := resp.GetValue(dbEntry); err == nil {
					// config and status might have changed ...
					log.Infof("CRD NetworkServiceWatcher: PUT detected: NetworkService: %s",
						dbEntry)
					mgr.updateStatus(*dbEntry)
				}

			case datasync.Delete:
				log.Infof("CRD NetworkServiceWatcher: deleting key: %s ", resp.GetKey())
				// tell k8s crd that resource has been removed
			}
		}
	}
}

// updates the CRD status in Kubernetes with the current status from the sfc-controller
func (mgr *CRDNetworkServiceMgr) updateStatus(sfcNetworkService model.NetworkService) error {
	// Fetch crdNetworkService from K8s cache
	// The name in sfc is the namespace/name, which is the "namespace key". Split it out.
	key := sfcNetworkService.Metadata.Name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	if namespace == "" {
		namespace = "default"
	}
	crdNetworkService, errGet := k8scrdPlugin.CrdController.networkServicesLister.NetworkServices(namespace).Get(name)
	if errGet != nil {
		log.Errorf("Could not get '%s' with namespace '%s", name, namespace)
		return errGet
	}
	// set status from sfc controller
	if sfcNetworkService.Status != nil {
		// NEVER modify objects from the store. It's a read-only, local cache.
		// You can use DeepCopy() to make a deep copy of original object and modify this copy
		// Or create a copy manually for better performance
		crdNetworkServiceCopy := crdNetworkService.DeepCopy()

		crdNetworkServiceCopy.NetworkServiceStatus = *sfcNetworkService.Status
		log.Infof("NetworkNode Status Msg: %s", crdNetworkServiceCopy.NetworkServiceStatus.Msg)

		// Until #38113 is merged, we must use Update instead of UpdateStatus to
		// update the Status block of the NetworkService resource. UpdateStatus will not
		// allow changes to the Spec of the resource, which is ideal for ensuring
		// nothing other than resource status has been updated.
		_, errUpdate := k8scrdPlugin.CrdController.sfcclientset.SfccontrollerV1alpha1().NetworkServices(crdNetworkServiceCopy.Namespace).Update(crdNetworkServiceCopy)
		return errUpdate
	}
	return nil
}