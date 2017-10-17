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
	"github.com/ligato/cn-infra/datasync"
)

// flush the ram cache to the sfc cache to the tree in etcd
func (plugin *SfcControllerPluginHandler) WriteRamCacheToEtcd() error {

	for _, ee := range plugin.ramConfigCache.EEs.ListValues() {
		if err := plugin.DatastoreExternalEntityPut(ee); err != nil {
			return err
		}
	}
	for _, he := range plugin.ramConfigCache.HEs.ListValues() {
		if err := plugin.DatastoreHostEntityPut(he); err != nil {
			return err
		}
	}
	for _, sfc := range plugin.ramConfigCache.SFCs.ListValues() {
		if err := plugin.DatastoreSfcEntityPut(sfc); err != nil {
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

// DatastoreClean clears the sfc tree in etcd (EEs, HEs, SFCs)
func (plugin *SfcControllerPluginHandler) DatastoreClean() error {
	plugin.Log.Infof("DatastoreClean: clearing etc tree - begin")

	if existed, err := plugin.db.Delete(controller.SfcControllerPrefix(), datasync.WithPrefix()); err != nil {
		return err
	} else {
		plugin.Log.Info("DatastoreClean: clearing etc tree SUCCESSFUL; existed=", existed)
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
		plugin.ramConfigCache.EEs.Put(ee)
		plugin.Log.Infof("DatastoreExternalEntityRetrieveAllIntoRamCache: adding ee: '%s': ", key, *ee)
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

// DatastoreExternalEntityDelete - Delete the specified entity from the sfc db in the etcd tree
func (plugin *SfcControllerPluginHandler) DatastoreExternalEntityDelete(ee *controller.ExternalEntity) error {
	key := controller.ExternalEntityNameKey(ee.Name)
	plugin.Log.Infof("DatastoreSfcEntityDelete: deleteing key: '%s'", key)
	_, err := plugin.db.Delete(key)
	return err
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
		plugin.ramConfigCache.HEs.Put(he)
		plugin.Log.Infof("DatastoreHostEntityRetrieveAllIntoRamCache: adding he: '%s': ", key, *he)
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

// DatastoreHostEntityDelete - Delete the specified entity from the sfc db in the etcd tree
func (plugin *SfcControllerPluginHandler) DatastoreHostEntityDelete(he *controller.HostEntity) error {
	key := controller.ExternalEntityNameKey(he.Name)
	plugin.Log.Infof("DatastoreSfcEntityDelete: deleteing key: '%s'", key)
	_, err := plugin.db.Delete(key)
	return err
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
		plugin.ramConfigCache.SFCs.Put(sfc)
		plugin.Log.Infof("DatastoreSfcEntityRetrieveAllIntoRamCache: adding sfc: '%s': ", key, *sfc)
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

// DatastoreSfcEntityDelete - Delete the specified entity from the sfc db in the etcd tree
func (plugin *SfcControllerPluginHandler) DatastoreSfcEntityDelete(sfc *controller.SfcEntity) error {
	key := controller.SfcEntityNameKey(sfc.Name)
	plugin.Log.Infof("DatastoreSfcEntityDelete: deleteing key: '%s'", key)
	_, err := plugin.db.Delete(key)
	return err
}

// GetExternalEntity gets from RAM cache
func (plugin *SfcControllerPluginHandler) GetExternalEntity(externalEntityName string) (entity *controller.ExternalEntity, found bool) {
	ee, found := plugin.ramConfigCache.EEs.GetValue(externalEntityName)
	return ee, found
}

// PutExternalEntity updates RAM cache & ETCD
func (plugin *SfcControllerPluginHandler) PutExternalEntity(ee *controller.ExternalEntity) error {
	if err := plugin.DatastoreExternalEntityPut(ee); err != nil {
		return err
	}

	plugin.ramConfigCache.EEs.Put(ee)

	//TODO do this outside rest package (watcher)
	if err := plugin.renderExternalEntity(ee, true, true); err != nil {
		return err
	}

	return nil
}

// ListExternalEntities lists RAM cache
func (plugin *SfcControllerPluginHandler) ListExternalEntities() []*controller.ExternalEntity {
	ret := []*controller.ExternalEntity{}
	for _, ee := range plugin.ramConfigCache.EEs.ListValues() {
		ret = append(ret, ee)
	}
	return ret
}

// GetHostEntity gets from RAM cache
func (plugin *SfcControllerPluginHandler) GetHostEntity(hostEntityName string) (entity *controller.HostEntity, found bool) {
	he, found := plugin.ramConfigCache.HEs.GetValue(hostEntityName)
	return he, found
}

// PutHostEntity updates RAM cache & ETCD
func (plugin *SfcControllerPluginHandler) PutHostEntity(he *controller.HostEntity) error {
	//TODO fire event go channel (to process this using watcher pattern)
	if err := plugin.DatastoreHostEntityPut(he); err != nil {
		return err
	}

	plugin.ramConfigCache.HEs.Put(he)

	//TODO do this outside rest package (watcher)
	if err := plugin.renderHostEntity(he, true, true); err != nil {
		return err
	}

	return nil
}

// ListHostEntities lists RAM cache
func (plugin *SfcControllerPluginHandler) ListHostEntities() []*controller.HostEntity {
	ret := []*controller.HostEntity{}
	for _, he := range plugin.ramConfigCache.HEs.ListValues() {
		ret = append(ret, he)
	}
	return ret
}

// GetSFCEntity gets from RAM cache
func (plugin *SfcControllerPluginHandler) GetSFCEntity(sfcName string) (entity *controller.SfcEntity, found bool) {
	sfc, found := plugin.ramConfigCache.SFCs.GetValue(sfcName)
	return sfc, found
}

// PutSFCEntity updates RAM cache & ETCD
func (plugin *SfcControllerPluginHandler) PutSFCEntity(sfc *controller.SfcEntity) error {
	//TODO fire event go channel (to process this using watcher pattern)
	if err := plugin.DatastoreSfcEntityPut(sfc); err != nil {
		return err
	}

	plugin.ramConfigCache.SFCs.Put(sfc)

	//TODO do this outside this package (watcher)
	if err := plugin.renderServiceFunctionEntity(sfc); err != nil {
		return err
	}

	return nil
}

// ListSFCEntities lists RAM cache
func (plugin *SfcControllerPluginHandler) ListSFCEntities() []*controller.SfcEntity {
	ret := []*controller.SfcEntity{}
	for _, sfc := range plugin.ramConfigCache.SFCs.ListValues() {
		ret = append(ret, sfc)
	}
	return ret
}
