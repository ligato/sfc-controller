# SFC Controller VxLAN Hub (External Router) and Spoke (Hosts) L2MP example

Please note that the content of this page is currently **WORK IN PROGRESS**.

This example describes a 3 host scenario with 3 VNF's.  There is also an
external router.  A VNF L2 P2MP service is described which bridges L2
traffic between the ports on the VNFs. The external router is used as
a hub for vxlan tunnels to the hosts.


## VNF's 2 on Vswitch 3 and the other on Vswitch 2

![VNF HubandSpoke](vxlan_2.png "VNF HubandSpoke")

1. Start up the controller with a yaml file

```
sfc-controller -sfc-config=toplogies/vxlanhubandspoke/l2pp/vxlanl2pp.yaml -clean
```

The yaml file looks like:
```
sfc_controller_config_version: 2
description: 3 node with 1 nic port, host-host vxlan brdge, vnf on each node, hub and spoke via external router

system_parameters:
  ipam_pools:
    - name: vxlan_loopback_pool
      network: 111.111.111.0/24
      
vnf_to_node_map:
  - vnf: vnf1
    node: vswitch3
  - vnf: vnf2
    node: vswitch2
  - vnf: vnf3
    node: vswitch3

nodes:
  - name: vswitch1
    node_type: host
    interfaces:
      - name: gigethernet13/0/1
        if_type: ethernet
        ip_addresses:
          - "10.100.1.1/24"
        custom_labels:
          - vxlan
  - name: vswitch2
    node_type: host
    interfaces:
      - name: gigethernet13/0/2
        if_type: ethernet
        ip_addresses:
          - "10.100.2.2/24"
        custom_labels:
          - vxlan 

  - name: vswitch3
    node_type: host
    interfaces:
      - name: gigethernet13/0/3
        if_type: ethernet
        ip_addresses:
          - "10.100.3.3/24"
        custom_labels:
          - vxlan         

  - name: router-a
    node_type: external
    interfaces:
      - name: gigethernet13/0/a
        if_type: ethernet
        ip_addresses:
          - "10.100.4.4/24"
        custom_labels:
          - vxlan

vnf_services:
  - name: service1
    vnfs:
      - name: vnf1
        vnf_type: vppcontainer
        interfaces:
          - name: port1
            if_type: memif
      - name: vnf2
        node: vswitch2
        vnf_type: vppcontainer
        interfaces:
          - name: port1
            if_type: memif
      - name: vnf3
        node: vswitch3
        vnf_type: vppcontainer
        interfaces:
          - name: port1
            if_type: memif
    connections:
      - conn_type: l2mp
        vnf_service_mesh: router-a-hub-and-spoke
        interfaces:
          - vnf: vnf1
            interface: port1
          - vnf: vnf2
            interface: port1
          - vnf: vnf3
            interface: port1

vnf_service_meshes:
  - name: router-a-hub-and-spoke
    service_mesh_type: hub_and_spoke
    connection_type: vxlan
    vxlan_hub_and_spoke_parms:
      hub_node_name: router-a
      vni: 6000
      loopback_ipam_pool_name: vxlan_loopback_pool
      outgoing_interface_label: vxlan


```

The vpp agent keys, and values are as follows::

