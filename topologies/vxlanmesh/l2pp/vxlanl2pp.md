# SFC Controller VxLAN L2PP example

Please note that the content of this page is currently **WORK IN PROGRESS**.

This example describes a 3 host scenario with 3 VNF's.  A VNF L2 P2P service
is described which chains ports from vnf1, to vnf2 to vnf3.  The vnf's can
be hosted on any combination of the hosts.  If the vnf's are hosted on the
same host, then the chain can be rendered with direct memif links between
the hosts.  Otherwise, the inter-vnf connections require a vxlan tunnel.
Note that the L2PP vnf service uses L2 cross-connected to chain the ports
together vs an L2MP vnf service would require a bridging solution as the
inter-vnf traffic can go from any nf port to any other vnf port in the connection
set.

This scenario illustrates how the sfc-controller adapts the service based on
changes in the topology.  Currently, to change the vnf-to-host mapping, an
HTTP REST request is used.  Kubernetes will also be able to effect changes
to this mapping once vnf's are scheduled by K8S.  The controller listens to
changes to K8S and adapts the L2 service to the topolgy changes.

1) Start the controller with a yaml file where the vnf's are on the same host.
2) Send an HTTP POST changing where the vnf's are mapped, one vnf per host.
3) Send an HTTP POST changing where the vnf's are mapped, vnf's not on a host.  

## VNF's on Same Host

![VNF Same Host](vxlan_1.png "VNF Same Host")

