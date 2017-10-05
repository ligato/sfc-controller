package vpptestdata

import (
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
)

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
	},}
