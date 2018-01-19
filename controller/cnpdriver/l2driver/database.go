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

// The database for the SFC l2 is stroed in ETCD.  The l2 uses
// the ligate/cn-infra which connects to ETCD.  The config for the l2
// is derived from the config yaml file which is passed vi command line args or
// it can be configured via HTTP REST calls.
// This module stores configuration for hosts, external routers, and the SFC's.
// The SFC's are chains or sets of containers that should be wired together.

package l2driver

// routines to "write" the sfc l2 IDs/id's to the datastore.  The datastore is makes use of the ETCD infra
// to CRUD id's for type host, host to external, host to host, and sfc.  The ETCD keys are defined in keys_l2.go

import (
	"github.com/ligato/sfc-controller/controller/cnpdriver/l2driver/model"
	"fmt"
)

// DatastoreReInitialize clears the sfc tree in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreReInitialize() error {

	log.Infof("DatastoreReInitialize: clearing etc tree")

	if err := cnpd.DatastoreHEIDsDeleteAll(); err != nil {
		log.Error("DatastoreReInitialize: DatastoreHEIDsDeleteAll: ", err)
		return err
	}
	if err := cnpd.DatastoreHE2EEIDsDeleteAll(); err != nil {
		log.Error("DatastoreReInitialize: DatastoreHE2EEIDsDeleteAll: ", err)
		return err
	}
	//if err := cnpd.DatastoreHE2HEIDsDeleteAll(); err != nil {
	//	log.Error("DatastoreReInitialize: DatastoreHE2HEIDsDeleteAll: ", err)
	//}
	if err := cnpd.DatastoreSFCIDsDeleteAll(); err != nil {
		log.Error("DatastoreReInitialize: DatastoreSFCIDsDeleteAll: ", err)
		return err
	}

	return nil
}

// DatastoreHEIDsCreate creates the specified entity in the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHEIDsCreate(heName string,
	macAddrID uint32) (string, *l2.HEIDs, error) {

	he := &l2.HEIDs{
		Name: heName,
		LoopbackMacAddrId: macAddrID,
	}

	key := l2.HEIDsNameKey(he.Name)

	log.Infof("DatastoreHEIDsCreate: setting key: '%s'", key, he)

	err := cnpd.db.Put(key, he)
	if err != nil {
		log.Error("DatastoreHEIDsCreate: error storing key: '%s'", key)
		log.Error("DatastoreHEIDsCreate: databroker put: ", err)
		return "", nil, err
	}

	return key, he, nil
}

// DatastoreHEIDsRetrieve gets the specified entity from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHEIDsRetrieve(heName string) (*l2.HEIDs, error) {

	name := l2.HEIDsNameKey(heName)
	he := &l2.HEIDs{}
	found, _, err := cnpd.db.GetValue(name, he)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	if !found {
		err = fmt.Errorf("DatastoreHEIDsRetrieve: not found: %s", name)
		log.Info(err)
		return nil, err
	}

	return he, err
}

// DatastoreHEIDsDeleteAll removes the specified entities from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHEIDsDeleteAll() error {

	log.Info("DatastoreHEIDsDeleteAll: begin ...")
	defer log.Info("DatastoreHEIDsDeleteAll: exit ...")

	return cnpd.DatastoreHEIDsIterate(func(key string, ee *l2.HEIDs) {
		log.Infof("DatastoreHEIDsDeleteAll: deleting ee: '%s': ", key, *ee)
		cnpd.db.Delete(key)
	})
}

// DatastoreHEIDsIterate iterates over the set of specified entities in the sfc tree in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHEIDsIterate(actionFunc func(key string,
	val *l2.HEIDs)) error {

	kvi, err := cnpd.db.ListValues(l2.HEIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		he := &l2.HEIDs{}
		err := kv.GetValue(he)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		log.Infof("DatastoreHEIDsIterate: iterating HE ID: '%s': ", kv.GetKey(), he)
		actionFunc(kv.GetKey(), he)

	}
}

// DatastoreHEIDsUpdate updates the specified entity in the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHEIDsUpdate(ee *l2.HEIDs) error {

	return nil
}

// DatastoreHEIDsDelete deletes the specified entity from the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHEIDsDelete(ee *l2.HEIDs) error {

	return nil
}

