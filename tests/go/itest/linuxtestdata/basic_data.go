package linuxtestdata

import (
	"github.com/ligato/vpp-agent/plugins/linuxplugin/model/interfaces"
)

// Agent1Veth01 are test data for VETH interface for agent1 test data
var Agent1Veth01 = interfaces.LinuxInterfaces_Interface{
	Name:        "agent1_afpacket",
	Enabled:     true,
	//PhysAddress: "02:00:00:00:00:02",
	Mtu:         1500,
	Type:        interfaces.LinuxInterfaces_VETH,
	//IpAddresses: []string{"10.0.0.10/24"},
	Veth: &interfaces.LinuxInterfaces_Interface_Veth{
		PeerIfName: "_agent1_",
	},
}
