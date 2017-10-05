package vnfdriver

import (
	"io/ioutil"

	"github.com/ghodss/yaml"
	"github.com/ligato/sfc-controller/plugins/vnfdriver/model/vnf"
)

type yamlConfig struct {
	Version     string          `json:"vnf_plugin_config_version"`
	Description string          `json:"description"`
	VNFs        []vnf.VnfEntity `json:"vnf_entities"`
}

func (p *Plugin) readConfigFromFile(fpath string) error {

	log.Debugf("Reading vnf-config from file: '%s'", fpath)

	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}

	p.yamlConfig = &yamlConfig{}

	if err := yaml.Unmarshal(b, p.yamlConfig); err != nil {
		return err
	}

	log.Debugf("vnf-config: %v", p.yamlConfig)

	return nil
}