```
[
  {
    "VppKey": "/vnf-agent/vswitch2/vpp/config/v1/interface/IF_MEMIF_VSWITCH_vnf2_port1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_MEMIF_VSWITCH_vnf2_port1",
      "type": 2,
      "enabled": true,
      "mtu": 1500,
      "memif": {
        "master": true,
        "id": 2,
        "socket_filename": "/tmp/memif_vswitch2.sock"
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch2/vpp/config/v1/bd/L2BD_service1_CONN_1",
    "VppEntryType": "l2bd",
    "L2BD": {
      "name": "L2BD_service1_CONN_1",
      "flood": true,
      "unknown_unicast_flood": true,
      "forward": true,
      "learn": true,
      "interfaces": [
        {
          "name": "IF_MEMIF_VSWITCH_vnf2_port1"
        },
        {
          "name": "IF_VXLAN_FROM_SPOKE_vswitch2_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000"
        }
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch2/vpp/config/v1/interface/gigethernet13/0/2",
    "VppEntryType": "interface",
    "IFace": {
      "name": "gigethernet13/0/2",
      "type": 1,
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "10.100.2.2/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/interface/IF_MEMIF_VSWITCH_vnf1_port1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_MEMIF_VSWITCH_vnf1_port1",
      "type": 2,
      "enabled": true,
      "mtu": 1500,
      "memif": {
        "master": true,
        "id": 1,
        "socket_filename": "/tmp/memif_vswitch3.sock"
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vnf3/vpp/config/v1/interface/port1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "port1",
      "type": 2,
      "enabled": true,
      "mtu": 1500,
      "memif": {
        "id": 3,
        "socket_filename": "/tmp/memif_vswitch3.sock"
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vnf1/vpp/config/v1/interface/port1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "port1",
      "type": 2,
      "enabled": true,
      "mtu": 1500,
      "memif": {
        "id": 1,
        "socket_filename": "/tmp/memif_vswitch3.sock"
      }
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/interface/gigethernet13/0/a",
    "VppEntryType": "interface",
    "IFace": {
      "name": "gigethernet13/0/a",
      "type": 1,
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "10.100.4.4/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/interface/IF_VXLAN_LOOPBACK_router-a",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_LOOPBACK_router-a",
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "111.111.111.1/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/bd/L2BD_service1_CONN_1",
    "VppEntryType": "l2bd",
    "L2BD": {
      "name": "L2BD_service1_CONN_1",
      "flood": true,
      "unknown_unicast_flood": true,
      "forward": true,
      "learn": true,
      "interfaces": [
        {
          "name": "IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch2_VSRVC_service1_CONN_0_VNI_6000"
        },
        {
          "name": "IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch3_VSRVC_service1_CONN_0_VNI_6000"
        }
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/bd/L2BD_service1_CONN_1",
    "VppEntryType": "l2bd",
    "L2BD": {
      "name": "L2BD_service1_CONN_1",
      "flood": true,
      "unknown_unicast_flood": true,
      "forward": true,
      "learn": true,
      "interfaces": [
        {
          "name": "IF_MEMIF_VSWITCH_vnf1_port1"
        },
        {
          "name": "IF_MEMIF_VSWITCH_vnf3_port1"
        },
        {
          "name": "IF_VXLAN_FROM_SPOKE_vswitch3_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000"
        }
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/interface/gigethernet13/0/3",
    "VppEntryType": "interface",
    "IFace": {
      "name": "gigethernet13/0/3",
      "type": 1,
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "10.100.3.3/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/interface/IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch2_VSRVC_service1_CONN_0_VNI_6000",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch2_VSRVC_service1_CONN_0_VNI_6000",
      "type": 5,
      "enabled": true,
      "vxlan": {
        "src_address": "111.111.111.1",
        "dst_address": "111.111.111.3",
        "vni": 6000
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/interface/IF_MEMIF_VSWITCH_vnf3_port1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_MEMIF_VSWITCH_vnf3_port1",
      "type": 2,
      "enabled": true,
      "mtu": 1500,
      "memif": {
        "master": true,
        "id": 3,
        "socket_filename": "/tmp/memif_vswitch3.sock"
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/interface/IF_VXLAN_LOOPBACK_vswitch3",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_LOOPBACK_vswitch3",
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "111.111.111.2/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.2.2",
    "VppEntryType": "l3vrf",
    "L3Route": {
      "description": "L3VRF_VXLAN Node:router-a to Node:vswitch2",
      "dst_ip_addr": "111.111.111.3/24",
      "next_hop_addr": "10.100.2.2",
      "outgoing_interface": "gigethernet13/0/a",
      "preference": 5
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch2/vpp/config/v1/interface/IF_VXLAN_LOOPBACK_vswitch2",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_LOOPBACK_vswitch2",
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "111.111.111.3/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch1/vpp/config/v1/interface/gigethernet13/0/1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "gigethernet13/0/1",
      "type": 1,
      "enabled": true,
      "mtu": 1500,
      "ip_addresses": [
        "10.100.1.1/24"
      ]
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch2/vpp/config/v1/interface/IF_VXLAN_FROM_SPOKE_vswitch2_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_FROM_SPOKE_vswitch2_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000",
      "type": 5,
      "enabled": true,
      "vxlan": {
        "src_address": "111.111.111.3",
        "dst_address": "111.111.111.1",
        "vni": 6000
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch2/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.4.4",
    "VppEntryType": "l3vrf",
    "L3Route": {
      "description": "L3VRF_VXLAN Node:vswitch2 to Node:router-a",
      "dst_ip_addr": "111.111.111.1/24",
      "next_hop_addr": "10.100.4.4",
      "outgoing_interface": "gigethernet13/0/2",
      "preference": 5
    }
  },
  {
    "VppKey": "/vnf-agent/vnf2/vpp/config/v1/interface/port1",
    "VppEntryType": "interface",
    "IFace": {
      "name": "port1",
      "type": 2,
      "enabled": true,
      "mtu": 1500,
      "memif": {
        "id": 2,
        "socket_filename": "/tmp/memif_vswitch2.sock"
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/interface/IF_VXLAN_FROM_SPOKE_vswitch3_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_FROM_SPOKE_vswitch3_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000",
      "type": 5,
      "enabled": true,
      "vxlan": {
        "src_address": "111.111.111.2",
        "dst_address": "111.111.111.1",
        "vni": 6000
      }
    }
  },
  {
    "VppKey": "/vnf-agent/vswitch3/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.4.4",
    "VppEntryType": "l3vrf",
    "L3Route": {
      "description": "L3VRF_VXLAN Node:vswitch3 to Node:router-a",
      "dst_ip_addr": "111.111.111.1/24",
      "next_hop_addr": "10.100.4.4",
      "outgoing_interface": "gigethernet13/0/3",
      "preference": 5
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.3.3",
    "VppEntryType": "l3vrf",
    "L3Route": {
      "description": "L3VRF_VXLAN Node:router-a to Node:vswitch3",
      "dst_ip_addr": "111.111.111.2/24",
      "next_hop_addr": "10.100.3.3",
      "outgoing_interface": "gigethernet13/0/a",
      "preference": 5
    }
  },
  {
    "VppKey": "/vnf-agent/router-a/vpp/config/v1/interface/IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch3_VSRVC_service1_CONN_0_VNI_6000",
    "VppEntryType": "interface",
    "IFace": {
      "name": "IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch3_VSRVC_service1_CONN_0_VNI_6000",
      "type": 5,
      "enabled": true,
      "vxlan": {
        "src_address": "111.111.111.1",
        "dst_address": "111.111.111.2",
        "vni": 6000
      }
    }
  }
]


```

