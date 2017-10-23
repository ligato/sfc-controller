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

// These are validation routines intended to verify correctness of the config
// individual fields.

package core

import (
	"fmt"
	"github.com/ligato/sfc-controller/controller/model/controller"
)

func (sfcCtrlPlugin *SfcControllerPluginHandler) validateRamCache() error {

	if err := sfcCtrlPlugin.validateSystemParameters(&sfcCtrlPlugin.ramConfigCache.SysParms); err != nil {
		return err
	}

	for _, ee := range sfcCtrlPlugin.ramConfigCache.EEs {
		if err := sfcCtrlPlugin.validateEE(&ee); err != nil {
			return err
		}
	}
	for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
		if err := sfcCtrlPlugin.validateHE(&he); err != nil {
			return err
		}
	}
	for _, sfc := range sfcCtrlPlugin.ramConfigCache.SFCs {
		if err := sfcCtrlPlugin.validateSFC(&sfc); err != nil {
			return err
		}
	}

	return nil
}

// validate the system parameters
func (sfcCtrlPlugin *SfcControllerPluginHandler) validateSystemParameters(sp *controller.SystemParameters) error {

	if sp.Mtu == 0 {
		log.Info("validateSystemParameters: sys mtu = 0, defaulting ot 1500")
		sp.Mtu = 1500 // if not provided, default it to 1500
		return nil
	}

	return nil
}

// validate the External Router, TODO: perform better/complete validation
func (sfcCtrlPlugin *SfcControllerPluginHandler) validateEE(ee *controller.ExternalEntity) error {

	if ee.Name == "" {
		err := fmt.Errorf("Missing entity name")
		return err
	} else if ee.MgmntIpAddress == "" { //|| !validIpAddress(ee.MgmntIpAddress) {
		err := fmt.Errorf("Invalid mgmt_ip_address: '%s'", ee.MgmntIpAddress)
		return err
	}

	return nil
}

// validate the Host Entity, TODO: perform better/complete validation
func (sfcCtrlPlugin *SfcControllerPluginHandler) validateHE(he *controller.HostEntity) error {

	if he.Name == "" {
		err := fmt.Errorf("Missing entity name")
		return err
	}

	return nil
}

// validate the SFC, TODO: perform better/complete validation
func (sfcCtrlPlugin *SfcControllerPluginHandler) validateSFC(sfc *controller.SfcEntity) error {

	if sfc.Name == "" {
		err := fmt.Errorf("Missing entity name")
		return err
	}
	numSfcElements := len(sfc.GetElements())
	if numSfcElements <= 0 {
		return nil
	}
	log.Debugf("validateSFC: sfc=", sfc)

	//for i, sfcChainElement := range sfc.GetElements() {
	//	log.Debugf("validateSFC: sfc_chain_element[%d]=", i, sfcChainElement)
	//	if sfcChainElement.Type == controller.SfcElementType_EXTERNAL_ENTITY {
	//		if i > 0 && i < numSfcElements-1 {
	//			err := fmt.Errorf("External entity cannot be inside the chain: i='%d', sfcElement:'%s'",
	//				i, sfcChainElement.Container)
	//			return err
	//		}
	//		if _, exists := sfcCtrlPlugin.ramConfigCache.EEs[sfcChainElement.Container]; !exists {
	//			err := fmt.Errorf("External entity in chain does not exist, chain: i='%d', sfcElement:'%s'",
	//				i, sfcChainElement.Container)
	//			return err
	//		}
	//	}
	//}

	return nil
}