// DatastoreHE2EEIDsCreate creates the specified entity in the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2EEIDsCreate(heName string, eeName string,
	vlanID uint32) (string, *l2.HE2EEIDs, error) {

	he2ee := &l2.HE2EEIDs{
		HeName: heName,
		EeName: eeName,
		VlanId: vlanID,
	}

	key := l2.HE2EEIDsNameKey(heName, eeName)

	log.Infof("DatastoreHE2EEIDsCreate: setting key: '%s'", key)

	err := cnpd.db.Put(key, he2ee)
	if err != nil {
		log.Error("DatastoreHE2EEIDsCreate: error storing key: '%s'", key)
		log.Error("DatastoreHE2EEIDsCreate: databroker put: ", err)
		return "", nil, err
	}

	return key, he2ee, nil
}

// DatastoreHE2EEIDsRetrieve gets the specified entity from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2EEIDsRetrieve(heName string, eeName string) (*l2.HE2EEIDs, error) {

	key := l2.HE2EEIDsNameKey(heName, eeName)
	he2ee := &l2.HE2EEIDs{}
	found, _, err := cnpd.db.GetValue(key, he2ee)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	if !found {
		err = fmt.Errorf("DatastoreHE2EEIDsRetrieve: not found: %s", key)
		log.Info(err)
		return nil, err
	}
	return he2ee, err
}

// DatastoreHE2EEIDsDeleteAll removes the specified entities from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2EEIDsDeleteAll() error {

	log.Info("DatastoreHE2EEIDsDeleteAll: begin ...")
	defer log.Info("DatastoreHE2EEIDsDeleteAll: exit ...")

	return cnpd.DatastoreHE2EEIDsIterate(func(key string, he2ee *l2.HE2EEIDs) {
		log.Infof("DatastoreHE2EEIDsDeleteAll: deleting he2ee: '%s': ", key, *he2ee)
		cnpd.db.Delete(key)
	})
}

// DatastoreHE2EEIDsIterate iterates over the set of specified entities in the sfc tree in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2EEIDsIterate(actionFunc func(key string,
	he *l2.HE2EEIDs)) error {

	kvi, err := cnpd.db.ListValues(l2.HE2EEIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		he2ee := &l2.HE2EEIDs{}
		err := kv.GetValue(he2ee)
		if err != nil {
			log.Fatal(err)
			return nil
		}

		log.Infof("DatastoreHE2EEIDsIterate: getting he2ee: '%s': ", kv.GetKey())
		actionFunc(kv.GetKey(), he2ee)

	}
}

// DatastoreHE2EEIDsUpdate updates the specified entity in the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2EEIDsUpdate(ee *l2.HE2EEIDs) error {

	return nil
}

// DatastoreHE2EEIDsDelete deletes the specified entity from the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2EEIDsDelete(ee *l2.HE2EEIDs) error {

	return nil
}

// DatastoreHE2HEIDsCreate creates the specified entity in the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2HEIDsCreate(shName string, dhName string,
	vlanID uint32) (string, *l2.HE2HEIDs, error) {

	sh2dh := &l2.HE2HEIDs{
		ShName: shName,
		DhName: dhName,
		VlanId: vlanID,
	}

	key := l2.HE2HEIDsNameKey(shName, dhName)

	log.Infof("DatastoreHE2HEIDsCreate: setting key: '%s'", key)

	err := cnpd.db.Put(key, sh2dh)
	if err != nil {
		log.Errorf("DatastoreHE2HEIDsCreate: error storing key: '%s'", key)
		log.Error("DatastoreHE2HEIDsCreate: databroker put: ", err)
		return "", nil, err
	}

	return key, sh2dh, nil
}

// DatastoreHE2HEIDsRetrieve gets the specified entity from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2HEIDsRetrieve(shName string, dhName string) (*l2.HE2HEIDs, error) {

	key := l2.HE2HEIDsNameKey(shName, dhName)
	sh2dh := &l2.HE2HEIDs{}
	found, _, err := cnpd.db.GetValue(key, sh2dh)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	if !found {
		err = fmt.Errorf("DatastoreHE2HEIDsRetrieve: not found: %s", key)
		log.Info(err)
		return nil, err
	}
	return sh2dh, err
}

