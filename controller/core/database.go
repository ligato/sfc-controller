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
func (sfcCtrlPlugin *SfcControllerPluginHandler) WriteRamCacheToEtcd() error {

	for _, ee := range sfcCtrlPlugin.ramConfigCache.EEs {
		if err := sfcCtrlPlugin.DatastoreExternalEntityPut(&ee); err != nil {
			return err
		}
	}
	for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
		if err := sfcCtrlPlugin.DatastoreHostEntityPut(&he); err != nil {
			return err
		}
	}
	for _, sfc := range sfcCtrlPlugin.ramConfigCache.SFCs {
		if err := sfcCtrlPlugin.DatastoreSfcEntityPut(&sfc); err != nil {
			return err
		}
	}

	return nil
}

// pull the sfc db from the etcd tree into the ram cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) ReadEtcdDatastoreIntoRamCache() error {

	log.Infof("ReadEtcdDatastoreIntoRamCache: start ...")

	if err := sfcCtrlPlugin.DatastoreExternalEntityRetrieveAllIntoRamCache(); err != nil {
		return err
	}
	if err := sfcCtrlPlugin.DatastoreHostEntityRetrieveAllIntoRamCache(); err != nil {
		return err
	}
	if err := sfcCtrlPlugin.DatastoreSfcEntityRetrieveAllIntoRamCache(); err != nil {
		return err
	}

	log.Infof("ReadEtcdDatastoreIntoRamCache: end ...")

	return nil
}

// clear the sfc tree in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreClean() error {

	log.Infof("DatastoreClean: clearing etc tree")

	if err := sfcCtrlPlugin.DatastoreExternalEntityDeleteAll(); err != nil {
		log.Error("DatastoreClean: DatastoreExternalEntityDeleteAll: ", err)
	}
	if err := sfcCtrlPlugin.DatastoreHostEntityDeleteAll(); err != nil {
		log.Error("DatastoreClean: DatastoreHostEntityDeleteAll: ", err)
	}
	if err := sfcCtrlPlugin.DatastoreSfcEntityDeleteAll(); err != nil {
		log.Error("DatastoreClean: DatastoreSfcEntityDeleteAll: ", err)
	}

	return nil
}

// create the specified entity in the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityPut(ee *controller.ExternalEntity) error {

	name := controller.ExternalEntityNameKey(ee.Name)

	log.Infof("DatastoreExternalEntityPut: setting key: '%s'", name)

	err := sfcCtrlPlugin.db.Put(name, ee)
	if err != nil {
		log.Error("DatastoreExternalEntityPut: error storing key: '%s'", name)
		log.Error("DatastoreExternalEntityPut: databroker put: ", err)
		return err
	}
	return nil
}

// pull the specified entities from the sfc db in etcd into the sfc ram cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityRetrieveAllIntoRamCache() error {

	return sfcCtrlPlugin.DatastoreExternalEntityIterate(func(key string, ee *controller.ExternalEntity) {
		sfcCtrlPlugin.ramConfigCache.EEs[key] = *ee
		log.Infof("DatastoreExternalEntityRetrieveAllIntoRamCache: adding ee: '%s': ", key, *ee)
	})
}

// remove the specified entities from the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityDeleteAll() error {

	log.Info("DatastoreExternalEntityDeleteAll: begin ...")
	defer log.Info("DatastoreExternalEntityDeleteAll: exit ...")

	return sfcCtrlPlugin.DatastoreExternalEntityIterate(func(name string, ee *controller.ExternalEntity) {
		key := controller.ExternalEntityNameKey(name)
		log.Infof("DatastoreExternalEntityDeleteAll: deleting ee: '%s': ", key, *ee)
		sfcCtrlPlugin.db.Delete(key)
	})
}

// iterate over the set of specified entities in the sfc tree in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityIterate(actionFunc func(key string,
	val *controller.ExternalEntity)) error {

	kvi, err := sfcCtrlPlugin.db.ListValues(controller.ExternalEntityKeyPrefix())
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
			return nil
		}
		log.Infof("DatastoreExternalEntityIterate: iterating ee: '%s': ", ee.Name, ee)
		actionFunc(ee.Name, ee)

	}
	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityDelete(ee *controller.ExternalEntity) error {

	return nil
}

// create the specified entity in the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityPut(he *controller.HostEntity) error {

	name := controller.HostEntityNameKey(he.Name)

	log.Infof("DatastoreHostEntityPut: setting key: '%s'", name)

	err := sfcCtrlPlugin.db.Put(name, he)
	if err != nil {
		log.Error("DatastoreHostEntityPut: error storing key: '%s'", name)
		log.Error("DatastoreHostEntityPut: databroker put: ", err)
		return err
	}
	return nil
}

// pull the specified entities from the sfc db in etcd into the sfc ram cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityRetrieveAllIntoRamCache() error {

	return sfcCtrlPlugin.DatastoreHostEntityIterate(func(key string, he *controller.HostEntity) {
		sfcCtrlPlugin.ramConfigCache.HEs[key] = *he
		log.Infof("DatastoreHostEntityRetrieveAllIntoRamCache: adding he: '%s': ", key, *he)
	})
}

// remove the specified entities from the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityDeleteAll() error {

	log.Info("DatastoreHostEntityDeleteAll: begin ...")
	defer log.Info("DatastoreHostEntityDeleteAll: exit ...")

	return sfcCtrlPlugin.DatastoreHostEntityIterate(func(name string, he *controller.HostEntity) {
		key := controller.HostEntityNameKey(name)
		log.Infof("DatastoreHostsEntityDeleteAll: deleting he: '%s': ", key, *he)
		sfcCtrlPlugin.db.Delete(key)
	})
}

