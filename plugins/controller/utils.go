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
// "fmt"
// "github.com/ligato/sfc-controller/plugins/controller/model"
// "github.com/ligato/sfc-controller/plugins/controller/vppagent"
	"strings"
	"fmt"
)

// ConnPodName extracts the pod name from the pod/interface string
func ConnPodName(connPodInterfaceString string) string {
	n := strings.Index(connPodInterfaceString, "/")
	if n == -1 {
		return ""
	}
	return connPodInterfaceString[0:n]
}

// ConnInterfaceName extracts the interface from the pod/interface string
func ConnInterfaceName(connPodInterfaceString string) string {
	n := strings.Index(connPodInterfaceString, "/")
	if n == -1 {
		return ""
	}
	return connPodInterfaceString[n+1:]
}

// ConnPodInterfaceNames returns 2 strings (pod, interface)
func ConnPodInterfaceNames(connPodInterfaceString string) (string, string) {
	n := strings.Index(connPodInterfaceString, "/")
	if n == -1 {
		return "", ""
	}
	return connPodInterfaceString[0:n], connPodInterfaceString[n+1:]
}

// NodeInterfaceNames returns 2 strings (node, interface)
func NodeInterfaceNames(nodePodInterfaceString string) (string, string) {
	return ConnPodInterfaceNames(nodePodInterfaceString)
}

// ConnPodInterfaceSlashToUScore changes the / to a _
func ConnPodInterfaceSlashToUScore(connPodInterfaceString string) string {
	s0, s1 := ConnPodInterfaceNames(connPodInterfaceString)
	return fmt.Sprintf("%s_%s", s0, s1)
}

func ipAddressArraysEqual(a1 []string, a2 []string) bool {
	if len(a1) != len(a2) {
		return false
	}
	foundCount := 0
	for _, e1 := range a1 {
		for _, e2 := range a2 {
			if e1 == e2 {
				foundCount++
				break
			}
		}
	}
	if foundCount != len(a1) {
		return false
	}

	return true
}
