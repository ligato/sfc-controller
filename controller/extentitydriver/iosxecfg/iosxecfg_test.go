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
	"testing"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/sfc-controller/controller/extentitydriver/iosxecfg/model/iosxe"
	"github.com/ligato/cn-infra/logging/logrus"
)

func TestInterfaces(t *testing.T) {
	var log = logrus.DefaultLogger()
	log.SetLevel(logging.DebugLevel)

	s, err := NewSSHSession("10.195.94.48", 22, "cisco", "cisco")
	if err != nil {
		t.Error(err)
		return
	}
	defer s.Close()

	// add
	err = s.AddInterface(&iosxe.Interface{
		Name:        "loopback53",
		Type:        iosxe.InterfaceType_SOFTWARE_LOOPBACK,
		Decription:  "loopback 53",
		IpAddress:   "1.2.3.53/24",
		IpRedirects: false,

		//ServiceInstance: &iosxenb.Interface_ServiceInstance{
		//	Id:            53,
		//	Encapsulation: "untagged",
		//},

		//Vxlan: []*iosxenb.Interface_Vxlan{
		//	{
		//		Vni:              5053,
		//		DstAddress:       "1.2.3.4",
		//		SrcInterfaceName: "loopback0",
		//	},
		//},
	})
	if err != nil {
		t.Error(err)
		return
	}

	// dump
	bds, err := s.DumpInterfaces()
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%v\n", bds)

	// delete
	err = s.DeleteInterface(&iosxe.Interface{
		Name: "loopback53",
		Type: iosxe.InterfaceType_SOFTWARE_LOOPBACK,
	})
	if err != nil {
		t.Error(err)
		return
	}

	err = s.CopyRunningToStartup()
	if err != nil {
		t.Error(err)
		return
	}
}

func TestBridgeDomains(t *testing.T) {
	log.SetLevel(logging.DebugLevel)

	s, err := NewSSHSession("10.195.94.48", 22, "cisco", "cisco")
	if err != nil {
		t.Error(err)
		return
	}
	defer s.Close()

	// add
	err = s.AddBridgeDomain(&iosxe.BridgeDomain{
		Id:  456,
		Vni: []uint32{5456},

		Interfaces: []*iosxe.BridgeDomain_Interface{
			{
				Name:            "GigabitEthernet3",
				ServiceInstance: 456,
			},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	// dump
	bds, err := s.DumpBridgeDomains()
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%v\n", bds)

	// delete
	err = s.DeleteBridgeDomain(&iosxe.BridgeDomain{Id: 456})
	if err != nil {
		t.Error(err)
		return
	}
}

func TestStaticRoutes(t *testing.T) {
	log.SetLevel(logging.DebugLevel)

	s, err := NewSSHSession("10.195.94.48", 22, "cisco", "cisco")
	if err != nil {
		t.Error(err)
		return
	}
	defer s.Close()

	// add
	err = s.AddStaticRoute(&iosxe.StaticRoute{
		DstAddress:     "1.2.3.4/32",
		NextHopAddress: "8.8.8.8",
	})
	if err != nil {
		t.Error(err)
		return
	}

	// dump
	bds, err := s.DumpStaticRoutes()
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%v\n", bds)

	// delete
	err = s.DeleteStaticRoute(&iosxe.StaticRoute{
		DstAddress:     "1.2.3.4/32",
		NextHopAddress: "8.8.8.8",
	})
	if err != nil {
		t.Error(err)
		return
	}
}