1. Start up the controller with a yaml file (vnf's on same host)

```
sfc-controller -sfc-config=toplogies/vxlanmesh/l2pp/vxlanl2pp.yaml -clean
```

The yaml file looks like:
```

    sfc_controller_config_version: 2
    description: 3 node with 1 nic port, host-host vxlan mesh, vnf on each node
    
    system_parameters:
      memif_directory: "/run/vpp" # a common folder where all vnf's and vswitches are mounted

    ipam_pools:  # allocate internode vxlan mesh enpoints from a pool
      - metadata:
          name: vxlan_tunnel_pool
          labels:
        spec:
          scope: system
          network: 111.111.111.0/24
          start_range: 1
          end_range: 3

    network_pod_to_node_map:
      - pod: vnf1
        node: vswitch1
      - pod: vnf2
        node: vswitch1
      - pod: vnf3
        node: vswitch1
        
    network_nodes:
      - metadata:
          name: vswitch1
        spec:
          node_type: host
          interfaces:
            - name: GigabitEthernet0/8/1
              if_type: ethernet
              ip_addresses:
                - "10.100.1.1/24"
              labels:
                - vxlan # outgoing i/f for static route from the vxlan loopback address to remote vxlan dest(s)
      - metadata:
          name: vswitch2
        spec:
          node_type: host
          interfaces:
            - name: GigabitEthernet0/8/2
              if_type: ethernet
              ip_addresses:
                - "10.100.2.2/24"
              labels:
                - vxlan # outgoing i/f for static route from the vxlan loopback address to remote vxlan dest(s)

      - metadata:
          name: vswitch3
        spec:
          node_type: host
          interfaces:
            - name: GigabitEthernet0/8/3
              if_type: ethernet
              ip_addresses:
                - "10.100.3.3/24"
              labels:
                - vxlan # outgoing i/f for static route from the vxlan loopback address to remote vxlan dest(s)

    network_services:
      - metadata:
          name: my-network-service
        spec:
          network_pods:
            - metadata:
                name: vnf1
              spec:
                pod_type: vppcontainer
                interfaces:
                  - name: port1
                    if_type: memif
                  - name: port2
                    if_type: memif
            - metadata:
                name: vnf2
              spec:
                pod_type: vppcontainer
                interfaces:
                  - name: port1
                    if_type: memif
                  - name: port2
                    if_type: memif                    
            - metadata:
                name: vnf3
              spec:
                pod_type: vppcontainer
                interfaces:
                  - name: port1
                    if_type: memif
                  - name: port2
                    if_type: memif                  
          connections:
            - conn_type: l2pp # l2x ports on same vnf
              pod_interfaces:
                - vnf1/port1
                - vnf2/port2
            - conn_type: l2pp
              network_node_overlay_name: inter_node_vxlan_mesh
              pod_interfaces:
                - vnf1/port2
                - vnf2/port1
            - conn_type: l2pp # l2x ports on same vnf
              pod_interfaces:
                - vnf2/port1
                - vnf2/port2                
            - conn_type: l2pp
              network_node_overlay_name: inter_node_vxlan_mesh
              pod_interfaces:
                - vnf2/port2
                - vnf3/port1
            - conn_type: l2pp # l2x ports on same vnf
              pod_interfaces:
                - vnf3/port1
                - vnf3/port2                  

    network_node_overlays:
      - metadata:
          name:
            inter_node_vxlan_mesh
        spec:
          service_mesh_type: mesh
          connection_type: vxlan
          vxlan_mesh_parms:
            vni_range_start: 5000
            vni_range_end: 5999
            loopback_ipam_pool_name: vxlan_tunnel_pool
            create_loopback_interface: true
            create_loopback_static_routes: true
            

```

The sfc-controller creates memif's on each of the specified ports in the network service.  As a hack
for testing the datapath, the ports in the vnf can be wired together via L2 x-connects so data can
be driven into the vnf1/port1, and easily traverse the pods to vnf3/port2.  The inter-pod connectivity
is effected by the creation of memif's between the pods.  Note: if a direct memif is not desired and
the vswitch is required to transport the traffic between the vnf's, the use the conn_method: vswitch
in the yaml.  Also, if the if_type is veth or tap, then the connectivity will go through the vswitch.


## VNF's on Three Different Hosts

![VNF Three Hosts](vxlan_3.png "VNF Three Hosts")

1. Send an HTTP POST curl command to the controller

```
curl -X POST -d @- << EOF http://0.0.0.0:9191/sfc-controller/v2/config/network_pod_to_node_map
[
  {
    "pod": "vnf1",
    "node": "vswitch1"
  },
  {
    "pod": "vnf2",
    "node": "vswitch2"
  },
  {
    "pod": "vnf3",
    "node": "vswitch3"
  }
]
EOF
```

The point to point traffic in this case will need to traverse hosts so a memif interface is created on the
vswitch which pairs with the memif in the vnf, then a host to host vxlan tunnel is created.  An l2
x-connect is made beween the vswitch memif to the vxlan vtep.  The "chain" is effectively formed by
creating the series of intra-pod l2x connects, and inter-host vxlan tunnels.



## VNF's not on any hosts

![VNF No Hosts](vxlan_0.png "VNF No Hosts")

1. Send an HTTP POST curl command to the controller

```
curl -X POST -d @- << EOF http://0.0.0.0:9191/sfc-controller/v2/config/network_pod_to_node_map
[
  {
    "pod": "vnf1"
  },
  {
    "pod": "vnf2"
  },
  {
    "pod": "vnf3"
  }
]
EOF
```
The sfc-controller cleans up the configuration in etcd, as it does not know where the vnf's
are hosted.

[0]: https://github.com/ligato/vpp-agent
[1]: https://fd.io/
[2]: https://github.com/ligato/cn-infra/blob/master/docs/readmes/cn_virtual_function.md
[3]: https://github.com/ligato/sfc-controller/tree/master/controller/cnpdriver
[4]: https://github.com/ligato/cn-infra
[5]: https://github.com/ligato/cn-infra/tree/master/core
[6]: https://hub.docker.com/r/ligato/sfc-controller/
[7]: docker/dev_vpp_agent/README.md#running-etcd-server-on-local-host
[8]: https://github.com/ligato/vpp-agent#quickstart