The etcd /sfc-controller subtree look like:

```

/sfc-controller/v2/config/vnf-service-mesh/router-a-hub-and-spoke
{"name":"router-a-hub-and-spoke","service_mesh_type":"hub_and_spoke","connection_type":"vxlan","vxlan_hub_and_spoke_parms":{"hub_node_name":"router-a","vni":6000,"loopback_ipam_pool_name":"vxlan_loopback_pool","outgoing_interface_label":"vxlan"}}
/sfc-controller/v2/config/node/router-a
{"name":"router-a","node_type":"external","interfaces":[{"name":"gigethernet13/0/a","if_type":"ethernet","ip_addresses":["10.100.4.4/24"],"custom_labels":["vxlan"]}]}
/sfc-controller/v2/config/node/vswitch1
{"name":"vswitch1","node_type":"host","interfaces":[{"name":"gigethernet13/0/1","if_type":"ethernet","ip_addresses":["10.100.1.1/24"],"custom_labels":["vxlan"]}]}
/sfc-controller/v2/config/node/vswitch2
{"name":"vswitch2","node_type":"host","interfaces":[{"name":"gigethernet13/0/2","if_type":"ethernet","ip_addresses":["10.100.2.2/24"],"custom_labels":["vxlan"]}]}
/sfc-controller/v2/config/node/vswitch3
{"name":"vswitch3","node_type":"host","interfaces":[{"name":"gigethernet13/0/3","if_type":"ethernet","ip_addresses":["10.100.3.3/24"],"custom_labels":["vxlan"]}]}
/sfc-controller/v2/config/system-parameters
{"mtu":1500,"default_static_route_preference":5,"ipam_pools":[{"name":"vxlan_loopback_pool","network":"111.111.111.0/24"}]}
/sfc-controller/v2/config/vnf-service/service1
{"name":"service1","vnfs":[{"name":"vnf1","vnf_type":"vppcontainer","interfaces":[{"name":"port1","if_type":"memif"}]},{"name":"vnf2","vnf_type":"vppcontainer","interfaces":[{"name":"port1","if_type":"memif"}]},{"name":"vnf3","vnf_type":"vppcontainer","interfaces":[{"name":"port1","if_type":"memif"}]}],"connections":[{"conn_type":"l2mp","vnf_service_mesh":"router-a-hub-and-spoke","interfaces":[{"vnf":"vnf1","interface":"port1"},{"vnf":"vnf2","interface":"port1"},{"vnf":"vnf3","interface":"port1"}]}]}
/sfc-controller/v2/config/vnf-to-node/vnf1
{"vnf":"vnf1","node":"vswitch3"}
/sfc-controller/v2/config/vnf-to-node/vnf2
{"vnf":"vnf2","node":"vswitch2"}
/sfc-controller/v2/config/vnf-to-node/vnf3
{"vnf":"vnf3","node":"vswitch3"}
/sfc-controller/v2/status/node/router-a
{"name":"router-a","oper_status":"OperUp","msg":["OK"],"rendered_vpp_agent_entries":[{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/interface/gigethernet13/0/a","vpp_agent_type":"interface"}]}
/sfc-controller/v2/status/node/vswitch1
{"name":"vswitch1","oper_status":"OperUp","msg":["OK"],"rendered_vpp_agent_entries":[{"vpp_agent_key":"/vnf-agent/vswitch1/vpp/config/v1/interface/gigethernet13/0/1","vpp_agent_type":"interface"}]}
/sfc-controller/v2/status/node/vswitch2
{"name":"vswitch2","oper_status":"OperUp","msg":["OK"],"rendered_vpp_agent_entries":[{"vpp_agent_key":"/vnf-agent/vswitch2/vpp/config/v1/interface/gigethernet13/0/2","vpp_agent_type":"interface"}]}
/sfc-controller/v2/status/node/vswitch3
{"name":"vswitch3","oper_status":"OperUp","msg":["OK"],"rendered_vpp_agent_entries":[{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/interface/gigethernet13/0/3","vpp_agent_type":"interface"}]}
/sfc-controller/v2/status/vnf-service/service1
{"name":"service1","oper_status":"OperUp","msg":["OK"],"rendered_vpp_agent_entries":[{"vpp_agent_key":"/vnf-agent/vnf1/vpp/config/v1/interface/port1","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/interface/IF_MEMIF_VSWITCH_vnf1_port1","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vnf2/vpp/config/v1/interface/port1","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch2/vpp/config/v1/interface/IF_MEMIF_VSWITCH_vnf2_port1","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vnf3/vpp/config/v1/interface/port1","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/interface/IF_MEMIF_VSWITCH_vnf3_port1","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/interface/IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch3_VSRVC_service1_CONN_0_VNI_6000","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/interface/IF_VXLAN_LOOPBACK_router-a","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.3.3","vpp_agent_type":"l3vrf"},{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/interface/IF_VXLAN_FROM_SPOKE_vswitch3_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/interface/IF_VXLAN_LOOPBACK_vswitch3","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.4.4","vpp_agent_type":"l3vrf"},{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/interface/IF_VXLAN_FROM_HUB_router-a_TO_SPOKE_vswitch2_VSRVC_service1_CONN_0_VNI_6000","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.2.2","vpp_agent_type":"l3vrf"},{"vpp_agent_key":"/vnf-agent/vswitch2/vpp/config/v1/interface/IF_VXLAN_FROM_SPOKE_vswitch2_TO_HUB_router-a_VSRVC_service1_CONN_0_VNI_6000","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch2/vpp/config/v1/interface/IF_VXLAN_LOOPBACK_vswitch2","vpp_agent_type":"interface"},{"vpp_agent_key":"/vnf-agent/vswitch2/vpp/config/v1/vrf/0/fib/111.111.111.0/24/10.100.4.4","vpp_agent_type":"l3vrf"},{"vpp_agent_key":"/vnf-agent/vswitch2/vpp/config/v1/bd/L2BD_service1_CONN_1","vpp_agent_type":"l2bd"},{"vpp_agent_key":"/vnf-agent/vswitch3/vpp/config/v1/bd/L2BD_service1_CONN_1","vpp_agent_type":"l2bd"},{"vpp_agent_key":"/vnf-agent/router-a/vpp/config/v1/bd/L2BD_service1_CONN_1","vpp_agent_type":"l2bd"}]}



```


[0]: https://github.com/ligato/vpp-agent
[1]: https://fd.io/
[2]: https://github.com/ligato/cn-infra/blob/master/docs/readmes/cn_virtual_function.md
[3]: https://github.com/ligato/sfc-controller/tree/master/controller/cnpdriver
[4]: https://github.com/ligato/cn-infra
[5]: https://github.com/ligato/cn-infra/tree/master/core
[6]: https://hub.docker.com/r/ligato/sfc-controller/
[7]: docker/dev_vpp_agent/README.md#running-etcd-server-on-local-host
[8]: https://github.com/ligato/vpp-agent#quickstart