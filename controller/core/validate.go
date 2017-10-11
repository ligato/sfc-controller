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

func (plugin *SfcControllerPluginHandler) validateRamCache() error {

	for _, ee := range plugin.ramConfigCache.EEs {
		if err := plugin.ValidateExternalEntity(&ee); err != nil {
			return err
		}
	}
	for _, he := range plugin.ramConfigCache.HEs {
		if err := plugin.ValidateHostEntity(&he); err != nil {
			return err
		}
	}
	for _, sfc := range plugin.ramConfigCache.SFCs {
		if err := plugin.ValidateSFCEntity(&sfc); err != nil {
			return err
		}
	}

	return nil
}

// ValidateExternalEntity validates the External Router, TODO: perform better/complete validation
func (plugin *SfcControllerPluginHandler) ValidateExternalEntity(ee *controller.ExternalEntity) error {

	if ee.Name == "" {
		err := fmt.Errorf("Missing entity name")
		return err
	} else if ee.MgmntIpAddress == "" { //|| !validIpAddress(ee.MgmntIpAddress) {
		err := fmt.Errorf("Invalid mgmt_ip_address: '%s'", ee.MgmntIpAddress)
		return err
	}

	return nil
}

// ValidateHostEntity validates the Host Entity, TODO: perform better/complete validation
func (plugin *SfcControllerPluginHandler) ValidateHostEntity(he *controller.HostEntity) error {

	if he.Name == "" {
		err := fmt.Errorf("Missing entity name")
		return err
	}

	if he.Vni == 0 { // if not supplied, generate one
		plugin.seq.VLanID++ //TODO it would be nice to check if we do not have already this one (next phase)
		he.Vni = plugin.seq.VLanID
	}

	if he.LoopbackMacAddr == "" { // if not supplied, generate one
		plugin.seq.MacInstanceID++ //TODO it would be nice to check if we do not have already this one (next phase)
		he.LoopbackMacAddr = formatMacAddress(plugin.seq.MacInstanceID)
	}

	return nil
}

// ValidateSFCEntity validates the SFC, TODO: perform better/complete validation
func (plugin *SfcControllerPluginHandler) ValidateSFCEntity(sfc *controller.SfcEntity) error {

	if sfc.Name == "" {
		err := fmt.Errorf("Missing entity name")
		return err
	}
	numSfcElements := len(sfc.GetElements())
	if numSfcElements <= 0 {
		return nil
	}
	plugin.Log.Debugf("ValidateSFCEntity: sfc=", sfc)

	//for i, sfcChainElement := range sfc.GetElements() {
	//	plugin.Log.Debugf("ValidateSFCEntity: sfc_chain_element[%d]=", i, sfcChainElement)
	//	if sfcChainElement.Type == controller.SfcElementType_EXTERNAL_ENTITY {
	//		if i > 0 && i < numSfcElements-1 {
	//			err := fmt.Errorf("External entity cannot be inside the chain: i='%d', sfcElement:'%s'",
	//				i, sfcChainElement.Container)
	//			return err
	//		}
	//		if _, exists := plugin.ramConfigCache.EEs[sfcChainElement.Container]; !exists {
	//			err := fmt.Errorf("External entity in chain does not exist, chain: i='%d', sfcElement:'%s'",
	//				i, sfcChainElement.Container)
	//			return err
	//		}
	//	}
	//}

	// the sfc controller can be responsible for managing the mac and ip addresses if not provided t
	// TODO: figure out how ipam, and macam should be done, for now just start at 1 and go up :-(
	if sfc.SfcIpv4Prefix == "" {
		plugin.seq.IPInstanceID++ //TODO it would be nice to check if we do not have already this one (next phase)
		sfc.SfcIpv4Prefix = formatIpv4Address(plugin.seq.IPInstanceID)
	}

	if sfc.SfcIpv4PrefixLen == 0 {
		sfc.SfcIpv4PrefixLen = 24 //TODO it would be nice to check if we do not have already this one (next phase)
	}

	for _, sfcEl := range sfc.Elements {
		if sfcEl.MacAddr == "" {
			plugin.seq.MacInstanceID++ //TODO it would be nice to check if we do not have already this one (next phase)
			sfcEl.MacAddr = formatMacAddress(plugin.seq.MacInstanceID)
		}

		if sfcEl.PortId == 0 { // if not supplied, generate one
			plugin.seq.PortID++ //TODO it would be nice to check if we do not have already this one (next phase)
			sfcEl.PortId = plugin.seq.PortID
		}
	}

	return nil
}

func formatMacAddress(macInstanceId uint32) string {
	return "02:00:00:00:00:" + fmt.Sprintf("%02X", macInstanceId)
}

func formatIpv4Address(ipInstanceId uint32) string {
	return "10.0.0." + fmt.Sprintf("%d", ipInstanceId)
}
