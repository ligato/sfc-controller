package vpptestdata

import (
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/l2"
)

// VPP1MEMIF1 are test data for vpp1's first memif interface
var VPP1MEMIF1 = interfaces.Interfaces_Interface{
	Name:    "IF_MEMIF_VSWITCH__vpp1_memif1",
	Enabled: true,
	//PhysAddress: "02:02:02:02:02:02",
	Type: interfaces.InterfaceType_MEMORY_INTERFACE,
	Mtu:  1500,
	//IpAddresses: []string{"10.0.0.1/24"},
	Memif: &interfaces.Interfaces_Interface_Memif{
		Id:             1,
		SocketFilename: "/tmp/memif.sock",
		Master:         true,
	},
}

// VPP1MEMIF2 are test data for vpp1's second memif interface
var VPP1MEMIF2 = interfaces.Interfaces_Interface{
	Name:    "IF_MEMIF_VSWITCH__vpp1_memif2",
	Enabled: true,
	//PhysAddress: "02:00:00:00:00:01",
	Mtu:  1500,
	Type: interfaces.InterfaceType_MEMORY_INTERFACE,
	//IpAddresses: []string{"10.0.0.10/24"},
	Memif: &interfaces.Interfaces_Interface_Memif{
		Id:             2,
		SocketFilename: "/tmp/memif.sock",
		Master:         true,
	}}

// BDINTERNALEWHOST1 are test data for bridge domain with internal EW host1
var BDINTERNALEWHOST1 = l2.BridgeDomains_BridgeDomain{
	Name:                "BD_INTERNAL_EW_HOST-1",
	Flood:               true,
	UnknownUnicastFlood: true,
	Forward:             true,
	Learn:               true,
	ArpTermination:      false,
	MacAge:              0,
	Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{Name: "IF_MEMIF_VSWITCH__vpp1_memif1"},
		{Name: "IF_MEMIF_VSWITCH__vpp1_memif2"},
		{Name: "IF_AFPIF_VSWITCH__agent1_afpacket1"}},
	ArpTerminationTable: nil,
}

//time="2017-10-06 11:35:03.62305" level=info msg="ReconcileEnd: add BD key to etcd:
// /vnf-agent/HOST-1/vpp/config/v1/bd/BD_INTERNAL_EW_HOST-1
//

//time= "2017-10-06 11:55:55.48623" level = info msg = "{[name:"agent1_afpacket" enabled:true phys_address:"02:00:00:00:00:02" mtu:1500 namespace:<type:MICROSERVICE_REF_NS > veth:<peer_if_name:"_agent1_" > ]}" loc = "cnpdriver/sfcctlr_l2_driver.go(1394)" logger = defaultLogger tag = 00000000
//TODO time = "2017-10-06 11:55:55.49223" level = info msg = "ReconcileEnd: add i/f key to etcd: /vnf-agent/HOST-1/vpp/config/v1/interface/GigabitEthernet13/0/0{GigabitEthernet13/0/0  ETHERNET_CSMACD true  1500 [8.42.0.2] <nil> <nil> <nil> <nil>}" loc = "cnpdriver/sfcctlr_l2_driver.go(235)" logger = defaultLogger tag = 00000000

// Agent1Afpacket01 are test data of Agent1's AF packet interface
var Agent1Afpacket01 = interfaces.Interfaces_Interface{
	Name:    "IF_AFPIF_VSWITCH__agent1_afpacket1",
	Enabled: true,
	//PhysAddress: "02:00:00:00:00:02",
	Mtu:  1500,
	Type: interfaces.InterfaceType_AF_PACKET_INTERFACE,
	//IpAddresses: []string{"10.0.0.10/24"},
	Afpacket: &interfaces.Interfaces_Interface_Afpacket{
		HostIfName: "_agent1_",
	},
	//namespace:<type:MICROSERVICE_REF_NS > veth:<peer_if_name:"_agent1_" >
}

// Agent1Loopback are test data of Agent1's software loopback interface
var Agent1Loopback = interfaces.Interfaces_Interface{
	Name:        "IF_LOOPBACK_H_HOST-1",
	Enabled:     true,
	PhysAddress: "02:00:00:AA:BB:00",
	Mtu:         1500,
	Type:        interfaces.InterfaceType_SOFTWARE_LOOPBACK,
	IpAddresses: []string{"6.0.0.100"},
}
