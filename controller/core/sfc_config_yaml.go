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

// This config yaml file is loaded into a data structure and pulled in by the
// controller.

package core

import (
	"github.com/ghodss/yaml"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"io/ioutil"
)

// YamlConfig is container struct for yaml config file
type YamlConfig struct {
	Version     int                           `json:"sfc_controller_config_version"`
	Description string                        `json:"description"`
	EEs         []controller.ExternalEntity   `json:"external_entities"`
	HEs         []controller.HostEntity       `json:"host_entities"`
	SFCs        []controller.SfcEntity        `json:"sfc_entities"`
	SysParms    *controller.SystemParameters   `json:"system_parameters"`
}

// open the file and parse the yaml into the json datastructure
func (sfcCtrlPlugin *SfcControllerPluginHandler) readConfigFromFile(fpath string) error {

	log.Debugf("fpath of sfc-config: '%s'", fpath)

	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}

	sfcCtrlPlugin.yamlConfig = &YamlConfig{}

	if err := yaml.Unmarshal(b, sfcCtrlPlugin.yamlConfig); err != nil {
		return err
	}

	log.Debugf("sfc-config: '%s'", sfcCtrlPlugin.yamlConfig)

	return nil
}

// read external, hosts and chains, and render config via CNP and EE drivers
func (sfcCtrlPlugin *SfcControllerPluginHandler) copyYamlConfigToRAMCache() error {

	if sfcCtrlPlugin.yamlConfig.SysParms != nil {
		sfcCtrlPlugin.ramConfigCache.SysParms = *sfcCtrlPlugin.yamlConfig.SysParms
		log.Debugf("copyYamlConfigToRAMCache: sp: ", sfcCtrlPlugin.yamlConfig.SysParms)
	}

	for _, ee := range sfcCtrlPlugin.yamlConfig.EEs {
		sfcCtrlPlugin.ramConfigCache.EEs[ee.Name] = ee
		log.Debugf("copyYamlConfigToRAMCache: ee: ", ee)
	}
	for _, he := range sfcCtrlPlugin.yamlConfig.HEs {
		sfcCtrlPlugin.ramConfigCache.HEs[he.Name] = he
		log.Debugf("copyYamlConfigToRAMCache: he: ", he)
	}
	for _, sfc := range sfcCtrlPlugin.yamlConfig.SFCs {
		sfcCtrlPlugin.ramConfigCache.SFCs[sfc.Name] = sfc
		log.Debugf("copyYamlConfigToRAMCache: sfc: ", sfc)
		log.Debugf("copyYamlConfigToRAMCache: num_chain_elements=%d", len(sfc.GetElements()))
		for i, sfcChainElement := range sfc.GetElements() {
			log.Debugf("copyYamlConfigToRAMCache: sfc_chain_element[%d]=", i, sfcChainElement)
		}
	}

	return nil
}
