# SFC Controller Topology Examples

Please note that the content of this page is currently **WORK IN PROGRESS**.

The controller is capable of supporting various inter-node mesh types,
interface types, and connectivity options.  Examples of mesh types are:
vxlan mesh, srv6, ipv4, ipv6, dot1q vlans, ipsec.  Interface types such as
memif, veth, and tap, and connectivity options are l2pp and l2mp.

The controller adapts dynamically to vnf to host changes. When K8s places a
vnf on a host, the controller will adjust the configuration based on where
the vnf is placed and how it relates to other vnfs in the vnf service 
definition.

## L2 P2P VNFs chained on same host and vxlan mesh between hosts

See [here](vxlanmesh/l2pp/vxlanl2pp.md) for an expmple of a layer 2
point to point port chained vnf-service.  The toplogy starts out by importing
the yaml file where each vnf is on the same host.  It then adapts to changes
where the vnfs are placed on separate hosts.

## L2 P2MP VNFs bridged on same host and vxlan mesh between hosts

See [here](vxlanmesh/l2mp/vxlanl2mp.md) for an example of a layer 2
point to multi-point port bridged vnf-service.  The toplogy starts out by importing
the yaml file where each vnf is on the same host.  It then adapts to changes
where the vnfs are placed on separate hosts.

## L2 P2MP VNFs bridged via vxlan tunnels:  external router (hub), hosts (spokes)

See [here](vxlanhubandspoke/l2mp/vxlanl2mp.md) for an example of a layer 2 service
where an external router is used as the hub of a hub and spoke vxlan tunnel
mesh where 2 of teh VNFs are on one host, and another is on a difernt host.
There are two vxlan tunnels from the hub to the hosts.
