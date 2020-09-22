# Project: SFC Controller REST API's

 The features of the SFC Controller can be controlled by POST/GET to supported URLs with JSON payloads.
 
 The purpose of the REST api is to CRUD objects from the SFC controller [model.](../plugins/controller/model/controller.proto)
 The controller objects will be processed and a new set of vpp agent compatible ETCD entries will be created.  The goal
 of the controller was to have a centralized view of the network and create connectivity of the interfaces in the VNF
 containers into the vswitch on each host.  If the containers connectivity spans hosts, then the inter-host connectivity
 will be also rendered into each vswitch on each host.
 
## Http GET URLs

To get a YAML dump of each of the objects in the sfc model plus the rendered set of vpp object that were created in ETCD ...

 ```
 * curl http://0.0.0.0:9191/sfc-controller
 ```

Each network node in the system must be configured.  There will be one vswitch which is attached to the NICs southbound ethernet
interfaces and to the northbound VNF container interfaces. 
To get a JSON dump of each of the network nodes ...

 ```
 * curl http://0.0.0.0:9191/sfc-controller/v2/config/network-nodes
 ```

Each ipam pool in the system must be configured.  The ipam pool is used by the interfaces to dynamically get assigned
and address.  To get a JSON dump of the IPAM pools ...

 ```
 * curl http://0.0.0.0:9191/sfc-controller/v2/config/ipam-pools
 ```
 Each pod to node mapping in the system must be configured.  This will tell the system which node/vswitch to dynammically
 create vswitch and vnf interfaces and connect them.  Then based on the connectivity options in the netowrk services,
 the controller can interconnect the VNFs. To get a JSON dump of the pod to node mappings ...
 
  ```
  * curl http://0.0.0.0:9191/sfc-controller/v2/config/network-pod-to-node-map
  ```

 Network node overlays are required so that if the pods in a network service spans network nodes, then options in
 the network node overlay will be used to connect the pods together across the network.  For example: if podA is
 mapped and running on nodeA, and podB is mapped and running on nodeB, and the overlay option is to use VxLANs
 to interconnect nodeA and nodeB, then the controller will create all the necessary plumbing ie render the 
 appropriate vpp objects to effect the connectivity between podA and podB. To get a JSON dump of the overlays ...
 
  ```
  * curl http://0.0.0.0:9191/sfc-controller/v2/config/network-node-overlays
  ```
 Network services are required to describe a set of pods, their interfaces, and how those interfaces should be connected. To get a JSON dump of the network services ...
 
  ```
  * curl http://0.0.0.0:9191/sfc-controller/v2/config/network-services
  ```

