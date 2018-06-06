// Copyright 2017 The Kubernetes Authors.
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
"fmt"
"time"

corev1 "k8s.io/api/core/v1"
"k8s.io/apimachinery/pkg/api/errors"
"k8s.io/apimachinery/pkg/util/runtime"
"k8s.io/apimachinery/pkg/util/wait"
kubeinformers "k8s.io/client-go/informers"
"k8s.io/client-go/kubernetes"
"k8s.io/client-go/kubernetes/scheme"
typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
"k8s.io/client-go/tools/cache"
"k8s.io/client-go/tools/record"
"k8s.io/client-go/util/workqueue"

clientset "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client/clientset/versioned"
sfcscheme "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client/clientset/versioned/scheme"
informers "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client/informers/externalversions"
listers "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client/listers/sfccontroller/v1alpha1"
)

const controllerAgentName = "sfc-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a NetworkNode is synced
	SuccessSynced = "Synced"
	// MessageResourceSynced is the message used for an Event fired when a NetworkNode
	// is synced successfully
	MessageResourceSynced = "NetworkNode synced successfully"
)

// Controller is the controller implementation for NetworkNode resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sfcclientset is a clientset for our own API group
	sfcclientset clientset.Interface

	networkNodesLister        listers.NetworkNodeLister
	networkNodesSynced        cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sfc crd controller
func NewController(
	kubeclientset kubernetes.Interface,
	sfcclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	sfcInformerFactory informers.SharedInformerFactory) *Controller {

	// obtain references to shared index informers for the SFC Controller types.
	networkNodeInformer := sfcInformerFactory.Sfccontroller().V1alpha1().NetworkNodes()

	// Create event broadcaster
	// Add sfc-controller types to the default Kubernetes Scheme so Events can be
	// logged for sfc-controller types.
	sfcscheme.AddToScheme(scheme.Scheme)
	log.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		sfcclientset:   sfcclientset,
		networkNodesLister:       networkNodeInformer.Lister(),
		networkNodesSynced:       networkNodeInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NetworkNodes"),
		recorder:          recorder,
	}

	log.Info("Setting up event handlers")
	// Set up an event handler for when NetworkNode resources change
	networkNodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueNetworkNode,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueNetworkNode(new)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Info("Starting NetworkNode controller")

	// Wait for the caches to be synced before starting workers
	log.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.networkNodesSynced); !ok {
		log.Fatalf("Error running controller: %s", "failed to wait for caches to sync")
		return
	}

	log.Info("Starting workers")
	// Launch two workers to process NetworkNode resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// NetworkNode resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		log.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the NetworkNode resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the NetworkNode resource with this namespace/name
	networkNode, err := c.networkNodesLister.NetworkNodes(namespace).Get(name)
	if err != nil {
		// The NetworkNode resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("networkNode '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

    // Handle CRD Sync with SFC Controller data
	k8scrdPlugin.NetworkNodeMgr.HandleCRDSync(*networkNode)

	// Update status block of NetworkNode - TODO: should be some pending status, final status is after rendering
	//err = c.updateNetworkNodeStatus(networkNode, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(networkNode, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

//func (c *Controller) updateNetworkNodeStatus(networkNode *sfcv1alpha1.NetworkNode, deployment *appsv1.Deployment) error {
//	// NEVER modify objects from the store. It's a read-only, local cache.
//	// You can use DeepCopy() to make a deep copy of original object and modify this copy
//	// Or create a copy manually for better performance
//	networkNodeCopy := networkNode.DeepCopy()
//	networkNodeCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
//	// Until #38113 is merged, we must use Update instead of UpdateStatus to
//	// update the Status block of the NetworkNode resource. UpdateStatus will not
//	// allow changes to the Spec of the resource, which is ideal for ensuring
//	// nothing other than resource status has been updated.
//	_, err := c.sfcclientset.SfccontrollerV1alpha1().NetworkNodes(networkNode.Namespace).Update(networkNodeCopy)
//	return err
//}

// enqueueNetworkNode takes a NetworkNode resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than NetworkNode.
func (c *Controller) enqueueNetworkNode(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}
