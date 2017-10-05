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
		if err := sfcCtrlPlugin.DatastoreExternalEntityCreate(&ee); err != nil {
			return err
		}
	}
	for _, he := range sfcCtrlPlugin.ramConfigCache.HEs {
		if err := sfcCtrlPlugin.DatastoreHostEntityCreate(&he); err != nil {
			return err
		}
	}
	for _, sfc := range sfcCtrlPlugin.ramConfigCache.SFCs {
		if err := sfcCtrlPlugin.DatastoreSfcEntityCreate(&sfc); err != nil {
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
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreReInitialize() error {

	log.Infof("DatastoreReInitialize: clearing etc tree")

	if err := sfcCtrlPlugin.DatastoreExternalEntityDeleteAll(); err != nil {
		log.Error("DatastoreReInitialize: DatastoreExternalEntityDeleteAll: ", err)
	}
	if err := sfcCtrlPlugin.DatastoreHostEntityDeleteAll(); err != nil {
		log.Error("DatastoreReInitialize: DatastoreHostEntityDeleteAll: ", err)
	}
	if err := sfcCtrlPlugin.DatastoreSfcEntityDeleteAll(); err != nil {
		log.Error("DatastoreReInitialize: DatastoreSfcEntityDeleteAll: ", err)
	}

	return nil
}

// create the specified entity in the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityCreate(ee *controller.ExternalEntity) error {

	name := controller.ExternalEntityNameKey(ee.Name)

	log.Infof("DatastoreExternalEntityCreate: setting key: '%s'", name)

	err := sfcCtrlPlugin.db.Put(name, ee)
	if err != nil {
		log.Error("DatastoreExternalEntityCreate: error storing key: '%s'", name)
		log.Error("DatastoreExternalEntityCreate: databroker put: ", err)
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

// Update the specified entity in the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityUpdate(ee *controller.ExternalEntity) error {

	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreExternalEntityDelete(ee *controller.ExternalEntity) error {

	return nil
}

// create the specified entity in the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityCreate(he *controller.HostEntity) error {

	name := controller.HostEntityNameKey(he.Name)

	log.Infof("DatastoreHostEntityCreate: setting key: '%s'", name)

	err := sfcCtrlPlugin.db.Put(name, he)
	if err != nil {
		log.Error("DatastoreHostEntityCreate: error storing key: '%s'", name)
		log.Error("DatastoreHostEntityCreate: databroker put: ", err)
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

// Update the specified entity in the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityUpdate(ee *controller.HostEntity) error {

	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreHostEntityDelete(ee *controller.HostEntity) error {

	return nil
}

// create the specified entity in the sfc db in etcd
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityCreate(sfc *controller.SfcEntity) error {

	name := controller.SfcEntityNameKey(sfc.Name)

	log.Infof("DatastoreSfcEntityCreate: setting key: '%s'", name)

	err := sfcCtrlPlugin.db.Put(name, sfc)
	if err != nil {
		log.Error("DatastoreSfcEntityCreate: error storing key: '%s'", name)
		log.Error("DatastoreSfcEntityCreate: databroker put: ", err)
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

	log.Info("DatastoreSfcEntityDeleteAll: begin ...")
	defer log.Info("DatastoreSfcEntityDeleteAll: exit ...")

	return sfcCtrlPlugin.DatastoreSfcEntityIterate(func(name string, sfc *controller.SfcEntity) {
		key := controller.SfcEntityNameKey(name)
		log.Infof("DatastoreSfcEntityDeleteAll: deleting sfc: '%s': ", key, *sfc)
		sfcCtrlPlugin.db.Delete(key)
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

// Update the specified entity in the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityUpdate(ee *controller.HostEntity) error {

	return nil
}

// Delete the specified entity from the sfc db in the etcd tree
func (sfcCtrlPlugin *SfcControllerPluginHandler) DatastoreSfcEntityDelete(ee *controller.HostEntity) error {

	return nil
}
