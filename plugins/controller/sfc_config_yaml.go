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

package controller

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/ligato/sfc-controller/plugins/controller/model"
	"io/ioutil"
	"time"
)

const sfcYamlVersion = 2

// SfcConfigYaml is the container struct for the YAML config file
type SfcConfigYaml struct {
	Version             int                               `json:"sfc_controller_config_version"`
	Description         string                            `json:"description"`
	SysParms            *controller.SystemParameters      `json:"system_parameters"`
	IPAMPools           []*controller.IPAMPool            `json:"ipam_pools"`
	NetworkPodToNodeMap []*controller.NetworkPodToNodeMap `json:"network_pod_to_node_map"`
	NetworkNodeOverlays []*controller.NetworkNodeOverlay  `json:"network_node_overlays"`
	NetworkNodes        []*controller.NetworkNode         `json:"network_nodes"`
	NetworkServices     []*controller.NetworkService      `json:"network_services"`
	RAMCache            *CacheType                        `json:"ram_cache"`
}

// SfcConfigYamlReadFromFile parses the yaml into YamlConfig
func (s *Plugin) SfcConfigYamlReadFromFile(fpath string) (*SfcConfigYaml, error) {

	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	yamlConfig := &SfcConfigYaml{}

	log.Infof("SfcConfigYamlReadFromFile: yaml.YAMLToJSON ...")
	jb, err := yaml.YAMLToJSON(b)
	if err != nil {
		log.Infof("SfcConfigYamlReadFromFile: yaml=%s ...", string(b))
		return nil, err
	}
	log.Infof("SfcConfigYamlReadFromFile: json.Unmarshal ...")
	if err = json.Unmarshal(jb, yamlConfig); err != nil {
		log.Infof("SfcConfigYamlReadFromFile: json=%s ...", string(jb))
		return nil, err
	}

	log.Infof("SfcConfigYamlReadFromFile: ok, gostruct=%v ...", yamlConfig)

	//if err := yaml.Unmarshal(b, yamlConfig); err != nil {
	//	return nil, err
	//}

	return yamlConfig, nil
}

// SfcSystemCacheToYaml is used by http to return the whole system config in YAML format
func (s *Plugin) SfcSystemCacheToYaml() ([]byte, error) {
	yamlConfig := &SfcConfigYaml{}

	yamlConfig.Version = sfcYamlVersion
	yamlConfig.Description = fmt.Sprintf("Config: %s", time.Now())
	yamlConfig.NetworkNodes = ctlrPlugin.NetworkNodeMgr.ToArray()
	yamlConfig.NetworkPodToNodeMap = ctlrPlugin.NetworkPodNodeMapMgr.ToArray()
	yamlConfig.NetworkServices = ctlrPlugin.NetworkServiceMgr.ToArray()
	yamlConfig.NetworkNodeOverlays = ctlrPlugin.NetworkNodeOverlayMgr.ToArray()
	yamlConfig.IPAMPools = ctlrPlugin.IpamPoolMgr.ToArray()
	yamlConfig.SysParms = ctlrPlugin.SysParametersMgr.sysParmCache
	yamlConfig.RAMCache = &ctlrPlugin.ramCache

	yamlBytes, err := yaml.Marshal(yamlConfig)
	if err != nil {
		return nil, err
	}
	return yamlBytes, nil
}

// SfcConfigYamlProcessConfig processes each object and adds it to the system
func (s *Plugin) SfcConfigYamlProcessConfig(y *SfcConfigYaml) error {

	if y.Version != sfcYamlVersion {
		return fmt.Errorf("SfcConfigYamlProcessConfig: incorrect yaml version, expecting %d, got: %d",
			sfcYamlVersion, y.Version)
	}

	if y.SysParms != nil {
		log.Debugf("SfcConfigYamlProcessConfig: system parameters: %v", y.SysParms)
		if err := ctlrPlugin.SysParametersMgr.HandleCRUDOperationCU(y.SysParms); err != nil {
			return err
		}
	}

	log.Debugf("SfcConfigYamlProcessConfig: ipam pools: %v", y.IPAMPools)
	for _, ipamPool := range y.IPAMPools {
		if err := ctlrPlugin.IpamPoolMgr.HandleCRUDOperationCU(ipamPool); err != nil {
			return err
		}
	}

	for _, nn := range y.NetworkNodes {
		log.Debugf("SfcConfigYamlProcessConfig: network node: %v", nn)
		if err := ctlrPlugin.NetworkNodeMgr.HandleCRUDOperationCU(nn); err != nil {
			return err
		}
	}

	for _, ns := range y.NetworkServices {
		log.Debugf("SfcConfigYamlProcessConfig: network-service: %v", ns)
		if err := ctlrPlugin.NetworkServiceMgr.HandleCRUDOperationCU(ns); err != nil {
			return err
		}
	}

	for _, p2n := range y.NetworkPodToNodeMap {
		log.Debugf("SfcConfigYamlProcessConfig: network-pod-to-node-map: %v", p2n)
		if err := ctlrPlugin.NetworkPodNodeMapMgr.HandleCRUDOperationCU(p2n); err != nil {
			return err
		}
	}

	log.Debugf("SfcConfigYamlProcessConfig: network-node-overlays: %v", y.NetworkNodeOverlays)
	for _, nno := range y.NetworkNodeOverlays {
		if err := ctlrPlugin.NetworkNodeOverlayMgr.HandleCRUDOperationCU(nno); err != nil {
			return err
		}
	}

	return nil
}
