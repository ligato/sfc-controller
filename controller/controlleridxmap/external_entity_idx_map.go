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
)

// ExternalEntityIdxMap is basically a set of External Entities indexed
// primary by entity name (without secondary indexes).
type ExternalEntityIdxMap interface {
	// GetMapping returns internal read-only mapping with Value
	// of type interface{}.
	GetMapping() idxmap.NamedMapping

	// GetValue looks up previously stored entity by it's name.
	GetValue(entityName string) (data *controller.ExternalEntity, exists bool)

	// ListValues all stored External Entities
	ListValues() []*controller.ExternalEntity

	// WatchNameToIdx allows to subscribe for watching changes in externalEntityMap
	// mapping.
	WatchNameToIdx(subscriber core.PluginName, pluginChannel func(*ExternalEntityEvent))
}

// ToChanExternalEntityEvent is helper function that enables to receive events from WatchNameToIdx into channel
// instead of direct callback
func ToChanExternalEntityEvent(channel chan *ExternalEntityEvent) func(*ExternalEntityEvent) {
	return func(event *ExternalEntityEvent) {
		channel <- event
	}
}

// ExternalEntityIdxMapRW exposes not only ExternalEntityIdxMap but also write methods.
type ExternalEntityIdxMapRW interface {
	ExternalEntityIdxMap

	// RegisterName adds new item into name-to-index mapping.
	Put(externalEntity *controller.ExternalEntity)

	// Delete removes an item identified by name from mapping
	Delete(entityName string) (data *controller.ExternalEntity, exists bool)
}

// NewExternalEntityMap is a constructor for ExternalEntityIdxMapRW.
func NewExternalEntityMap(owner core.PluginName) ExternalEntityIdxMapRW {
	return &externalEntityMap{mapping: mem.NewNamedMapping(logroot.StandardLogger(),
		owner, controller.ExternalEntityKeyPrefix(), IndexExternalEntity)}
}

// externalEntityMap is a type-safe implementation of ExternalEntityIdxMap(RW).
type externalEntityMap struct {
	mapping idxmap.NamedMappingRW
}

// ExternalEntityEvent represents an item sent through the watch channel
// in ExternalEntityMap.WatchNameToIdx().
// In contrast to NameToIdxDto it contains a typed Value.
type ExternalEntityEvent struct {
	idxmap.NamedMappingEvent
	Value *controller.ExternalEntity
}

// GetMapping returns internal read-only mapping.
// It is used in tests to inspect the content of the externalEntityMap.
func (swi *externalEntityMap) GetMapping() idxmap.NamedMapping {
	return swi.mapping
}

// Put adds new item into the name-to-index mapping.
// <externalEntity> entity that will be written to the map (External Entity contains name that identifies this entity)
func (swi *externalEntityMap) Put(externalEntity *controller.ExternalEntity) {
	swi.mapping.Put(externalEntity.Name, externalEntity)
}

// IndexExternalEntity does nothing.
func IndexExternalEntity(data interface{}) map[string][]string {
	logroot.StandardLogger().Debug("IndexExternalEntity ", data)

	indexes := map[string][]string{}
	externalEntity, ok := data.(*controller.ExternalEntity)
	if !ok || externalEntity == nil {
		return indexes
	}

	/*state := externalEntity.State
	if state != 0 {
		indexes[stateIndexKey] = []string{state.String()}
	}*/
	return indexes
}

// Delete removes an item identified by name from mapping
func (swi *externalEntityMap) Delete(entityName string) (data *controller.ExternalEntity, exists bool) {
	meta, exists := swi.mapping.Delete(entityName)
	return swi.castdata(meta), exists
}

// GetValue looks up previously stored item identified by index in mapping.
func (swi *externalEntityMap) GetValue(entityName string) (data *controller.ExternalEntity, exists bool) {
	meta, exists := swi.mapping.GetValue(entityName)
	if exists {
		data = swi.castdata(meta)
	}
	return data, exists
}

// ListValues all values stored External Entities
func (swi *externalEntityMap) ListValues() []*controller.ExternalEntity {
	ret := []*controller.ExternalEntity{}
	for _, entityName := range swi.mapping.ListAllNames() {
		meta, exists := swi.mapping.GetValue(entityName)
		if exists {
			ret = append(ret, swi.castdata(meta))
		}
	}

	return ret
}

func (swi *externalEntityMap) castdata(meta interface{}) *controller.ExternalEntity {
	if externalEntity, ok := meta.(*controller.ExternalEntity); ok {
		return externalEntity
	}

	return nil
}

// WatchNameToIdx allows to subscribe for watching changes in externalEntityMap mapping
func (swi *externalEntityMap) WatchNameToIdx(subscriber core.PluginName, callback func(*ExternalEntityEvent)) {
	swi.mapping.Watch(subscriber, func(event idxmap.NamedMappingGenericEvent) {
		callback(&ExternalEntityEvent{
			NamedMappingEvent: event.NamedMappingEvent,
			Value:             swi.castdata(event.Value),
		})
	})
}
