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

// Package core : The API module exports methods to enable other plugin to access
// info stored in ram caches.
package core

import (
)

func (sfcCtrlPlugin *SfcControllerPluginHandler) GetSfcInterfaceIPAndMac(container string, port string) (string, string, error) {
	return sfcCtrlPlugin.cnpDriverPlugin.GetSfcInterfaceIPAndMac(container, port)
}