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

// Main file for the SFC controller.  It loads all the plugins.
package main

import (
	agent_api "github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logmanager"
	"github.com/ligato/cn-infra/logging/logrus"

	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/namsral/flag"

	"github.com/ligato/cn-infra/health/probe"
	"github.com/ligato/sfc-controller/plugins/controller"
	"github.com/ligato/sfc-controller/plugins/k8scrd"
)

var log = logrus.DefaultLogger()

// Init is the Go init() function for the plugin. It should
// contain the boiler plate initialization code that is executed
// when the plugin is loaded into the Agent.
func init() {
	flag.String("etcdv3-config", "etcd.conf",
		"Location of the Etcd configuration file; also set via 'ETCDV3_CONFIG' env variable.")

	log.SetLevel(logging.DebugLevel)
	//TODO with Lukas pluginapi.RegisterLogger(PluginID, log.StandardLogger())
}

// Flavor is set of common used generic plugins. This flavour can be used as a base
// for different flavours. The plugins are initialized in the same order as they appear
// in the structure.
type Flavor struct {
	local.FlavorLocal
	HTTP       rest.Plugin
	HealthRPC  probe.Plugin
	LogMngRPC  logmanager.Plugin
	ETCD       etcdv3.Plugin
	Controller controller.Plugin
	K8sCRD     k8scrd.Plugin

	injected bool
}

// Inject interconnects plugins - injects the dependencies. If it has been called
// already it is no op.
func (f *Flavor) Inject() bool {
	if f.injected {
		return false
	}

	f.FlavorLocal.Inject()

	httpInfraDeps := f.InfraDeps("http", local.WithConf())
	f.HTTP.Deps.Log = httpInfraDeps.Log
	f.HTTP.Deps.PluginName = httpInfraDeps.PluginName
	f.HTTP.Deps.PluginConfig = httpInfraDeps.PluginConfig

	logMngInfraDeps := f.InfraDeps("log-mng-rpc")
	f.LogMngRPC.Deps.Log = logMngInfraDeps.Log
	f.LogMngRPC.Deps.PluginName = logMngInfraDeps.PluginName
	f.LogMngRPC.Deps.PluginConfig = logMngInfraDeps.PluginConfig
	f.LogMngRPC.LogRegistry = f.FlavorLocal.LogRegistry()
	f.LogMngRPC.HTTP = &f.HTTP

	f.HealthRPC.Deps.PluginLogDeps = *f.LogDeps("health-rpc")
	f.HealthRPC.Deps.HTTP = &f.HTTP
	f.HealthRPC.Deps.StatusCheck = &f.StatusCheck

	f.ETCD.Deps.PluginInfraDeps = *f.InfraDeps("etcdv3")

	f.Controller.Etcd = &f.ETCD
	f.Controller.HTTPmux = &f.HTTP
	f.Controller.FlavorLocal = &f.FlavorLocal

	f.K8sCRD.Etcd = &f.ETCD
	f.K8sCRD.HTTPmux = &f.HTTP
	f.K8sCRD.FlavorLocal = &f.FlavorLocal
	f.K8sCRD.Controller = &f.Controller

	f.injected = true

	return true
}

// Plugins returns all plugins from the flavour. The set of plugins is supposed
// to be passed to the agent constructor. The method calls inject to make sure that
// dependencies have been injected.
func (f *Flavor) Plugins() []*agent_api.NamedPlugin {
	f.Inject()
	log.Info("LISTPLUGINS: ", agent_api.ListPluginsInFlavor(f))
	return agent_api.ListPluginsInFlavor(f)
}

func main() {

	f := Flavor{}
	//agent := agent_api.NewAgent(log, 15*time.Second, f.Plugins()...)
	agent := agent_api.NewAgent(&f)
	agent_api.EventLoopWithInterrupt(agent, nil)
}