// iterate over the set of specified entities in the sfc tree in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityIterate(actionFunc func(key string,
	he *controller.HostEntity)) error {

	kvi, err := sfcCtrlPlugin.db.ListValues(controller.HostEntityKeyPrefix())
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
			return nil
		}

		log.Infof("DatastoreHostEntityIterate: getting he: '%s': ", he.Name, he)
		actionFunc(he.Name, he)

	}
	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityDelete(ee *controller.HostEntity) error {

	return nil
}

// create the specified entity in the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityPut(sfc *controller.SfcEntity) error {

	name := controller.SfcEntityNameKey(sfc.Name)

	log.Infof("DatastoreSfcEntityPut: setting key: '%s'", name)

	err := sfcCtrlPlugin.db.Put(name, sfc)
	if err != nil {
		log.Error("DatastoreSfcEntityPut: error storing key: '%s'", name)
		log.Error("DatastoreSfcEntityPut: databroker put: ", err)
		return err
	}

	return nil
}

// pull the specified entities from the sfc db in etcd into the sfc ram cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityRetrieveAllIntoRamCache() error {

	return sfcCtrlPlugin.DatastoreSfcEntityIterate(func(key string, sfc *controller.SfcEntity) {
		sfcCtrlPlugin.ramConfigCache.SFCs[key] = *sfc
		log.Infof("DatastoreSfcEntityRetrieveAllIntoRamCache: adding sfc: '%s': ", key, *sfc)
	})
}

// remove the specified entities from the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityDeleteAll() error {

	//TODO DELETE ALL could delete all key by prefixes db.Delete(prefix, WithPrefix())

	log.Info("DatastoreSfcEntityDeleteAll: begin ...")
	defer log.Info("DatastoreSfcEntityDeleteAll: exit ...")

	return sfcCtrlPlugin.DatastoreSfcEntityIterate(func(name string, sfc *controller.SfcEntity) {
		key := controller.SfcEntityNameKey(name)
		log.Infof("DatastoreSfcEntityDeleteAll: deleting sfc: '%s': ", key, *sfc)
		sfcCtrlPlugin.db.Delete(key) //TODO move to DatastoreSfcEntityDelete
	})
}

// iterate over the set of specified entities in the sfc tree in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityIterate(actionFunc func(key string,
	sfc *controller.SfcEntity)) error {

	kvi, err := sfcCtrlPlugin.db.ListValues(controller.SfcEntityKeyPrefix())
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
			return nil
		}

		log.Infof("DatastoreSfcEntityIterate: getting sfc: '%s': ", sfc.Name, sfc)
		actionFunc(sfc.Name, sfc)

	}
	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityDelete(ee *controller.HostEntity) error {
	return nil
}

// GetExternalEntity gets from RAM cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) GetExternalEntity(externalEntityName string) (entity *controller.ExternalEntity, found bool) {
	//TODO - do this thread safe
	ee, found := sfcCtrlPlugin.ramConfigCache.EEs[externalEntityName]
	return &ee, found
}

// PutExternalEntity updates RAM cache & ETCD
func (sfcCtrlPlugin *SfcControllerPluginHandler) PutExternalEntity(ee *controller.ExternalEntity) {
	//TODO - do this thread safe
	sfcCtrlPlugin.ramConfigCache.EEs[ee.Name] = *ee
	sfcCtrlPlugin.DatastoreExternalEntityPut(ee)
	//TODO fire event go channel (to process this using watcher pattern)
}

// ListExternalEntities lists RAM cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) ListExternalEntities() []*controller.ExternalEntity {
	//TODO - do this thread safe
	ret := []*controller.ExternalEntity{}
	for _, ee := range sfcCtrlPlugin.ramConfigCache.EEs {
		ret = append(ret, &ee)
	}
	return ret
}

// GetHostEntity gets from RAM cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) GetHostEntity(hostEntityName string) (entity *controller.HostEntity, found bool) {
	//TODO - do this thread safe
	he, found := sfcCtrlPlugin.ramConfigCache.HEs[hostEntityName]
	return &he, found
}

// PutHostEntity updates RAM cache & ETCD
func (sfcCtrlPlugin *SfcControllerPluginHandler) PutHostEntity(he *controller.HostEntity) {
	//TODO - do this thread safe
	sfcCtrlPlugin.ramConfigCache.HEs[he.Name] = *he
	//TODO fire event go channel (to process this using watcher pattern)
	sfcCtrlPlugin.DatastoreHostEntityPut(he)
}

// ListHostEntities lists RAM cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) ListHostEntities() []*controller.HostEntity {
	//TODO - do this thread safe
	ret := []*controller.HostEntity{}
	for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
		ret = append(ret, he)
	}
	return ret
}

// GetSFCEntity gets from RAM cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) GetSFCEntity(sfcName string) (entity *controller.SfcEntity, found bool) {
	//TODO - do this thread safe
	sfc, found := sfcCtrlPlugin.ramConfigCache.SFCs[sfcName]
	return &sfc, found
}

// PutHostEntity updates RAM cache & ETCD
func (sfcCtrlPlugin *SfcControllerPluginHandler) PutSFCEntity(sfc *controller.SfcEntity) {
	//TODO - do this thread safe
	sfcCtrlPlugin.ramConfigCache.SFCs[sfc.Name] = *sfc
	//TODO fire event go channel (to process this using watcher pattern)
	sfcCtrlPlugin.DatastoreSfcEntityPut(sfc)
}

// ListHostEntities lists RAM cache
func (sfcCtrlPlugin *SfcControllerPluginHandler) ListSFCEntities() []*controller.SfcEntity {
	//TODO - do this thread safe
	ret := []*controller.SfcEntity{}
	for _, sfc := range sfcCtrlPlugin.ramConfigCache.SFCs {
		ret = append(ret, &sfc)
	}
	return ret
}
