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
	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/rpc/rest"

	"github.com/ligato/sfc-controller/plugins/controller"
	"github.com/namsral/flag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	"time"
	kubeinformers "k8s.io/client-go/informers"
	clientset "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client/clientset/versioned"
	informers "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/client/informers/externalversions"
	"github.com/ligato/sfc-controller/plugins/k8scrd/pkg/signals"
)

// PluginID is plugin identifier (must be unique throughout the system)
const PluginID core.PluginName = "k8sCRD"

var (
	kubeconfig   string // cli flag - see RegisterFlags
	log                        = logrus.DefaultLogger()
	ctlrPlugin   *controller.Plugin
	k8scrdPlugin *Plugin
)

// RegisterFlags add command line flags.
func RegisterFlags() {
	flag.StringVar(&kubeconfig, "kubeconfig", "",
		"Name of a k8s CRD kubeconfig file to load at startup")
}

// LogFlags dumps the command line flags
func LogFlags() {
	log.Debugf("LogFlags:")
	log.Debugf("\tk8s crd kubeconfig:'%s'", kubeconfig)
}

// Init is the Go init() function for the s. It should
// contain the boiler plate initialization code that is executed
// when the plugin is loaded into the Agent.
func init() {
	// Logger must be initialized for each s individually.
	//log.SetLevel(logging.DebugLevel)
	log.SetLevel(logging.InfoLevel)

	RegisterFlags()
}

// CacheType is ram cache of controller entities indexed by entity name
type CacheType struct {
	// state
}

// Plugin contains the controllers information
type Plugin struct {
	Etcd    *etcdv3.Plugin
	HTTPmux *rest.Plugin
	*local.FlavorLocal
	Controller     *controller.Plugin
	ramConfigCache CacheType
	IpamPoolMgr CRDIpamPoolMgr
	NetworkNodeMgr CRDNetworkNodeMgr
	NetworkNodeOverlayMgr CRDNetworkNodeOverlayMgr
	NetworkServiceMgr CRDNetworkServiceMgr
}

// Init the controller, read the db, reconcile/resync, render config to etcd
func (s *Plugin) Init() error {

	k8scrdPlugin = s
	ctlrPlugin = s.Controller

	log.Infof("Init: %s enter ...", PluginID)
	defer log.Infof("Init: %s exit ", PluginID)

	// Flag variables registered in init() are ready to use in InitPlugin()
	LogFlags()

	// Register providing status reports (push mode)
	s.StatusCheck.Register(PluginID, nil)
	s.StatusCheck.ReportStateChange(PluginID, statuscheck.Init, nil)

	s.RegisterModelTypeManagers()

	s.initMgrs()

	if kubeconfig != "" {
		initK8sCRDProcessing(kubeconfig)
	}

	return nil
}

func (s *Plugin) initMgrs() {
	for _, entry := range RegisteredManagers {
		log.Infof("initMgrs: initing %s ...", entry.modelTypeName)
		entry.mgr.Init()
	}
}

func (s *Plugin) afterInitMgrs() {
	for _, entry := range RegisteredManagers {
		log.Infof("afterInitMgrs: after initing %s ...", entry.modelTypeName)
		entry.mgr.AfterInit()
	}
}

// AfterInit is called after all plugin are init-ed
func (s *Plugin) AfterInit() error {
	log.Info("AfterInit:", PluginID)

	// at this point, plugins are all loaded

	s.afterInitMgrs()

	s.StatusCheck.ReportStateChange(PluginID, statuscheck.OK, nil)

	return nil
}

// Close performs close down procedures
func (s *Plugin) Close() error {
	return nil
}

func initK8sCRDProcessing(k8sCRDConfigFile string) {

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s, kubeconfig = '%s'", err.Error(), kubeconfig)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	sfcKubeClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building sfcKube clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	sfcKubeInformerFactory := informers.NewSharedInformerFactory(sfcKubeClient, time.Second*30)

	crdController := NewController(kubeClient, sfcKubeClient, kubeInformerFactory, sfcKubeInformerFactory)

	go kubeInformerFactory.Start(stopCh)
	go sfcKubeInformerFactory.Start(stopCh)

	go crdController.Run(2, stopCh)
}