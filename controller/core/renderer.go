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

// Package core : The renderer uses the config for hosts, external entites,
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
func (sfcCtrlPlugin *SfcControllerPluginHandler) renderConfigFromRAMCache() error {


	log.Infof("render system parameters from ram cache")
	if err := sfcCtrlPlugin.renderSystemParameters(&sfcCtrlPlugin.ramConfigCache.SysParms); err != nil {
		log.Error("Error rendering sys parms:", sfcCtrlPlugin.ramConfigCache.SysParms)
		os.Exit(1)
	}

	log.Infof("render host entities from ram cache")
	for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
		if err := sfcCtrlPlugin.renderHostEntity(&he, true, false); err != nil {
			log.Error("Error rendering host entity:", he)
			os.Exit(1)
		}
	}

	log.Infof("render external entities from ram cache")
	for _, ee := range sfcCtrlPlugin.ramConfigCache.EEs {
		if err := sfcCtrlPlugin.renderExternalEntity(&ee, true, false); err != nil {
			log.Error("Error rendering external entity:", ee)
			os.Exit(1)
		}
	}

	for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
		if err := sfcCtrlPlugin.renderHostEntity(&he, false, true); err != nil {
			log.Error("Error rendering host entity:", he)
			os.Exit(1)
		}
	}

	log.Infof("render external entities from ram cache")
	for _, ee := range sfcCtrlPlugin.ramConfigCache.EEs {
		if err := sfcCtrlPlugin.renderExternalEntity(&ee, false, true); err != nil {
			log.Error("Error rendering external entity:", ee)
			os.Exit(1)
		}
	}

	log.Infof("render sfc's from ram cache")
	for _, sfc := range sfcCtrlPlugin.ramConfigCache.SFCs {
		if err := sfcCtrlPlugin.renderServiceFunctionEntity(&sfc); err != nil {
			log.Error("Error rendering service function chain:", sfc)
			os.Exit(1)
		}
	}

	sfcCtrlPlugin.cnpDriverPlugin.Dump()

	return nil
}

func (sfcCtrlPlugin *SfcControllerPluginHandler) renderSystemParameters(sp *controller.SystemParameters) error {

	log.Infof("renderSystemParameters: sp: ", *sp)

	return sfcCtrlPlugin.cnpDriverPlugin.SetSystemParameters(sp)

}

// For this ee, find all host entities and effect external entity to host wiring.  Will need a "session"
// per external entity, and this session will be used to communicate wiring configuration.  Also, if the
// configOnlyEE is false, then from each host entity, wire from host to this ee.
func (sfcCtrlPlugin *SfcControllerPluginHandler) renderExternalEntity(ee *controller.ExternalEntity, configOnlyEE bool,
	wireToOtherEntities bool) error {

	log.Infof("renderExternalEntity: ee:'%s'/'%s', configOnlyEE=%d, wireToOtherEntities=%d",
		ee.Name, ee.MgmntIpAddress, configOnlyEE, wireToOtherEntities)

	if configOnlyEE {
		log.Infof("WireInternalsForExternalEntity: ee:'%s'", ee.Name)
		sfcCtrlPlugin.cnpDriverPlugin.WireInternalsForExternalEntity(ee)
	}

	if wireToOtherEntities {
		for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
			log.Infof("WireHostEntityToExternalEntity: he:'%s' to ee:'%s'", he.Name, ee.Name)
			sfcCtrlPlugin.cnpDriverPlugin.WireHostEntityToExternalEntity(&he, ee)
		}
	}

	return nil
}

// For this source host, find other host entities, and effect inter-host wiring, and host-external wiring.
// Also, if configOnlyHE is set, wire from each ee to this new host, and from all other hosts to this new one.
func (sfcCtrlPlugin *SfcControllerPluginHandler) renderHostEntity(sh *controller.HostEntity, configOnlyHE bool,
	wireToOtherEntities bool) error {

	log.Infof("renderHostEntity: sh:'%s', configOnlyFrom=%d, wireToOtherEntities=%d", sh.Name, configOnlyHE,
		wireToOtherEntities)

	if configOnlyHE {
		log.Infof("WireInternalsForHostEntity: he:'%s'", sh.Name)
		sfcCtrlPlugin.cnpDriverPlugin.WireInternalsForHostEntity(sh)
	}

	if wireToOtherEntities {
		for _, ee := range sfcCtrlPlugin.ramConfigCache.EEs {
			log.Infof("WireHostEntityToExternalEntity: he:'%s' to ee:'%s'/'%s'",
				sh.Name, ee.Name, ee.MgmntIpAddress)
			sfcCtrlPlugin.cnpDriverPlugin.WireHostEntityToExternalEntity(sh, &ee)
		}

		for _, dh := range sfcCtrlPlugin.ramConfigCache.HEs {
			if *sh != dh {
				log.Infof("WireHostEntityToDestinationHostEntity: sh:'%s' to dh:'%s'",
					sh.Name, dh.Name)
				sfcCtrlPlugin.cnpDriverPlugin.WireHostEntityToDestinationHostEntity(sh, &dh)
				log.Infof("WireHostEntityToDestinationHostEntity: dh:'%s' to sh:'%s'",
					dh.Name, sh.Name)
				sfcCtrlPlugin.cnpDriverPlugin.WireHostEntityToDestinationHostEntity(&dh, sh)
			}
		}
	}

	return nil
}

// For each element in the chain, ... render ...
func (sfcCtrlPlugin *SfcControllerPluginHandler) renderServiceFunctionEntity(sfc *controller.SfcEntity) error {

	log.Infof("renderServiceFunctionEntity: sfc:'%s'", sfc.Name)

	numSfcElements := len(sfc.GetElements())
	if numSfcElements == 0 {
		log.Warnf("renderServiceFunctionEntity: sfc:'%s' has no elements", sfc.Name)
		return nil
	}

	log.Infof("renderServiceFunctionEntity: WireSfcEntities: for '%s'/'%s'",
		sfc.Name, sfc.Description)
	if err := sfcCtrlPlugin.cnpDriverPlugin.WireSfcEntity(sfc); err != nil {
		return err
	}

	return nil
}
