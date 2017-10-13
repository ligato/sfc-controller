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

// The database for the SFC Controller is stroed in ETCD.  The controller uses
// the ligate/cn-infra which connects to ETCD.  The config for the controller
// is derived from the config yaml file which is passed vi command line args or
// it can be configured via HTTP REST calls.
// This module stores configuration for hosts, external routers, and the SFC's.
// The SFC's are chains or sets of containers that should be wired together.

package core

// routines to "write" the sfc controller data to the datastore.  The datastore is makes use of the ETCD infra
// to CRUD sntities of type host, exteranl, and sfc  The ETCD keys are defined in keys_controller.go

import (
	"github.com/ligato/sfc-controller/controller/model/controller"
)

// flush the ram cache to the sfc cache to the tree in etcd
func (plugin *SfcControllerPluginHandler) WriteRamCacheToEtcd() error {

	for _, ee := range plugin.ramConfigCache.EEs {
		if err := plugin.DatastoreExternalEntityPut(&ee); err != nil {
			return err
		}
	}
	for _, he := range plugin.ramConfigCache.HEs {
		if err := plugin.DatastoreHostEntityPut(&he); err != nil {
			return err
		}
	}
	for _, sfc := range plugin.ramConfigCache.SFCs {
		if err := plugin.DatastoreSfcEntityPut(&sfc); err != nil {
			return err
		}
	}

	return nil
}

// pull the sfc db from the etcd tree into the ram cache
func (plugin *SfcControllerPluginHandler) ReadEtcdDatastoreIntoRamCache() error {

	plugin.Log.Infof("ReadEtcdDatastoreIntoRamCache: start ...")

	if err := plugin.DatastoreExternalEntityRetrieveAllIntoRamCache(); err != nil {
		return err
	}
	if err := plugin.DatastoreHostEntityRetrieveAllIntoRamCache(); err != nil {
		return err
	}
	if err := plugin.DatastoreSfcEntityRetrieveAllIntoRamCache(); err != nil {
		return err
	}

	plugin.Log.Infof("ReadEtcdDatastoreIntoRamCache: end ...")

	return nil
}

// clear the sfc tree in etcd
func (plugin *SfcControllerPluginHandler) DatastoreClean() error {

	plugin.Log.Infof("DatastoreClean: clearing etc tree")

	if err := plugin.DatastoreExternalEntityDeleteAll(); err != nil {
		plugin.Log.Error("DatastoreClean: DatastoreExternalEntityDeleteAll: ", err)
	}
	if err := plugin.DatastoreHostEntityDeleteAll(); err != nil {
		plugin.Log.Error("DatastoreClean: DatastoreHostEntityDeleteAll: ", err)
	}
	if err := plugin.DatastoreSfcEntityDeleteAll(); err != nil {
		plugin.Log.Error("DatastoreClean: DatastoreSfcEntityDeleteAll: ", err)
	}

	return nil
}

// create the specified entity in the sfc db in etcd
func (plugin *SfcControllerPluginHandler) DatastoreExternalEntityPut(ee *controller.ExternalEntity) error {

	name := controller.ExternalEntityNameKey(ee.Name)

	plugin.Log.Infof("DatastoreExternalEntityPut: setting key: '%s'", name)

	err := plugin.db.Put(name, ee)
	if err != nil {
		plugin.Log.Error("DatastoreExternalEntityPut: error storing key: '%s'", name)
		plugin.Log.Error("DatastoreExternalEntityPut: databroker put: ", err)
		return err
	}
	return nil
}

// pull the specified entities from the sfc db in etcd into the sfc ram cache
func (plugin *SfcControllerPluginHandler) DatastoreExternalEntityRetrieveAllIntoRamCache() error {

	return plugin.DatastoreExternalEntityIterate(func(key string, ee *controller.ExternalEntity) {
		plugin.ramConfigCache.EEs[key] = *ee
		plugin.Log.Infof("DatastoreExternalEntityRetrieveAllIntoRamCache: adding ee: '%s': ", key, *ee)
	})
}

// remove the specified entities from the sfc db in etcd
func (plugin *SfcControllerPluginHandler) DatastoreExternalEntityDeleteAll() error {

	plugin.Log.Info("DatastoreExternalEntityDeleteAll: begin ...")
	defer plugin.Log.Info("DatastoreExternalEntityDeleteAll: exit ...")

	return plugin.DatastoreExternalEntityIterate(func(name string, ee *controller.ExternalEntity) {
		key := controller.ExternalEntityNameKey(name)
		plugin.Log.Infof("DatastoreExternalEntityDeleteAll: deleting ee: '%s': ", key, *ee)
		plugin.db.Delete(key)
	})
}

