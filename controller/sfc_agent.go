package controller

// Copyright (c) 2017 Cisco and/or its affiliates.
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

import (
	agent_api "github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logmanager"

	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/health/probe"
	"github.com/ligato/sfc-controller/controller/core"
	"github.com/ligato/sfc-controller/plugins/vnfdriver"
	"time"
	"github.com/ligato/sfc-controller/controller/cnpdriver"
	"github.com/ligato/sfc-controller/controller/rpc"
	"github.com/ligato/sfc-controller/controller/extentitydriver"
)

var checkPlugin agent_api.Plugin

// NewAgent returns a new instance of the Agent with plugins.
// <opts> example NewAgent(WithCustomPlugin(func(flavor *FlavorSFCFull) {
//     return []*NamedPlugin{{"MyCustom", MyCustomPlugin{flavor.PluginXY}}
// })
func NewAgent(opts ...agent_api.Option) *agent_api.Agent {
	flavor := &FlavorSFCFull{}
	plugins := flavor.Plugins()

	var agentCoreLogger logging.Logger
	maxStartup := 15 * time.Second

	for _, opt := range opts {
		switch opt.(type) {
		case pluginsLister:
			plugins = append(plugins, opt.(pluginsLister).Plugins(flavor)...)
		case *agent_api.WithTimeoutOpt:
			ms := opt.(*agent_api.WithTimeoutOpt).Timeout
			if ms > 0 {
				maxStartup = ms
			}
		case *agent_api.WithLoggerOpt:
			agentCoreLogger = opt.(*agent_api.WithLoggerOpt).Logger
		}
	}

	if agentCoreLogger == nil {
		agentCoreLogger = flavor.LoggerFor("agentcore")
	}

	return agent_api.NewAgent(agentCoreLogger, maxStartup, plugins...)
}

type pluginsLister interface {
	Plugins(*FlavorSFCFull) []*agent_api.NamedPlugin
}

// WithPluginsOpt is return value of WithPlugins() utility
// to define option of SFC Controller NewAgent().
type WithPluginsOpt struct {
	ListPlugins func(*FlavorSFCFull) []*agent_api.NamedPlugin
}

// WithPlugins for adding custom plugins to SFC Controller
// <ListPlugins> is a callback that uses flavor input to
// inject dependencies for custom plugins that are in output
func WithPlugins(listPlugins func(*FlavorSFCFull) []*agent_api.NamedPlugin) *WithPluginsOpt {
	return &WithPluginsOpt{listPlugins}
}

// OptionMarkerCore is just for marking implementation that it implements this interface
func (marker *WithPluginsOpt) OptionMarkerCore() {}

// FlavorSFCFull is set of common used generic plugins. This flavour can be used as a base
// for different flavours. The plugins are initialized in the same order as they appear
// in the structure.
type FlavorSFCFull struct {
	*local.FlavorLocal
	HTTP      rest.Plugin
	HealthRPC probe.Plugin
	LogMngRPC logmanager.Plugin
	ETCD      etcdv3.Plugin

	VNFDriver    vnfdriver.Plugin
	CNPDDriver   cnpdriver.SfcControllerCNPDriverAPI
	ExtEntDriver cnpdriver.WireExtEntity
	Sfc          core.SfcControllerPluginHandler
	SfcRPC       rpc.SfcControllerRPC

	injected bool
}

// Inject interconnects plugins - injects the dependencies. If it has been called
// already it is no op.
func (f *FlavorSFCFull) Inject() bool {
	if f.injected {
		return false
	}

	if f.FlavorLocal == nil {
		f.FlavorLocal = &local.FlavorLocal{}
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

	f.ETCD.Deps.PluginInfraDeps = *f.InfraDeps("etcdv3", local.WithConf())

	if f.Sfc.Deps.CNPDriver == nil {
		if f.CNPDDriver == nil {
			l2Driver := &cnpdriver.L2Driver{}
			l2Driver.Deps.PluginLogDeps = *f.LogDeps("sfc-l2-plugin")
			l2Driver.Deps.Etcd = &f.ETCD
			f.CNPDDriver = l2Driver
		}

		f.Sfc.Deps.CNPDriver = f.CNPDDriver
	}
	if f.Sfc.Deps.ExtEntityDriver == nil {
		if f.ExtEntDriver == nil {
			f.ExtEntDriver = &extentitydriver.Plugin{}
		}
		f.Sfc.Deps.ExtEntityDriver = f.ExtEntDriver
	}

	checkPlugin = &f.Sfc
	f.Sfc.Deps.PluginInfraDeps = *f.InfraDeps("sfc",
		local.WithConf("Name of a sfc config (yaml) file to load at startup", "sfc.conf"))
	f.Sfc.Deps.Etcd = &f.ETCD

	checkPlugin = &f.SfcRPC
	f.SfcRPC.Deps.HTTP = &f.HTTP
	f.SfcRPC.Deps.PluginLogDeps = *f.LogDeps("sfc-rpc")
	f.SfcRPC.Deps.SFCNorthbound = &f.Sfc

	checkPlugin = &f.VNFDriver
	f.VNFDriver.Etcd = &f.ETCD
	f.VNFDriver.HTTPmux = &f.HTTP

	f.injected = true

	return true
}

// Plugins returns all plugins from the flavour. The set of plugins is supposed
// to be passed to the agent constructor. The method calls inject to make sure that
// dependencies have been injected.
func (f *FlavorSFCFull) Plugins() []*agent_api.NamedPlugin {
	f.Inject()
	return agent_api.ListPluginsInFlavor(f)
}
