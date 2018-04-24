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
)

// PluginID is plugin identifier (must be unique throughout the system)
const PluginID core.PluginName = "k8sCRD"

var (
	k8sCRDConfigFile string // cli flag - see RegisterFlags
	log              = logrus.DefaultLogger()
	ctlrPlugin       *controller.Plugin
	k8scrdPlugin       *Plugin
)

// RegisterFlags add command line flags.
func RegisterFlags() {
	flag.StringVar(&k8sCRDConfigFile, "k8s-crd-config", "",
		"Name of a k8s CRD config file to load at startup")
}

// LogFlags dumps the command line flags
func LogFlags() {
	log.Debugf("LogFlags:")
	log.Debugf("\tk8s crd config:'%s'", k8sCRDConfigFile)
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
	NetworkNodeMgr CRDNetworkNodeMgr
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

	if k8sCRDConfigFile != "" {
		initK8sCRDProcessing(k8sCRDConfigFile)
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

	// open the file, pull out any k8s crd parameters required

	// via system installation, k8s crd model definition's have been registered to kubernetes
	// see k8s_crd_network_node.yaml, etc ...

	// setup watchers for the various controller objects like network_node, network_service
	// use the k8s crd api's to monitor crd->ctlr changes

	// there are ligato watchers setup for each object like network_node in each of the model
	// specific files so if the ctlr makes a change, the change will be monitored so k8scrd
	// can be updated ... see FILE: network_node.go, FUNC: InitAndRunWatcher for details
}