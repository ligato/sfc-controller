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

package l2

import (
	"github.com/ligato/sfc-controller/controller/model/controller"
)

func sfcControllerIDsKeyPrefix() string {
	return controller.SfcControllerPrefix() + "id/"
}

// HEIDsKeyPrefix returns the ETCD prefix
func HEIDsKeyPrefix() string {
	return sfcControllerIDsKeyPrefix() + "HE/"
}

// HE2EEIDsKeyPrefix returns the ETCD prefix
func HE2EEIDsKeyPrefix() string {
	return sfcControllerIDsKeyPrefix() + "H2E/"
}

// HE2HEIDsKeyPrefix returns the ETCD prefix
func HE2HEIDsKeyPrefix() string {
	return sfcControllerIDsKeyPrefix() + "H2H/"
}

// SFCIDsKeyPrefix returns the ETCD prefix
func SFCIDsKeyPrefix() string {
	return sfcControllerIDsKeyPrefix() + "SFC/"
}

// HEIDsNameKey returns the ETCD key
func HEIDsNameKey(name string) string {
	return HEIDsKeyPrefix() + name
}

// HE2EEIDsNameKey returns the ETCD key
func HE2EEIDsNameKey(heName string, eeName string) string {
	return HE2EEIDsKeyPrefix() + heName + "_" + eeName
}

// HE2HEIDsNameKey returns the ETCD key
func HE2HEIDsNameKey(shName string, dhName string) string {
	return HE2HEIDsKeyPrefix() + shName + "_" + dhName
}

// SFCIDsNameKey returns the ETCD key
func SFCIDsNameKey(name string) string {
	return SFCIDsKeyPrefix() + name
}

// SFCContainerPortIDsNameKey returns the ETCD key
func SFCContainerPortIDsNameKey(sfcName string, container string, port string) string {
	return SFCIDsNameKey(sfcName) + "/" + container + "_" + port
}
