// Copyright (c) 2018 Cisco and/or its affiliates.
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

package controller

import (
	"github.com/golang/protobuf/proto"
	"github.com/ligato/sfc-controller/plugins/controller/database"
	"github.com/ligato/sfc-controller/plugins/controller/model"
)

// Nodes and NetworkServices have interface definitions so an entity is either a node or netservice

func InitInterfaceStatus(
	entityName string,
	vppAgent string,
	entityInterface *controller.Interface) (*controller.InterfaceStatus, error) {

	entityInterfaceName := entityInterface.Parent + "/" + entityInterface.Name

	ifStatus, exists := ctlrPlugin.ramCache.InterfaceStates[entityInterfaceName]
	if !exists {
		ifStatus = &controller.InterfaceStatus{
			Name: entityInterfaceName,
			Node: vppAgent,
			IpamPoolNums: make(map[string]uint32,0),
		}
	}

	if entityInterface.MacAddress == "" {
		if ifStatus.MacAddress == "" {
			ifStatus.MacAddress, ifStatus.MacAddrID =
				ctlrPlugin.ramCache.MacAddrAllocator.Allocate()
		}
	} else {
		if ifStatus.MacAddress != entityInterface.MacAddress {
			ifStatus.MacAddress = entityInterface.MacAddress
		}
	}

	// rebuild the status ip addresses from the statically defined ones, then add the ones allocated
	// already from pools, then if pool addreesses are not allocated yet, then allocate as well
	ifStatus.IpAddresses = entityInterface.IpAddresses
	// for all already allocated from pool ip addresses, set as used in the allocators
	for poolName, ipNum := range ifStatus.IpamPoolNums {
		// if already have an allocated ipNum, ensure it is marked as used/set in the allocator
		ipAddress, err := ctlrPlugin.IpamPoolMgr.SetAddress(poolName, vppAgent, entityName, ipNum)
		if err != nil {
			return nil, err
		}
		ifStatus.IpAddresses = append(ifStatus.IpAddresses, ipAddress)
	}
	for _, poolName := range entityInterface.IpamPoolNames {
		if _, exists := ifStatus.IpamPoolNums[poolName]; !exists {
			ipAddress, ipNum, err := ctlrPlugin.IpamPoolMgr.AllocateAddress(poolName,
				vppAgent, entityName)
			if err != nil {
				return nil, err
			}
			ifStatus.IpAddresses = append(ifStatus.IpAddresses, ipAddress)
			ifStatus.IpamPoolNums[poolName] = ipNum
		}
	}

	for _, ipAddress := range ifStatus.IpAddresses {
		// make sure we set any static addresses in any of the pools, in case of overlap
		for _, ipamPool := range ctlrPlugin.IpamPoolMgr.ipamPoolCache {
			// if already have an allocated ipNum, ensure it is marked as used/set in the allocator
			ctlrPlugin.IpamPoolMgr.SetAddressIfInPool(ipamPool.Metadata.Name, vppAgent, entityName, ipAddress)
		}
	}

	log.Debugf("InitInterfaceStatus: %v", ifStatus)

	return ifStatus, nil
}

func PersistInterfaceStatus(
	interfaces map[string]*controller.InterfaceStatus,
	ifStatus *controller.InterfaceStatus,
	podName string, ifName string) {

	entityInterfaceName := 	podName + "/" + ifName
	interfaceKey := InterfaceKeyPrefix() + entityInterfaceName

	interfaces[entityInterfaceName] = ifStatus
	ctlrPlugin.ramCache.InterfaceStates[entityInterfaceName] = ifStatus
	database.WriteToDatastore(interfaceKey, ifStatus)

	log.Debugf("PersistInterfaceStatus: entityInterfaceName: %s, %v", entityInterfaceName, ifStatus)

}

func RetrieveInterfaceStatusFromRamCache(
	interfaces map[string]*controller.InterfaceStatus,
	podInterfaceName string) (*controller.InterfaceStatus, bool) {

	ifStatus ,exists := interfaces[podInterfaceName]

	log.Debugf("RetrieveInterfaceStatusFromRamCache: podInterfaceName: %s, value: %v, exists: %b",
		podInterfaceName, ifStatus, exists)

	return ifStatus, exists

}

func RemoveInterfaceStatus(
	interfaces map[string]*controller.InterfaceStatus,
	podName string, ifName string) {

	entityInterfaceName := 	podName + "/" + ifName
	interfaceKey := InterfaceKeyPrefix() + entityInterfaceName

	delete(interfaces, entityInterfaceName)
	delete(ctlrPlugin.ramCache.InterfaceStates, entityInterfaceName)
	database.DeleteFromDatastore(interfaceKey)

	log.Debugf("RemoveInterfaceStatus: entityInterfaceName: %s, %v", entityInterfaceName)
}

func UpdateRamCacheAllocatorsForInterfaceStatus(
	ifStatus *controller.InterfaceStatus,
	entityName string) error {

	// do not need to worry about setting the mac/memif id as we simply increment those so no need
	// to track them in the "allocator" ... which is really an "incrementor"

	// for all already allocated from pool ip addresses, set as used in the allocators
	for poolName, ipNum := range ifStatus.IpamPoolNums {
		// if already have an allocated ipNum, ensure it is marked as used/set in the allocator
		_, err := ctlrPlugin.IpamPoolMgr.SetAddress(poolName, ifStatus.Node, entityName, ipNum)
		if err != nil {
			return err
		}
	}

	for _, ipAddress := range ifStatus.IpAddresses {
		// make sure we set any static addresses in any of the pools
		for _, ipamPool := range ctlrPlugin.IpamPoolMgr.ipamPoolCache {
			// if already have an allocated ipNum, ensure it is marked as used/set in the allocator
			ctlrPlugin.IpamPoolMgr.SetAddressIfInPool(ipamPool.Metadata.Name, ifStatus.Node, entityName, ipAddress)
		}
	}

	return nil
}

// KeyPrefix provides sfc controller's interface
func InterfaceKeyPrefix() string {
	return controller.SfcControllerStatusPrefix() + "interface/"
}

// LoadAllInterfacesFromDatastoreIntoCache iterates over the etcd set
func LoadAllInterfacesFromDatastoreIntoCache() error {
	log.Debugf("LoadAllInterfacesFromDatastoreIntoCache: ...")
	return loadAllInterfacesFromDatastore(ctlrPlugin.ramCache.InterfaceStates)
}

// loadAllInterfacesFromDatastore iterates over the etcd set
func loadAllInterfacesFromDatastore(iFaces map[string]*controller.InterfaceStatus) error {
	var iFace *controller.InterfaceStatus
	return database.ReadIterate(InterfaceKeyPrefix(),
		func() proto.Message {
			iFace = &controller.InterfaceStatus{}
			return iFace
		},
		func(data proto.Message) {
			iFaces[iFace.Name] = iFace
			//log.Debugf("loadAllInterfacesFromDatastore: n=%v", ip)
		})
}