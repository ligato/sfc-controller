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
	"github.com/ligato/sfc-controller/controller/model/controller"
)

type YamlConfig struct {
	Version     string                      `json:"sfc_controller_config_version"`
	Description string                      `json:"description"`
	EEs         []controller.ExternalEntity `json:"external_entities"`
	HEs         []controller.HostEntity     `json:"host_entities"`
	SFCs        []controller.SfcEntity      `json:"sfc_entities"`
}

// open the file and parse the yaml into the go datastructure YamlConfig
func (plugin *SfcControllerPluginHandler) readConfigFromFile() (
	cfc *YamlConfig, found bool, err error) {

	plugin.Log.Debugf("sfc-config loading: '%s'", plugin.PluginConfig.GetConfigName())

	yamlConfig := &YamlConfig{}
	found, err = plugin.PluginConfig.GetValue(yamlConfig)
	if !found || err != nil {
		return nil, found, err
	}

	plugin.Log.Debugf("sfc-config loaded: '%s'", yamlConfig)

	return yamlConfig, true, nil
}

// read external, hosts and chains, and render config via CNP and EE drivers
func (plugin *SfcControllerPluginHandler) copyYamlConfigToRamCache(yamlConfig *YamlConfig) error {

	for _, ee := range yamlConfig.EEs {
		plugin.ramConfigCache.EEs[ee.Name] = ee
		plugin.Log.Debugf("copyYamlConfigToRamCache: ee: ", ee)
	}
	for _, he := range yamlConfig.HEs {
		plugin.ramConfigCache.HEs[he.Name] = he
		plugin.Log.Debugf("copyYamlConfigToRamCache: he: ", he)
	}
	for _, sfc := range yamlConfig.SFCs {
		plugin.ramConfigCache.SFCs[sfc.Name] = sfc
		plugin.Log.Debugf("copyYamlConfigToRamCache: sfc: ", sfc)
		plugin.Log.Debugf("copyYamlConfigToRamCache: num_chain_elements=%d", len(sfc.GetElements()))
		for i, sfcChainElement := range sfc.GetElements() {
			plugin.Log.Debugf("copyYamlConfigToRamCache: sfc_chain_element[%d]=", i, sfcChainElement)
		}
	}

	return nil
}
