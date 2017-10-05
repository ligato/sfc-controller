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

// AddInterface adds a new interface into the router's configuration.
func (s *Session) AddInterface(iface *iosxe.Interface) error {
	logFields := log.WithFields(logging.Fields{
		"ifName": iface.Name,
		"if":     iface,
	})
	logFields.Info("Adding interface")

	cmds := []string{
		"interface " + iface.Name,
	}

	// description
	if iface.Decription != "" {
		cmds = append(cmds, fmt.Sprintf("description %s", iface.Decription))
	}

	// IP address
	if iface.IpAddress != "" {
		ip, netmask, err := ipv4CidrToIPMask(iface.IpAddress)
		if err != nil {
			return err
		}
		cmds = append(cmds, fmt.Sprintf("ip address %s %s", ip, netmask))
	}

	// IP redirects (on by default)
	if !iface.IpRedirects {
		cmds = append(cmds, "no ip redirects")
	}

	// service-instance
	if iface.ServiceInstance != nil {
		cmds = append(cmds,
			fmt.Sprintf("service instance %d ethernet", iface.ServiceInstance.Id),
			fmt.Sprintf("encapsulation %s", iface.ServiceInstance.Encapsulation),
			"exit")
	}

	// VXLANs
	for _, vxlan := range iface.Vxlan {
		cmds = append(cmds, fmt.Sprintf("member vni %d", vxlan.Vni))
		cmds = append(cmds, fmt.Sprintf("ingress-replication %s", vxlan.DstAddress))
		cmds = append(cmds, "exit")
		cmds = append(cmds, fmt.Sprintf("source-interface %s", vxlan.SrcInterfaceName))
	}

	cmds = append(cmds, "exit")

	// execute the commands on VTY
	s.enterConfigMode()
	resp, err := s.vty.ExecCMD(cmds...)

	if err != nil {
		logFields.Error("Error by configuring interface: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}
	return nil
}

// DeleteInterface deletes the interface from the router's configuration.
func (s *Session) DeleteInterface(iface *iosxe.Interface) error {
	logFields := log.WithFields(logging.Fields{
		"ifName": iface.Name,
		"if":     iface,
	})
	logFields.Info("Deleting interface")

	var resp string
	var err error

	// execute the commands on VTY
	s.enterConfigMode()
	if iface.Type == iosxe.InterfaceType_ETHERNET_CSMACD {
		resp, err = s.vty.ExecCMD(fmt.Sprintf("default interface %s", iface.Name))
	} else {
		resp, err = s.vty.ExecCMD(fmt.Sprintf("no interface %s", iface.Name))
	}

	if err != nil {
		logFields.Error("Error by deleting interface: ", err)
		return err
	}
	if err = checkResponse(resp); err != nil {
		return err
	}
	return nil
}

// ModifyInterface modifies the existing interface config to a new version of it.
func (s *Session) ModifyInterface(oldIface, newIface *iosxe.Interface) error {

	// TODO: implement harmless modification (without deleting and impacting of existing config)

	err := s.DeleteInterface(oldIface)
	if err != nil {
		return err
	}

	err = s.AddInterface(newIface)
	if err != nil {
		return err
	}

	return nil
}

// DumpInterfaces dumps all existing interfaces from the router into a map indexed by interface names.
func (s *Session) DumpInterfaces() (map[string]*iosxe.Interface, error) {
	log.Debug("Dumping interfaces")

	ifs := make(map[string]*iosxe.Interface)

	// dump via VTY
	s.exitConfigMode()
	resp, err := s.vty.ExecCMD("show running-config | section interface")

	if err != nil {
		log.Error("Error by dumping interfaces: ", err)
		return ifs, err
	}
	if err = checkResponse(resp); err != nil {
		return ifs, err
	}

	var ifName, srcIfName string
	var vxlan *iosxe.Interface_Vxlan

	for _, line := range strings.Split(strings.TrimSuffix(resp, "\n"), "\n") {
		if strings.HasPrefix(line, "interface ") {
			// create interface
			ifName = strings.TrimPrefix(line, "interface ")
			ifs[ifName] = &iosxe.Interface{
				Name:        ifName,
				Type:        getInterfaceType(ifName),
				IpRedirects: true, /* default */
			}
			// zero if-local variables
			srcIfName = ""
			vxlan = nil
		} else {
			// fill in interface details
			if _, ok := ifs[ifName]; !ok {
				log.Warn("non-existing interface %s", ifName)
				continue
			}

			// description
			if strings.HasPrefix(line, "description ") {
				description := strings.TrimPrefix(line, "description ")
				ifs[ifName].Decription = strings.Trim(description, "\"")
			}

			// ip address
			var ip, netmask string
			if _, err := fmt.Sscanf(line, "ip address %s %s", &ip, &netmask); err == nil {
				prefixSize, _ := net.IPMask(net.ParseIP(netmask).To4()).Size()
				ifs[ifName].IpAddress = ip + "/" + strconv.Itoa(prefixSize)
			}

			// ip redirects
			if strings.HasPrefix(line, "no ip redirects") {
				ifs[ifName].IpRedirects = false
			}

			// service instance
			var serviceInstance uint32
			if _, err := fmt.Sscanf(line, "service instance %d ethernet", &serviceInstance); err == nil {
				ifs[ifName].ServiceInstance = &iosxe.Interface_ServiceInstance{Id: serviceInstance}
			}
			if strings.HasPrefix(line, "encapsulation ") && ifs[ifName].ServiceInstance != nil {
				ifs[ifName].ServiceInstance.Encapsulation = strings.TrimPrefix(line, "encapsulation ")
			}

			// VXLAN
			var vni uint32
			if _, err := fmt.Sscanf(line, "member vni %d", &vni); err == nil {
				vxlan = &iosxe.Interface_Vxlan{
					Vni:              vni,
					SrcInterfaceName: srcIfName,
				}
				ifs[ifName].Vxlan = append(ifs[ifName].Vxlan, vxlan)
			}
			if strings.HasPrefix(line, "ingress-replication ") && vxlan != nil {
				vxlan.DstAddress = strings.TrimPrefix(line, "ingress-replication ")
			}
			if strings.HasPrefix(line, "source-interface ") {
				srcIfName = strings.TrimPrefix(line, "source-interface ")
			}
		}
	}

	return ifs, nil
}

// getInterfaceType returns interface type derived from interface name.
func getInterfaceType(ifName string) iosxe.InterfaceType {
	switch {
	case strings.HasPrefix(ifName, "Loopback"):
		return iosxe.InterfaceType_SOFTWARE_LOOPBACK
	case strings.HasPrefix(ifName, "BDI"):
		return iosxe.InterfaceType_BDI_INTERFACE
	case strings.HasPrefix(ifName, "nve"):
		return iosxe.InterfaceType_NVE_INTERFACE
	default:
		return iosxe.InterfaceType_ETHERNET_CSMACD
	}
}
