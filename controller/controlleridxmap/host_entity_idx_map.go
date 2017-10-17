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

package controlleridxmap

import (
	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"fmt"
)

// HostEntityIdxMap is basically a set of Host Entities indexed
// primary by entity name with secondary indexes: VlanID, MacAddr.
type HostEntityIdxMap interface {
	// GetMapping returns internal read-only mapping with Value
	// of type interface{}.
	GetMapping() idxmap.NamedMapping

	// GetValue looks up previously stored entity by it's name.
	GetValue(entityName string) (data *controller.HostEntity, exists bool)

	// ListValues all values stored Host Entities
	ListValues() []*controller.HostEntity

	// LookupByVlandId returns Host Entity names of items that contains given VlanID (VNI)
	LookupByVlandId(vlandId uint32) []string

	// LookupByMacAddr returns Host Entity names of items that contains given looback mac address
	LookupByMacAddr(macAddr string) []string

	// WatchNameToIdx allows to subscribe for watching changes in hostEntityMap
	// mapping.
	// <subscriber> caller of the API
	// <pluginChannel>
	WatchNameToIdx(subscriber core.PluginName, callback func(*HostEntityEvent))
}

// ToChanHostEntityEvent is helper function that enables to receive events from WatchNameToIdx into channel
// instead of direct callback
func ToChanHostEntityEvent(channel chan *HostEntityEvent) func(*HostEntityEvent) {
	return func(event *HostEntityEvent) {
		channel <- event
	}
}

// HostEntityIdxMapRW exposes not only HostEntityIdxMap but also write methods.
type HostEntityIdxMapRW interface {
	HostEntityIdxMap

	// RegisterName adds new item into name-to-index mapping.
	Put(hostEntity *controller.HostEntity)

	// Delete removes an item identified by name from mapping
	Delete(entityName string) (data *controller.HostEntity, exists bool)
}

// NewHostEntityMap is a constructor for HostEntityIdxMapRW.
func NewHostEntityMap(owner core.PluginName) HostEntityIdxMapRW {
	return &hostEntityMap{mapping: mem.NewNamedMapping(logroot.StandardLogger(),
		owner, "plugin status", IndexHostEntity)}
}

// hostEntityMap is a type-safe implementation of HostEntityIdxMap(RW).
type hostEntityMap struct {
	mapping idxmap.NamedMappingRW
}

// HostEntityEvent represents an item sent through the watch channel
// in HostEntityMap.WatchNameToIdx().
// In contrast to NameToIdxDto it contains a typed Value.
type HostEntityEvent struct {
	idxmap.NamedMappingEvent
	Value *controller.HostEntity
}

const (
	vlandIdKey = "vlanIdKey"
)

// GetMapping returns internal read-only mapping.
// It is used in tests to inspect the content of the hostEntityMap.
func (swi *hostEntityMap) GetMapping() idxmap.NamedMapping {
	return swi.mapping
}

// Put adds new item into the name-to-index mapping.
// <hostEntity> entity that will be written to the map (Host Entity contains name that identifies this entity)
func (swi *hostEntityMap) Put(hostEntity *controller.HostEntity) {
	swi.mapping.Put(hostEntity.Name, hostEntity)
}

// IndexHostEntity creates indexes for plugin states and records the state
// passed as untyped data.
func IndexHostEntity(data interface{}) map[string][]string {
	logroot.StandardLogger().Debug("IndexHostEntity ", data)

	indexes := map[string][]string{}
	hostEntity, ok := data.(*controller.HostEntity)
	if !ok || hostEntity == nil {
		return indexes
	}

	vni := hostEntity.Vni
	if vni != 0 {
		indexes[vlandIdKey] = []string{fmt.Sprintf("%s", vni)}
	}
	return indexes
}

// Delete removes an item identified by name from mapping
func (swi *hostEntityMap) Delete(entityName string) (data *controller.HostEntity, exists bool) {
	meta, exists := swi.mapping.Delete(entityName)
	return swi.castdata(meta), exists
}

// GetValue looks up previously stored item identified by index in mapping.
func (swi *hostEntityMap) GetValue(entityName string) (data *controller.HostEntity, exists bool) {
	meta, exists := swi.mapping.GetValue(entityName)
	if exists {
		data = swi.castdata(meta)
	}
	return data, exists
}

// ListValues all values stored Host Entities
func (swi *hostEntityMap) ListValues() []*controller.HostEntity {
	ret := []*controller.HostEntity{}
	for _, entityName := range swi.mapping.ListAllNames() {
		meta, exists := swi.mapping.GetValue(entityName)
		if exists {
			ret = append(ret, swi.castdata(meta))
		}
	}

	return ret
}

// LookupByVlandId returns names of items that contains given VlanID (VNI) in Value
func (swi *hostEntityMap) LookupByVlandId(vlandId uint32) []string {
	return swi.mapping.ListNames(vlandIdKey, fmt.Sprintf("%d", vlandId))
}

// LookupByMacAddr returns names of items that contains given MAC address in Value
func (swi *hostEntityMap) LookupByMacAddr(macAddr string) []string {
	return swi.mapping.ListNames(macAddrKey, macAddr)
}

func (swi *hostEntityMap) castdata(meta interface{}) *controller.HostEntity {
	if hostEntity, ok := meta.(*controller.HostEntity); ok {
		return hostEntity
	}

	return nil
}

// WatchNameToIdx allows to subscribe for watching changes in hostEntityMap mapping
func (swi *hostEntityMap) WatchNameToIdx(subscriber core.PluginName, callback func(*HostEntityEvent)) {
	swi.mapping.Watch(subscriber, func(event idxmap.NamedMappingGenericEvent) {
		callback(&HostEntityEvent{
			NamedMappingEvent: event.NamedMappingEvent,
			Value:             swi.castdata(event.Value),
		})
	})
}