// iterate over the set of specified entities in the sfc tree in etcd
func (plugin *SfcControllerPluginHandler) DatastoreExternalEntityIterate(actionFunc func(key string,
	val *controller.ExternalEntity)) error {

	kvi, err := plugin.db.ListValues(controller.ExternalEntityKeyPrefix())
	if err != nil {
		plugin.Log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		ee := &controller.ExternalEntity{}
		err := kv.GetValue(ee)
		if err != nil {
			plugin.Log.Fatal(err)
			return nil
		}
		plugin.Log.Infof("DatastoreExternalEntityIterate: iterating ee: '%s': ", ee.Name, ee)
		actionFunc(ee.Name, ee)

	}
	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (plugin *SfcControllerPluginHandler) DatastoreExternalEntityDelete(ee *controller.ExternalEntity) error {

	return nil
}

// create the specified entity in the sfc db in etcd
func (plugin *SfcControllerPluginHandler) DatastoreHostEntityPut(he *controller.HostEntity) error {

	name := controller.HostEntityNameKey(he.Name)

	plugin.Log.Infof("DatastoreHostEntityPut: setting key: '%s'", name)

	err := plugin.db.Put(name, he)
	if err != nil {
		plugin.Log.Error("DatastoreHostEntityPut: error storing key: '%s'", name)
		plugin.Log.Error("DatastoreHostEntityPut: databroker put: ", err)
		return err
	}
	return nil
}

// pull the specified entities from the sfc db in etcd into the sfc ram cache
func (plugin *SfcControllerPluginHandler) DatastoreHostEntityRetrieveAllIntoRamCache() error {

	return plugin.DatastoreHostEntityIterate(func(key string, he *controller.HostEntity) {
		plugin.ramConfigCache.HEs[key] = *he
		plugin.Log.Infof("DatastoreHostEntityRetrieveAllIntoRamCache: adding he: '%s': ", key, *he)
	})
}

// remove the specified entities from the sfc db in etcd
func (plugin *SfcControllerPluginHandler) DatastoreHostEntityDeleteAll() error {

	plugin.Log.Info("DatastoreHostEntityDeleteAll: begin ...")
	defer plugin.Log.Info("DatastoreHostEntityDeleteAll: exit ...")

	return plugin.DatastoreHostEntityIterate(func(name string, he *controller.HostEntity) {
		key := controller.HostEntityNameKey(name)
		plugin.Log.Infof("DatastoreHostsEntityDeleteAll: deleting he: '%s': ", key, *he)
		plugin.db.Delete(key)
	})
}

// iterate over the set of specified entities in the sfc tree in etcd
func (plugin *SfcControllerPluginHandler) DatastoreHostEntityIterate(actionFunc func(key string,
	he *controller.HostEntity)) error {

	kvi, err := plugin.db.ListValues(controller.HostEntityKeyPrefix())
	if err != nil {
		plugin.Log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		he := &controller.HostEntity{}
		err := kv.GetValue(he)
		if err != nil {
			plugin.Log.Fatal(err)
			return nil
		}

		plugin.Log.Infof("DatastoreHostEntityIterate: getting he: '%s': ", he.Name, he)
		actionFunc(he.Name, he)

	}
	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (plugin *SfcControllerPluginHandler) DatastoreHostEntityDelete(ee *controller.HostEntity) error {

	return nil
}

// create the specified entity in the sfc db in etcd
func (plugin *SfcControllerPluginHandler) DatastoreSfcEntityPut(sfc *controller.SfcEntity) error {

	name := controller.SfcEntityNameKey(sfc.Name)

	plugin.Log.Infof("DatastoreSfcEntityPut: setting key: '%s'", name)

	err := plugin.db.Put(name, sfc)
	if err != nil {
		plugin.Log.Error("DatastoreSfcEntityPut: error storing key: '%s'", name)
		plugin.Log.Error("DatastoreSfcEntityPut: databroker put: ", err)
		return err
	}

	return nil
}

// pull the specified entities from the sfc db in etcd into the sfc ram cache
func (plugin *SfcControllerPluginHandler) DatastoreSfcEntityRetrieveAllIntoRamCache() error {

	return plugin.DatastoreSfcEntityIterate(func(key string, sfc *controller.SfcEntity) {
		plugin.ramConfigCache.SFCs[key] = *sfc
		plugin.Log.Infof("DatastoreSfcEntityRetrieveAllIntoRamCache: adding sfc: '%s': ", key, *sfc)
	})
}

// remove the specified entities from the sfc db in etcd
func (plugin *SfcControllerPluginHandler) DatastoreSfcEntityDeleteAll() error {

	//TODO DELETE ALL could delete all key by prefixes db.Delete(prefix, WithPrefix())

	plugin.Log.Info("DatastoreSfcEntityDeleteAll: begin ...")
	defer plugin.Log.Info("DatastoreSfcEntityDeleteAll: exit ...")

	return plugin.DatastoreSfcEntityIterate(func(name string, sfc *controller.SfcEntity) {
		key := controller.SfcEntityNameKey(name)
		plugin.Log.Infof("DatastoreSfcEntityDeleteAll: deleting sfc: '%s': ", key, *sfc)
		plugin.db.Delete(key) //TODO move to DatastoreSfcEntityDelete
	})
}

// iterate over the set of specified entities in the sfc tree in etcd
func (plugin *SfcControllerPluginHandler) DatastoreSfcEntityIterate(actionFunc func(key string,
	sfc *controller.SfcEntity)) error {

	kvi, err := plugin.db.ListValues(controller.SfcEntityKeyPrefix())
	if err != nil {
		plugin.Log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		sfc := &controller.SfcEntity{}
		err := kv.GetValue(sfc)
		if err != nil {
			plugin.Log.Fatal(err)
			return nil
		}

		plugin.Log.Infof("DatastoreSfcEntityIterate: getting sfc: '%s': ", sfc.Name, sfc)
		actionFunc(sfc.Name, sfc)

	}
	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (plugin *SfcControllerPluginHandler) DatastoreSfcEntityDelete(ee *controller.HostEntity) error {
	return nil
}

// GetExternalEntity gets from RAM cache
func (plugin *SfcControllerPluginHandler) GetExternalEntity(externalEntityName string) (entity *controller.ExternalEntity, found bool) {
	//TODO - do this thread safe
	ee, found := plugin.ramConfigCache.EEs[externalEntityName]
	return &ee, found
}

// PutExternalEntity updates RAM cache & ETCD
func (plugin *SfcControllerPluginHandler) PutExternalEntity(ee *controller.ExternalEntity) {
	//TODO - do this thread safe
	plugin.ramConfigCache.EEs[ee.Name] = *ee
	plugin.DatastoreExternalEntityPut(ee)
	//TODO fire event go channel (to process this using watcher pattern)
}

// ListExternalEntities lists RAM cache
func (plugin *SfcControllerPluginHandler) ListExternalEntities() []*controller.ExternalEntity {
	//TODO - do this thread safe
	ret := []*controller.ExternalEntity{}
	for _, ee := range plugin.ramConfigCache.EEs {
		ret = append(ret, &ee)
	}
	return ret
}

// GetHostEntity gets from RAM cache
func (plugin *SfcControllerPluginHandler) GetHostEntity(hostEntityName string) (entity *controller.HostEntity, found bool) {
	//TODO - do this thread safe
	he, found := plugin.ramConfigCache.HEs[hostEntityName]
	return &he, found
}

// PutHostEntity updates RAM cache & ETCD
func (plugin *SfcControllerPluginHandler) PutHostEntity(he *controller.HostEntity) {
	//TODO - do this thread safe
	plugin.ramConfigCache.HEs[he.Name] = *he
	//TODO fire event go channel (to process this using watcher pattern)
	plugin.DatastoreHostEntityPut(he)
}

// ListHostEntities lists RAM cache
func (plugin *SfcControllerPluginHandler) ListHostEntities() []*controller.HostEntity {
	//TODO - do this thread safe
	ret := []*controller.HostEntity{}
	for _, he := range plugin.ramConfigCache.HEs {
		ret = append(ret, &he)
	}
	return ret
}

// GetSFCEntity gets from RAM cache
func (plugin *SfcControllerPluginHandler) GetSFCEntity(sfcName string) (entity *controller.SfcEntity, found bool) {
	//TODO - do this thread safe
	sfc, found := plugin.ramConfigCache.SFCs[sfcName]
	return &sfc, found
}

// PutHostEntity updates RAM cache & ETCD
func (plugin *SfcControllerPluginHandler) PutSFCEntity(sfc *controller.SfcEntity) {
	//TODO - do this thread safe
	plugin.ramConfigCache.SFCs[sfc.Name] = *sfc
	//TODO fire event go channel (to process this using watcher pattern)
	plugin.DatastoreSfcEntityPut(sfc)
}

// ListHostEntities lists RAM cache
func (plugin *SfcControllerPluginHandler) ListSFCEntities() []*controller.SfcEntity {
	//TODO - do this thread safe
	ret := []*controller.SfcEntity{}
	for _, sfc := range plugin.ramConfigCache.SFCs {
		ret = append(ret, &sfc)
	}
	return ret
}

// WatchHostEntity allows other plugins to receive Host Entity northbound configuration changes
func (plugin *SfcControllerPluginHandler) WatchHostEntity(subscriberName string,
	callback func(*controller.HostEntity) error) error {

	plugin.ramConfigCache.watcherHEs[subscriberName] = callback

	return nil
}

// WatchSFCEntity allows other plugins to receive SFC Entity northbound configuration changes
func (plugin *SfcControllerPluginHandler) WatchSFCEntity(subscriberName string,
	callback func(*controller.SfcEntity) error) error {

	plugin.ramConfigCache.watcherSFCs[subscriberName] = callback

	return nil
}

// WatchExternalEntity allows other plugins to receive External Entity northbound configuration changes
func (plugin *SfcControllerPluginHandler) WatchExternalEntity(subscriberName string,
	callback func(*controller.ExternalEntity) error) error {

	plugin.ramConfigCache.watcherEEs[subscriberName] = callback

	return nil
}
