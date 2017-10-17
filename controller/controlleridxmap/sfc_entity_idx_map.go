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

// SfcEntityIdxMap is basically a set of Host Entities indexed
// primary by entity name with secondary indexes: IpAddr, MacAddr and PortId.
type SfcEntityIdxMap interface {
	// GetMapping returns internal read-only mapping with Value
	// of type interface{}.
	GetMapping() idxmap.NamedMapping

	// GetValue looks up previously stored entity by it's name.
	GetValue(entityName string) (data *controller.SfcEntity, exists bool)

	// ListValues all stored SFC Entities
	ListValues() []*controller.SfcEntity

	// LookupByIpAddr returns SFC Entity names of items that contains given IP address in Value
	LookupByIpAddr(ipAddrPrefix string) []string

	// LookupByMacAddr returns SFC Entity names of items that contains given MAC address in Value
	LookupByMacAddr(macAddress string) []string

	// LookupByPortId returns SFC Entity names of items that contains given IP address in Value
	LookupByPortId(portId uint32) []string

	// WatchNameToIdx allows to subscribe for watching changes in sfcEntityMap
	// mapping.
	WatchNameToIdx(subscriber core.PluginName, callback func(*SfcEntityEvent))
}

// ToChanSfcEntityEvent is helper function that enables to receive events from WatchNameToIdx into channel
// instead of direct callback
func ToChanSfcEntityEvent(channel chan *SfcEntityEvent) func(*SfcEntityEvent) {
	return func(event *SfcEntityEvent) {
		channel <- event
	}
}

// SfcEntityIdxMapRW exposes not only SfcEntityIdxMap but also write methods.
type SfcEntityIdxMapRW interface {
	SfcEntityIdxMap

	// Put adds new item into name-to-index mapping.
	Put(sfcEntity *controller.SfcEntity)

	// Delete removes an item identified by name from mapping
	Delete(entityName string) (data *controller.SfcEntity, exists bool)
}

// NewSfcEntityMap is a constructor for SfcEntityIdxMapRW.
func NewSfcEntityMap(owner core.PluginName) SfcEntityIdxMapRW {
	return &sfcEntityMap{mapping: mem.NewNamedMapping(logroot.StandardLogger(),
		owner, "plugin status", IndexSfcEntity)}
}

// sfcEntityMap is a type-safe implementation of SfcEntityIdxMap(RW).
type sfcEntityMap struct {
	mapping idxmap.NamedMappingRW
}

// SfcEntityEvent represents an item sent through the watch channel
// in SfcEntityMap.WatchNameToIdx().
// In contrast to NameToIdxDto it contains a typed Value.
type SfcEntityEvent struct {
	idxmap.NamedMappingEvent
	Value *controller.SfcEntity
}

const (
	ipv4AdrKey = "ipv4AdrKey"
	macAddrKey = "macAddrKey"
	portIdKey  = "portIdKey"
)

// GetMapping returns internal read-only mapping.
// It is used in tests to inspect the content of the sfcEntityMap.
func (swi *sfcEntityMap) GetMapping() idxmap.NamedMapping {
	return swi.mapping
}

// Put adds new item into the name-to-index mapping.
// <sfcEntity> entity that will be written to the map (Sfc Entity contains name that identifies this entity)
func (swi *sfcEntityMap) Put(sfcEntity *controller.SfcEntity) {
	swi.mapping.Put(sfcEntity.Name, sfcEntity)
}

// IndexSfcEntity creates indexes for plugin states and records the state
// passed as untyped data.
func IndexSfcEntity(data interface{}) map[string][]string {
	logroot.StandardLogger().Debug("IndexSfcEntity ", data)

	indexes := map[string][]string{}
	sfcEntity, ok := data.(*controller.SfcEntity)
	if !ok || sfcEntity == nil {
		return indexes
	}

	ipv4Prefix := sfcEntity.SfcIpv4Prefix
	ipv4PrefixLen := sfcEntity.SfcIpv4PrefixLen

	if ipv4Prefix != "" && ipv4PrefixLen != 0 {
		indexedValue := fmt.Sprintf("%s/%d", ipv4Prefix, ipv4PrefixLen)
		indexes[ipv4AdrKey] = []string{indexedValue}
	}

	macAddrs := map[string]interface{}{}
	portIds := map[uint32]interface{}{}
	for _, sfcEl := range sfcEntity.Elements {
		if sfcEl.MacAddr != "" {
			macAddrs[sfcEl.MacAddr] = nil
		}

		if sfcEl.PortId != 0 {
			portIds[sfcEl.PortId] = nil
		}
	}
	macAddrList := make([]string, len(macAddrs))
	m := 0
	for macAddr := range macAddrs {
		macAddrList[m] = macAddr
		m++
	}
	portIdList := make([]string, len(portIds))
	p := 0
	for portId := range portIds {
		portIdList[p] = fmt.Sprintf("%d", portId)
		p++
	}
	indexes[macAddrKey] = macAddrList
	indexes[portIdKey] = portIdList

	return indexes
}

// Delete removes an item identified by name from mapping
func (swi *sfcEntityMap) Delete(entityName string) (data *controller.SfcEntity, exists bool) {
	meta, exists := swi.mapping.Delete(entityName)
	return swi.castdata(meta), exists
}

// GetValue looks up previously stored item identified by index in mapping.
func (swi *sfcEntityMap) GetValue(entityName string) (data *controller.SfcEntity, exists bool) {
	meta, exists := swi.mapping.GetValue(entityName)
	if exists {
		data = swi.castdata(meta)
	}
	return data, exists
}

// ListValues all values stored SFC Entities
func (swi *sfcEntityMap) ListValues() []*controller.SfcEntity {
	ret := []*controller.SfcEntity{}
	for _, entityName := range swi.mapping.ListAllNames() {
		meta, exists := swi.mapping.GetValue(entityName)
		if exists {
			ret = append(ret, swi.castdata(meta))
		}
	}

	return ret
}

// LookupByIpAddr returns SFC Entity names of items that contains given IP address in Value
func (swi *sfcEntityMap) LookupByIpAddr(ipAddrPrefix string) []string {
	return swi.mapping.ListNames(ipv4AdrKey, ipAddrPrefix)
}

// LookupByMacAddr returns SFC Entity names of items that contains given MAC address in Value
func (swi *sfcEntityMap) LookupByMacAddr(macAddress string) []string {
	return swi.mapping.ListNames(macAddrKey, macAddress)
}

// LookupByPortId SFC Entity returns names of items that contains given IP address in Value
func (swi *sfcEntityMap) LookupByPortId(portId uint32) []string {
	return swi.mapping.ListNames(portIdKey, fmt.Sprintf("%d", portId))
}

func (swi *sfcEntityMap) castdata(meta interface{}) *controller.SfcEntity {
	if sfcEntity, ok := meta.(*controller.SfcEntity); ok {
		return sfcEntity
	}

	return nil
}

// WatchNameToIdx allows to subscribe for watching changes in sfcEntityMap mapping
func (swi *sfcEntityMap) WatchNameToIdx(subscriber core.PluginName, callback func(*SfcEntityEvent)) {
	swi.mapping.Watch(subscriber, func(event idxmap.NamedMappingGenericEvent) {
		callback(&SfcEntityEvent{
			NamedMappingEvent: event.NamedMappingEvent,
			Value:             swi.castdata(event.Value),
		})
	})
}
