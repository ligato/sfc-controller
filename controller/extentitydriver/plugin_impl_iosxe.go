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

package extentitydriver

import (
	"github.com/ligato/cn-infra/utils/safeclose"
)

// Plugin initializes
type Plugin struct {
}

// Init the controller, read the db, reconcile/resync, render config to etcd
func (plugin *Plugin) Init() error {
	go processEEOperationChannel()
	return nil
}

// perform close down procedures
func (plugin *Plugin) Close() error {
	return safeclose.Close(EEOperationChannel)
}
