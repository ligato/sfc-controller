//go:generate protoc --proto_path=model/vnf --gogo_out=model/vnf model/vnf/vnf.proto

// Package vnfdriver is a SFC plugin that controls generic VPP-based VNF functionality.
package vnfdriver

import (
	"github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/namsral/flag"
)

// PluginID is the plugin identifier.
const PluginID core.PluginName = "vnf-driver"

var (
	vnfConfigFile string // CLI flag - see RegisterFlags
	log           = logrus.DefaultLogger()
)

// Plugin is the main VNF driver plugin handler structure.
type Plugin struct {
	Etcd    *etcdv3.Plugin
	HTTPmux *rest.Plugin

	db         keyval.ProtoBroker
	yamlConfig *yamlConfig
}

func init() {
	log.SetLevel(logging.DebugLevel)

	RegisterFlags()
}

// RegisterFlags registers command line flags.
func RegisterFlags() {
	flag.StringVar(&vnfConfigFile, "vnf-config", "",
		"Name of a vnf config (yaml) file to load at startup")
}

// Init initializes the plugin. Automatically called by the plugin infra.
func (p *Plugin) Init() error {

	p.db = p.Etcd.NewBroker(keyval.Root)

	log.Infof("Initializing p '%s'", PluginID)

	if vnfConfigFile != "" {
		p.readConfigFromFile(vnfConfigFile)

		// render individual VNFs
		for _, e := range p.yamlConfig.VNFs {
			p.renderVNF(&e)
		}

		// TODO: write to ETCD

	} else {

		// TODO: implement reading from ETCD

		//log.Error("Rendering VNF from ETCD not supported yet.")
	}

	return nil
}

// Close cleans up the plugin. Automatically called by the plugin infra.
func (p *Plugin) Close() error {
	return nil
}