// DatastoreHE2HEIDsDeleteAll removes the specified entities from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2HEIDsDeleteAll() error {

	log.Info("DatastoreHE2HEIDsDeleteAll: begin ...")
	defer log.Info("DatastoreHE2HEIDsDeleteAll: exit ...")

	return cnpd.DatastoreHE2HEIDsIterate(func(key string, sh2dh *l2.HE2HEIDs) {
		log.Infof("DatastoreHE2HEIDsDeleteAll: deleting he2ee: '%s': ", key, *sh2dh)
		cnpd.db.Delete(key)
	})
}

// DatastoreHE2HEIDsIterate iterates over the set of specified entities in the sfc tree in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2HEIDsIterate(actionFunc func(key string,
	he *l2.HE2HEIDs)) error {

	kvi, err := cnpd.db.ListValues(l2.HE2HEIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		sh2dh := &l2.HE2HEIDs{}
		err := kv.GetValue(sh2dh)
		if err != nil {
			log.Fatal(err)
			return nil
		}

		log.Infof("DatastoreHE2HEIDsIterate: getting sh2dh: '%s': ", kv.GetKey())
		actionFunc(kv.GetKey(), sh2dh)

	}
}

// DatastoreHE2HEIDsUpdate updates the specified entity in the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2HEIDsUpdate(ee *l2.HE2HEIDs) error {

	return nil
}

// DatastoreHE2HEIDsDelete deletes the specified entity from the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreHE2HEIDsDelete(ee *l2.HE2HEIDs) error {

	return nil
}

// DatastoreSFCIDsCreate creates the specified entity in the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreSFCIDsCreate(sfcName string, container string,
	port string, ipID uint32, macAddrID uint32, memifID uint32, vethID uint32) (string, *l2.SFCIDs, error) {

	sfc := &l2.SFCIDs{
		SfcName: sfcName,
		Container: container,
		Port: port,
		IpId: ipID,
		MacAddrId: macAddrID,
		MemifId: memifID,
		VethId: vethID,
	}

	key := l2.SFCContainerPortIDsNameKey(sfcName, container, port)

	log.Infof("DatastoreSFCIDsCreate: setting key: '%s'", key)

	err := cnpd.db.Put(key, sfc)
	if err != nil {
		log.Error("DatastoreSFCIDsCreate: error storing key: '%s'", key)
		log.Error("DatastoreSFCIDsCreate: databroker put: ", err)
		return "", nil, err
	}
	return key, sfc, nil
}

// DatastoreSFCIDsRetrieve gets the specified entity from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreSFCIDsRetrieve(sfcName string, container string,
	port string) (*l2.SFCIDs, error) {

	key := l2.SFCContainerPortIDsNameKey(sfcName, container, port)
	sfc := &l2.SFCIDs{}
	found, _, err := cnpd.db.GetValue(key, sfc)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	if !found {
		err = fmt.Errorf("DatastoreSFCIDsRetrieve: not found: %s", key)
		log.Info(err)
		return nil, err
	}
	return sfc, err
}

// DatastoreSFCIDsDeleteAll removes the specified entities from the sfc db in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreSFCIDsDeleteAll() error {

	log.Info("DatastoreSFCIDsDeleteAll: begin ...")
	defer log.Info("DatastoreSFCIDsDeleteAll: exit ...")

	return cnpd.DatastoreSFCIDsIterate(func(key string, sfc *l2.SFCIDs) {
		log.Infof("DatastoreSFCIDsDeleteAll: deleting sfc: '%s': ", key, *sfc)
		cnpd.db.Delete(key)
	})
}

// DatastoreSFCIDsIterate iterates over the set of specified entities in the sfc tree in etcd
func (cnpd *sfcCtlrL2CNPDriver) DatastoreSFCIDsIterate(actionFunc func(key string,
	sfc *l2.SFCIDs)) error {

	kvi, err := cnpd.db.ListValues(l2.SFCIDsKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		sfc := &l2.SFCIDs{}
		err := kv.GetValue(sfc)
		if err != nil {
			log.Fatal(err)
			return nil
		}

		log.Infof("DatastoreSFCIDsIterate: getting sfc: '%s': ", kv.GetKey(), sfc)
		actionFunc(kv.GetKey(), sfc)

	}
}

// DatastoreSFCIDsUpdate updates the specified entity in the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreSFCIDsUpdate(ee *l2.HE2EEIDs) error {

	return nil
}

// DatastoreSFCIDsDelete deletes the specified entity from the sfc db in the etcd tree
func (cnpd *sfcCtlrL2CNPDriver) DatastoreSFCIDsDelete(ee *l2.HE2EEIDs) error {

	return nil
}
