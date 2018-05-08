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

// The core plugin which drives the SFC Controller.  The core initializes the
// CNP dirver plugin based on command line args.  The database is initialized,
// and a resync is preformed based on what was already in the database.

package controller

// SfcControllerPrefix is the base for all entries
func SfcControllerPrefix() string {
	return "/sfc-controller"
}

// SfcControllerConfigPrefix provides sfc controller prefix
func SfcControllerConfigPrefix() string {
	return SfcControllerPrefix() + "/v2/config/"
}

// SfcControllerStatusPrefix provides sfc controller prefix
func SfcControllerStatusPrefix() string {
	return SfcControllerPrefix() + "/v2/status/"
}

// SfcControllerContivKSRPrefix is the base for all contiv ksr entries
func SfcControllerContivKSRPrefix() string {
	return "/vnf-agent/contiv-ksr"
}
