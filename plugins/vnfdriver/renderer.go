package vnfdriver

import (
	"fmt"

	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/vpp-agent/clientv1/linux"
	"github.com/ligato/vpp-agent/clientv1/linux/remoteclient"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"github.com/ligato/sfc-controller/plugins/vnfdriver/model/vnf"
)

func (p *Plugin) renderVNF(vnf *vnf.VnfEntity) error {

	log.Debugf("Rendering VNF entity '%s', container '%s'", vnf.Name, vnf.Container)

	for _, l2xconn := range vnf.L2Xconnects {
		err := p.renderVnfL2XConnect(vnf.Container, l2xconn)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Plugin) renderVnfL2XConnect(vppLabel string, l2xconn *vnf.VnfEntity_L2XConnect) error {

	if len(l2xconn.PortLabels) != 2 {
		err := fmt.Errorf("Incorrect count of port labels for l2xconnect: %d", len(l2xconn.PortLabels))
		log.Error(err)
		return err
	}

	log.Debugf("Rendering VNF l2xconnect ports: '%s' <-> '%s' for VPP '%s'",
		l2xconn.PortLabels[0], l2xconn.PortLabels[1], vppLabel)

	// l2xconnect port0 -> port1
	err := p.createXConnectPair(vppLabel, l2xconn.PortLabels[0], l2xconn.PortLabels[1])
	if err != nil {
		return err
	}

	// l2xconnect port1 -> port0
	err = p.createXConnectPair(vppLabel, l2xconn.PortLabels[1], l2xconn.PortLabels[0])
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugin) createXConnectPair(vppLabel, rxIf, txIf string) error {

	xconn := &l2.XConnectPairs_XConnectPair{
		ReceiveInterface:  rxIf,
		TransmitInterface: txIf,
	}

	log.Debugf("Storing l2xconnect config: %s", xconn)

	err := p.newRemoteClientTxn(vppLabel).Put().XConnect(xconn).Send().ReceiveReply()
	if err != nil {
		log.Errorf("Error by storing l2xconnect: %s", err)
		return err
	}

	return nil
}

func (p *Plugin) newRemoteClientTxn(microserviceLabel string) linux.DataChangeDSL {
	prefix := servicelabel.GetDifferentAgentPrefix(microserviceLabel)
	broker := p.Etcd.NewBroker(prefix)
	return remoteclient.DataChangeRequestDB(broker)
}
