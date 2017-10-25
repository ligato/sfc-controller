package sfctestdata

import (
	sfccore "github.com/ligato/sfc-controller/controller/core"
	"github.com/ligato/sfc-controller/controller/model/controller"
)

// VPP1MEMIF2LoopbackVETH are test data with 2 memif poiting to loopback VETH
var VPP1MEMIF2LoopbackVETH = sfccore.YamlConfig{
	HEs: []controller.HostEntity{{
		Name: "HOST-1",
		//mgmnt_ip_address: "192.168.0.1",
		EthIfName:       "GigabitEthernet13/0/0",
		EthIpv4:         "8.42.0.2",
		LoopbackMacAddr: "02:00:00:AA:BB:00",
		LoopbackIpv4:    "6.0.0.100"},
	},
	SFCs: []controller.SfcEntity{{
		Name:        "sfc1",
		Description: "Wire 2 VNF containers to the vpp switch",
		Type:        controller.SfcType_SFC_EW_BD,
		Elements: []*controller.SfcEntity_SfcElement{{
			PortLabel:        "vpp1_memif1",
			MacAddr:          "02:02:02:02:02:02",
			EtcdVppSwitchKey: "HOST-1",
			Type:             controller.SfcElementType_CONTAINER_AGENT_VPP_MEMIF,
		}, {
			PortLabel:        "vpp1_memif2",
			Ipv4Addr:         "10:0:0:10",
			EtcdVppSwitchKey: "HOST-1",
			Type:             controller.SfcElementType_CONTAINER_AGENT_VPP_MEMIF,
		}, {
			PortLabel:        "agent1_afpacket1",
			Ipv4Addr:         "10:0:0:11",
			EtcdVppSwitchKey: "HOST-1",
			Type:             controller.SfcElementType_CONTAINER_AGENT_VPP_AFP,
		}},
	}},
	EEs: []controller.ExternalEntity{{
		Name: "VRouter-1",
		MgmntIpAddress:"192.168.42.1",
		BasicAuthUser:"cisco",
		BasicAuthPasswd:"cisco",
	},{
		Name: "RAS-1",
		MgmntIpAddress:"192.168.42.2",
		BasicAuthUser:"cisco",
		BasicAuthPasswd:"cisco",
	}},
}
/*
	external_entities:
	- name: VRouter-1
	mgmnt_ip_address: 192.168.42.1
	basic_auth_user_name: cisco
	basic_auth_passwd: cisco
	eth_ipv4: 8.42.0.1
	eth_ipv4_mask: 255.255.255.0
	loopback_ipv4: 112.1.1.3
	loopback_ipv4_mask: 255.255.255.0

	- name: RAS-1
	basic_auth_user_name: cisco
	basic_auth_passwd: cisco
	mgmnt_ip_address: 192.168.42.2
	eth_ipv4: 8.42.0.1
	eth_ipv4_mask: 255.255.255.0
	loopback_ipv4: 112.1.1.3
	loopback_ipv4_mask: 255.255.255.0
*/
