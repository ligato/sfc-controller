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
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logmanager"
	log "github.com/ligato/cn-infra/logging/logrus"

	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/namsral/flag"

	"os"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/health/probe"

	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/health/statuscheck"
	sfc "github.com/ligato/sfc-controller/plugins/controller"
)

// Init is the Go init() function for the plugin. It should
// contain the boiler plate initialization code that is executed
// when the plugin is loaded into the Agent.
func init() {
	flag.String("etcdv3-config", "etcd.conf",
		"Location of the Etcd configuration file; also set via 'ETCDV3_CONFIG' env variable.")

	log.DefaultLogger().SetOutput(os.Stdout)
	log.DefaultLogger().SetLevel(logging.DebugLevel)
}

// Flavor is set of common used generic plugins. This flavour can be used as a base
// for different flavours. The plugins are initialized in the same order as they appear
// in the structure.
type SfcController struct {
	LogManager  *logmanager.Plugin
	HTTP        *rest.Plugin
	HealthProbe *probe.Plugin
	ETCD        *etcd.Plugin

	Sfc *sfc.Plugin
	//Crd *crd.Plugin
}

func (SfcController) String() string {
	return "SfcController"
}

// Init is called in startup phase. Method added in order to implement Plugin interface.
func (SfcController) Init() error {
	return nil
}

// AfterInit triggers the first resync.
func (SfcController) AfterInit() error {
	return nil
}

// Close is called in agent's cleanup phase. Method added in order to implement Plugin interface.
func (SfcController) Close() error {
	return nil
}

func main() {

	log.DefaultLogger().SetLevel(logging.DebugLevel)

	sfcPlugin := sfc.NewPlugin(
		sfc.UseDeps(func(deps *sfc.Deps) {
			deps.HTTPHandlers = &rest.DefaultPlugin
			deps.Etcd = &etcd.DefaultPlugin
			deps.StatusCheck = &statuscheck.DefaultPlugin
		}),
	)

	//crdPlugin := crd.NewPlugin(
	//	crd.UseDeps(func(deps *crd.Deps) {
	//		deps.HTTPHandlers = &rest.DefaultPlugin
	//		deps.Etcd = &etcd.DefaultPlugin
	//		deps.Controller = sfcPlugin
	//		deps.StatusCheck = &statuscheck.DefaultPlugin
	//	}),
	//)

	sfcAgent := &SfcController{
		LogManager:  &logmanager.DefaultPlugin,
		HTTP:        &rest.DefaultPlugin,
		HealthProbe: &probe.DefaultPlugin,
		ETCD:        &etcd.DefaultPlugin,
		Sfc:         sfcPlugin,
		//Crd:         crdPlugin,
	}

	a := agent.NewAgent(agent.AllPlugins(sfcAgent), agent.StartTimeout(getStartupTimeout()))
	if err := a.Run(); err != nil {
		log.DefaultLogger().Fatal(err)
	}

}

func getStartupTimeout() time.Duration {
	var err error
	var timeout time.Duration

	// valid env value must conform to duration format
	// e.g: 45s
	envVal := os.Getenv("STARTUPTIMEOUT")

	if timeout, err = time.ParseDuration(envVal); err != nil {
		timeout = 45 * time.Second
	}

	return timeout
}
