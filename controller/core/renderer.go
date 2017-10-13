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

// Package core: The renderer uses the config for hosts, external entites,
// and SFC's and invokes the registered CNP driver.  Driver API's are invoked
// for wiring hosts to hosts, hosts to external routers, and the SFC's.
package core

import (
	"github.com/ligato/sfc-controller/controller/model/controller"
	"os"
)

// Render the config: note that because we are wiring everything, we wire only one end when we
// process all hosts -> EEs, as when we process all EE -> Hosts, we will program the other end
// at that time.  BUT, when an individual REST call is made to add a new HOST, we will process
// both ends at that time.  See the http handler for an example of how the renderEE is done.
func (plugin *SfcControllerPluginHandler) renderConfigFromRamCache() error {

	plugin.Log.Infof("render host entities from ram cache")
	for _, he := range plugin.ramConfigCache.HEs {
		if err := plugin.renderHostEntity(&he, true, false); err != nil {
			plugin.Log.Error("Error rendering host entity:", he)
			os.Exit(1)
		}
	}

	plugin.Log.Infof("render external entities from ram cache")
	for _, ee := range plugin.ramConfigCache.EEs {
		if err := plugin.renderExternalEntity(&ee, true, false); err != nil {
			plugin.Log.Error("Error rendering external entity:", ee)
			os.Exit(1)
		}
	}

	for _, he := range plugin.ramConfigCache.HEs {
		if err := plugin.renderHostEntity(&he, false, true); err != nil {
			plugin.Log.Error("Error rendering host entity:", he)
			os.Exit(1)
		}
	}

	plugin.Log.Infof("render external entities from ram cache")
	for _, ee := range plugin.ramConfigCache.EEs {
		if err := plugin.renderExternalEntity(&ee, false, true); err != nil {
			plugin.Log.Error("Error rendering external entity:", ee)
			os.Exit(1)
		}
	}

	plugin.Log.Infof("render sfc's from ram cache")
	for _, sfc := range plugin.ramConfigCache.SFCs {
		if err := plugin.renderServiceFunctionEntity(&sfc); err != nil {
			plugin.Log.Error("Error rendering service function chain:", sfc)
			os.Exit(1)
		}
	}

	plugin.CNPDriver.Dump()

	return nil
}

// For this ee, find all host entities and effect external entity to host wiring.  Will need a "session"
// per external entity, and this session will be used to communicate wiring configuration.  Also, if the
// configOnlyEE is false, then from each host entity, wire from host to this ee.
func (plugin *SfcControllerPluginHandler) renderExternalEntity(ee *controller.ExternalEntity, configOnlyEE bool,
	wireToOtherEntities bool) error {

	plugin.Log.Infof("renderExternalEntity: ee:'%s'/'%s', configOnlyEE=%d, wireToOtherEntities=%d",
		ee.Name, ee.MgmntIpAddress, configOnlyEE, wireToOtherEntities)

	if configOnlyEE {
		plugin.Log.Infof("WireInternalsForExternalEntity: ee:'%s'", ee.Name)
		plugin.CNPDriver.WireInternalsForExternalEntity(ee)

		plugin.ExtEntityDriver.WireInternalsForExternalEntity(ee)
	}

	if wireToOtherEntities {
		for _, he := range plugin.ramConfigCache.HEs {
			plugin.Log.Infof("WireHostEntityToExternalEntity: he:'%s' to ee:'%s'", he.Name, ee.Name)
			plugin.CNPDriver.WireHostEntityToExternalEntity(&he, ee)

			// call the external entity api to queue a msg so that the external router config will be sent to the router
			// this will be replace perhaps by a watcher in the ext-ent driver
			plugin.ExtEntityDriver.WireHostEntityToExternalEntity(&he, ee)
		}
	}

	return nil
}

// For this source host, find other host entities, and effect inter-host wiring, and host-external wiring.
// Also, if configOnlyHE is set, wire from each ee to this new host, and from all other hosts to this new one.
func (plugin *SfcControllerPluginHandler) renderHostEntity(sh *controller.HostEntity, configOnlyHE bool,
	wireToOtherEntities bool) error {

	plugin.Log.Infof("renderHostEntity: sh:'%s', configOnlyFrom=%d, wireToOtherEntities=%d", sh.Name, configOnlyHE,
		wireToOtherEntities)

	if configOnlyHE {
		plugin.Log.Infof("WireInternalsForHostEntity: he:'%s'", sh.Name)
		plugin.CNPDriver.WireInternalsForHostEntity(sh)
	}

	if wireToOtherEntities {
		for _, ee := range plugin.ramConfigCache.EEs {
			plugin.Log.Infof("WireHostEntityToExternalEntity: he:'%s' to ee:'%s'/'%s'",
				sh.Name, ee.Name, ee.MgmntIpAddress)
			plugin.CNPDriver.WireHostEntityToExternalEntity(sh, &ee)
		}

		for _, dh := range plugin.ramConfigCache.HEs {
			if *sh != dh {
				plugin.Log.Infof("WireHostEntityToDestinationHostEntity: sh:'%s' to dh:'%s'",
					sh.Name, dh.Name)
				plugin.CNPDriver.WireHostEntityToDestinationHostEntity(sh, &dh)
				plugin.Log.Infof("WireHostEntityToDestinationHostEntity: dh:'%s' to sh:'%s'",
					dh.Name, sh.Name)
				plugin.CNPDriver.WireHostEntityToDestinationHostEntity(&dh, sh)
			}
		}
	}

	return nil
}

// For each element in the chain, ... render ...
func (plugin *SfcControllerPluginHandler) renderServiceFunctionEntity(sfc *controller.SfcEntity) error {

	plugin.Log.Infof("renderServiceFunctionEntity: sfc:'%s'", sfc.Name)

	numSfcElements := len(sfc.GetElements())
	if numSfcElements == 0 {
		plugin.Log.Warnf("renderServiceFunctionEntity: sfc:'%s' has no elements", sfc.Name)
		return nil
	}

	plugin.Log.Infof("renderServiceFunctionEntity: WireSfcEntities: for '%s'/'%s'",
		sfc.Name, sfc.Description)
	if err := plugin.CNPDriver.WireSfcEntity(sfc); err != nil {
		return err
	}

	return nil
}
