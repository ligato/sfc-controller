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

package iosxecfg

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/sfc-controller/controller/extentitydriver/iosxecfg/model/iosxe"
)

// AddStaticRoute adds a new static route into the router's configuration.
func (s *Session) AddStaticRoute(route *iosxe.StaticRoute) error {

	ip, netmask, err := ipv4CidrToIPMask(route.DstAddress)
	if err != nil {
		return err
	}
	return s.addDelIPRoute(ip, netmask, route.NextHopAddress, true)
}

// DeleteStaticRoute deletes the static route from the router's configuration.
func (s *Session) DeleteStaticRoute(route *iosxe.StaticRoute) error {

	ip, netmask, err := ipv4CidrToIPMask(route.DstAddress)
	if err != nil {
		return err
	}
	return s.addDelIPRoute(ip, netmask, route.NextHopAddress, false)
}

// ModifyStaticRoute modifies the existing static route config to a new version of it.
func (s *Session) ModifyStaticRoute(oldRoute, newRoute *iosxe.StaticRoute) error {
	err := s.DeleteStaticRoute(oldRoute)
	if err != nil {
		return err
	}

	err = s.AddStaticRoute(newRoute)
	if err != nil {
		return err
	}

	return nil
}

// DumpStaticRoutes dumps all existing static routes from the router into a map indexed by destination prefix.
func (s *Session) DumpStaticRoutes() (map[string]*iosxe.StaticRoute, error) {
	log.Debug("Dumping static routes")

	routes := make(map[string]*iosxe.StaticRoute)

	// dump via VTY
	s.exitConfigMode()
	resp, err := s.vty.ExecCMD("show running-config | section ip route")

	if err != nil {
		log.Error("Error by dumping static routes: ", err)
		return routes, err
	}
	if err = checkResponse(resp); err != nil {
		return routes, err
	}

	for _, line := range strings.Split(strings.TrimSuffix(resp, "\n"), "\n") {

		var ip, netmask, nhIP string
		if _, err := fmt.Sscanf(line, "ip route %s %s %s", &ip, &netmask, &nhIP); err == nil {
			// create the route
			prefixSize, _ := net.IPMask(net.ParseIP(netmask).To4()).Size()
			prefix := ip + "/" + strconv.Itoa(prefixSize)

			routes[prefix] = &iosxe.StaticRoute{
				DstAddress:     prefix,
				NextHopAddress: nhIP,
			}
		}
	}

	return routes, nil
}

// addDelIPRoute adds or deletes static route configuration on the router.
func (s *Session) addDelIPRoute(dstPrefix, dstMask, nextHopIP string, isAdd bool) error {
	logFields := log.WithFields(logging.Fields{
		"dstPrefix": dstPrefix,
		"dstMask":   dstMask,
		"nextHopIP": nextHopIP,
	})
	if isAdd {
		logFields.Info("Adding a new static route")
	} else {
		logFields.Info("Deleting static route")
	}

	mainCmd := ""
	if isAdd {
		mainCmd = fmt.Sprintf("ip route %s %s %s", dstPrefix, dstMask, nextHopIP)
	} else {
		mainCmd = fmt.Sprintf("no ip route %s %s %s", dstPrefix, dstMask, nextHopIP)
	}

	// execute the commands on VTY
	s.enterConfigMode()
	resp, err := s.vty.ExecCMD(mainCmd)

	if err != nil {
		log.Error("Error by configuring route: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}

	return nil
}
