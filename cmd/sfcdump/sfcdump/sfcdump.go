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

// Package sfcdump is a command-line tool for dumping the ETCD keys for each VPP label.
// Along with each key is a dump of each name/value pair in the structure.
package sfcdump

import (
	"fmt"
	"os"
	"strings"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logroot"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/ligato/sfc-controller/controller/utils"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l3plugin/model/l3"
	linuxIntf "github.com/ligato/vpp-agent/plugins/linuxplugin/ifplugin/model/interfaces"
)

var (
	EtcdVppLabelMap = make(map[string]interface{}) // keep this global as an upstream private project uses this
	log             = logroot.StandardLogger()
)

// SfcDump creates & returns db & dups the entities
func SfcDump() keyval.ProtoBroker {
	log.SetLevel(logging.DebugLevel)
	log.Println("Starting the etcd client...")

	_, db := createEtcdClient()

	sfcDatastoreSystemParametersDump(db)
	sfcDatastoreHostEntityDumpAll(db)
	sfcDatastoreExternalEntityDumpAll(db)
	sfcDatastoreSfcEntityDumpAll(db)
	for k := range EtcdVppLabelMap {
		fmt.Println("ETCD VPP LABEL: ", k)
		vnfDatastoreCustomLabelsDumpAll(db, k)
		vnfDatastoreInterfacesDumpAll(db, k)
		vnfDatastoreLinuxInterfacesDumpAll(db, k)
		vnfDatastoreBridgesDumpAll(db, k)
		vnfDatastoreL3RoutesDumpAll(db, k)
		vnfDatastoreInterfaceStatesDumpAll(db, k)
	}

	return db
}

func sfcDatastoreSystemParametersDump(db keyval.ProtoBroker) error {

	kvi, err := db.ListValues(controller.SystemParametersKey())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &controller.SystemParameters{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("SysParms: ", kv.GetKey(), entry)
	}
}

func sfcDatastoreSfcEntityDumpAll(db keyval.ProtoBroker) error {

	kvi, err := db.ListValues(controller.SfcEntityKeyPrefix())
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
		for _, sfcChainElement := range sfc.GetElements() {
			if sfcChainElement.EtcdVppSwitchKey != "" {
				EtcdVppLabelMap[sfcChainElement.EtcdVppSwitchKey] = sfcChainElement.EtcdVppSwitchKey
			}
			if sfcChainElement.Container != "" {
				EtcdVppLabelMap[sfcChainElement.Container] = sfcChainElement.Container
			}
		}
		fmt.Println("SFC: ", kv.GetKey(), sfc)
	}
}

func sfcDatastoreExternalEntityDumpAll(db keyval.ProtoBroker) error {

	kvi, err := db.ListValues(controller.ExternalEntityKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &controller.ExternalEntity{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		EtcdVppLabelMap[entry.Name] = entry.Name
		fmt.Println("EE: ", kv.GetKey(), entry)
	}
}

func sfcDatastoreHostEntityDumpAll(db keyval.ProtoBroker) error {

	kvi, err := db.ListValues(controller.HostEntityKeyPrefix())
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &controller.HostEntity{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		EtcdVppLabelMap[entry.Name] = entry.Name
		fmt.Println("HE: ", kv.GetKey(), entry)
	}
}

func vnfDatastoreInterfacesDumpAll(db keyval.ProtoBroker, etcdVppLabel string) error {

	kvi, err := db.ListValues(utils.InterfacePrefixKey(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &interfaces.Interfaces_Interface{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("Interface: ", kv.GetKey(), entry)
	}
}

func vnfDatastoreLinuxInterfacesDumpAll(db keyval.ProtoBroker, etcdVppLabel string) error {

	kvi, err := db.ListValues(utils.LinuxInterfacePrefixKey(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &linuxIntf.LinuxInterfaces_Interface{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("Linux i/f: ", kv.GetKey(), entry)
	}
}

func vnfDatastoreInterfaceStatesDumpAll(db keyval.ProtoBroker, etcdVppLabel string) error {

	kvi, err := db.ListValues(utils.InterfaceStatePrefixKey(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &interfaces.InterfacesState_Interface{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("Interface state: ", kv.GetKey(), entry)
	}
}

func vnfDatastoreBridgesDumpAll(db keyval.ProtoBroker, etcdVppLabel string) error {

	kvi, err := db.ListValues(utils.L2BridgeDomainKeyPrefix(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l2.BridgeDomains_BridgeDomain{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("BD: ", kv.GetKey(), entry)
	}
}

func vnfDatastoreL3RoutesDumpAll(db keyval.ProtoBroker, etcdVppLabel string) error {

	kvi, err := db.ListValues(utils.L3RouteKeyPrefix(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		entry := &l3.StaticRoutes_Route{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("Static route: ", kv.GetKey(), entry)
	}
}

func vnfDatastoreCustomLabelsDumpAll(db keyval.ProtoBroker, etcdVppLabel string) error {

	kvi, err := db.ListValues(utils.CustomInfoKey(etcdVppLabel))
	if err != nil {
		log.Fatal(err)
		return nil
	}

	for {
		kv, allReceived := kvi.GetNext()
		if allReceived {
			return nil
		}
		type label struct {
			value string
		}
		entry := &controller.CustomInfoType{}
		err := kv.GetValue(entry)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		fmt.Println("Custom Info: ", kv.GetKey(), entry)
	}
}

func createEtcdClient() (*etcdv3.BytesConnectionEtcd, keyval.ProtoBroker) {

	var err error
	var configFile string

	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		configFile = os.Args[1]
	} else {
		configFile = os.Getenv("ETCDV3_CONFIG")
	}
	fileConfiguration := &etcdv3.Config{}
	if configFile != "" {
		err = config.ParseConfigFromYamlFile(configFile, fileConfiguration)
		if err != nil {
			log.Error(err)
			return nil, nil
		}
	}
	cfg, err := etcdv3.ConfigToClientv3(fileConfiguration)
	if err != nil {
		log.Error(err)
		return nil, nil
	}

	bDB, err := etcdv3.NewEtcdConnectionWithBytes(*cfg, log)
	if err != nil {
		log.Fatal(err)
	}

	return bDB, kvproto.NewProtoWrapperWithSerializer(bDB, &keyval.SerializerJSON{})
}
