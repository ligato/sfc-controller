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

// package agentctl contains the agentctl tool for monitoring and configuring
// of VPP Agents. The tool connects to an Etcd instance, it discovers VPP
// Agents connected to the instance, and monitors their status. The tool can
// also write VPP Agent configuration into etcd (note that the VPP Agent does
// not have to be coneected to Etcd for that, it simply get the config when
// it connects).
package main

import (
	"fmt"
	"github.com/ligato/vpp-agent/cmd/agentctl/cmd"
	"os"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
