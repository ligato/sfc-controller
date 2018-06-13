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

// Controller is the controller implementation for our custom resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sfcclientset is a clientset for our own API group
	sfcclientset clientset.Interface

	ipamPoolsLister listers.IpamPoolLister
	ipamPoolsSynced cache.InformerSynced
	networkNodesLister listers.NetworkNodeLister
	networkNodesSynced cache.InformerSynced
	networkNodeOverlaysLister listers.NetworkNodeOverlayLister
	networkNodeOverlaysSynced cache.InformerSynced
	networkServicesLister listers.NetworkServiceLister
	networkServicesSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	ipamPoolsWorkqueue workqueue.RateLimitingInterface
	networkNodesWorkqueue workqueue.RateLimitingInterface
	networkNodeOverlaysWorkqueue workqueue.RateLimitingInterface
	networkServicesWorkqueue workqueue.RateLimitingInterface
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
	ipamPoolInformer := sfcInformerFactory.Sfccontroller().V1alpha1().IpamPools()
	networkNodeInformer := sfcInformerFactory.Sfccontroller().V1alpha1().NetworkNodes()
	networkNodeOverlayInformer := sfcInformerFactory.Sfccontroller().V1alpha1().NetworkNodeOverlays()
	networkServiceInformer := sfcInformerFactory.Sfccontroller().V1alpha1().NetworkServices()

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
		ipamPoolsLister: ipamPoolInformer.Lister(),
		ipamPoolsSynced: ipamPoolInformer.Informer().HasSynced,
		ipamPoolsWorkqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "IpamPools"),
		networkNodesLister:       networkNodeInformer.Lister(),
		networkNodesSynced:       networkNodeInformer.Informer().HasSynced,
		networkNodesWorkqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NetworkNodes"),
		networkNodeOverlaysLister:       networkNodeOverlayInformer.Lister(),
		networkNodeOverlaysSynced:       networkNodeOverlayInformer.Informer().HasSynced,
		networkNodeOverlaysWorkqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NetworkNodeOverlays"),
		networkServicesLister:       networkServiceInformer.Lister(),
		networkServicesSynced:       networkServiceInformer.Informer().HasSynced,
		networkServicesWorkqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NetworkServices"),
		recorder:          recorder,
	}

	log.Info("Setting up event handlers")
	// Set up an event handler for when resources change

	ipamPoolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueIpamPool,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueIpamPool(new)
		},
	})
	networkNodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueNetworkNode,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueNetworkNode(new)
		},
	})
	networkNodeOverlayInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueNetworkNodeOverlay,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueNetworkNodeOverlay(new)
		},
	})
	networkServiceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueNetworkService,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueNetworkService(new)
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
	defer c.ipamPoolsWorkqueue.ShutDown()
	defer c.networkNodesWorkqueue.ShutDown()
	defer c.networkNodeOverlaysWorkqueue.ShutDown()
	defer c.networkServicesWorkqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Info("Starting CRD controller")

	// Wait for the caches to be synced before starting workers
	log.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.ipamPoolsSynced, c.networkNodesSynced, c.networkNodeOverlaysSynced, c.networkServicesSynced); !ok {
		log.Fatalf("Error running controller: %s", "failed to wait for caches to sync")
		return
	}

	log.Info("Starting workers")
	// Launch two workers to process each type of resource
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runIpamPoolsWorker, time.Second, stopCh)
		go wait.Until(c.runNetworkNodesWorker, time.Second, stopCh)
		go wait.Until(c.runNetworkNodeOverlaysWorker, time.Second, stopCh)
		go wait.Until(c.runNetworkServicesWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runIpamPoolsWorker() {
	for c.processNextWorkItem(c.ipamPoolsWorkqueue, c.syncHandlerIpamPool) {
	}
}
func (c *Controller) runNetworkNodesWorker() {
	for c.processNextWorkItem(c.networkNodesWorkqueue, c.syncHandlerNetworkNode) {
	}
}
func (c *Controller) runNetworkNodeOverlaysWorker() {
	for c.processNextWorkItem(c.networkNodeOverlaysWorkqueue, c.syncHandlerNetworkNodeOverlay) {
	}
}
func (c *Controller) runNetworkServicesWorker() {
	for c.processNextWorkItem(c.networkServicesWorkqueue, c.syncHandlerNetworkService) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(queue workqueue.RateLimitingInterface, syncFunc func(string) error) bool {
	obj, shutdown := queue.Get()

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
		defer queue.Done(obj)
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
			queue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// resource to be synced.
		if err := syncFunc(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		queue.Forget(obj)
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
// converge the two. It then updates the Status block of the IpamPool resource
// with the current status of the resource.
func (c *Controller) syncHandlerIpamPool(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the IpamPool resource with this namespace/name
	ipamPool, err := c.ipamPoolsLister.IpamPools(namespace).Get(name)
	if err != nil {
		// The IpamPool resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("ipamPool '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Handle CRD Sync with SFC Controller data
	k8scrdPlugin.IpamPoolMgr.HandleCRDSync(*ipamPool)

	// Update status block of IpamPool - TODO: should be some pending status, final status is after rendering
	//err = c.updateIpamPoolStatus(ipamPool, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(ipamPool, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the NetworkNode resource
// with the current status of the resource.
func (c *Controller) syncHandlerNetworkNode(key string) error {
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

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the NetworkNodeOverlay resource
// with the current status of the resource.
func (c *Controller) syncHandlerNetworkNodeOverlay(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the NetworkNodeOverlay resource with this namespace/name
	networkNodeOverlay, err := c.networkNodeOverlaysLister.NetworkNodeOverlays(namespace).Get(name)
	if err != nil {
		// The NetworkNodeOverlay resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("networkNodeOverlay '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Handle CRD Sync with SFC Controller data
	k8scrdPlugin.NetworkNodeOverlayMgr.HandleCRDSync(*networkNodeOverlay)

	// Update status block of NetworkNodeOverlay - TODO: should be some pending status, final status is after rendering
	//err = c.updateNetworkNodeOverlayStatus(networkNodeOverlay, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(networkNodeOverlay, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the NetworkService resource
// with the current status of the resource.
func (c *Controller) syncHandlerNetworkService(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the NetworkService resource with this namespace/name
	networkService, err := c.networkServicesLister.NetworkServices(namespace).Get(name)
	if err != nil {
		// The NetworkService resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("networkService '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

    // Handle CRD Sync with SFC Controller data
	k8scrdPlugin.NetworkServiceMgr.HandleCRDSync(*networkService)

	// Update status block of NetworkService - TODO: should be some pending status, final status is after rendering
	//err = c.updateNetworkServiceStatus(networkService, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(networkService, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
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
func (c *Controller) enqueueIpamPool(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.ipamPoolsWorkqueue.AddRateLimited(key)
}

func (c *Controller) enqueueNetworkNode(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.networkNodesWorkqueue.AddRateLimited(key)
}

func (c *Controller) enqueueNetworkNodeOverlay(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.networkNodeOverlaysWorkqueue.AddRateLimited(key)
}

func (c *Controller) enqueueNetworkService(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.networkServicesWorkqueue.AddRateLimited(key)
}
