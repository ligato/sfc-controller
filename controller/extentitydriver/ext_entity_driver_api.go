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

// Package extentitydriver represents external entity driver that is used to configure external routers.
package extentitydriver

import (
	"fmt"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/ligato/sfc-controller/controller/utils"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/common/model/l3"
	"strings"
	"time"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/sfc-controller/controller/extentitydriver/iosxecfg"
	"github.com/ligato/sfc-controller/controller/extentitydriver/iosxecfg/model/iosxe"
	"net"
)

// TODO: make these constants configurable in the controller NB API?
const (
	VxlanSourceLoopbackID       = 0
	BridgeDomainServiceInstance = 20
)

const (
	eeOpSFCCtlrL2EEToHESSH      = 1
	eeOpSFCCtlrL2EEInternalsSSH = 2
)

// EEOperation is external entity operation
type EEOperation struct {
	ee  controller.ExternalEntity
	he  controller.HostEntity
	op  int
	vni uint32
	sr  *l3.StaticRoutes_Route
}

// external entity configuration
type eeConfig struct {
	nveInterface *iosxe.Interface
	bds          map[uint32]*iosxe.BridgeDomain
}

var eeConfigCache map[string]*eeConfig // map of external entity configurations indexed by ee mgmt IP address

// EEOperationChannel is channel for external entity operations
var EEOperationChannel = make(chan *EEOperation, 100)

var log = logrus.DefaultLogger()

// SfcExternalEntityDriverInit starts process for EEOperationChannel
func SfcExternalEntityDriverInit() {
	go processEEOperationChannel()
}

// SfcCtlrL2WireExternalEntityToHostEntity (called from the sfcctlr l2 driver) configures the bridge, vxlan tunnel, and static route
func SfcCtlrL2WireExternalEntityToHostEntity(ee controller.ExternalEntity, he controller.HostEntity,
	vni uint32, sr *l3.StaticRoutes_Route) error {

	switch ee.EeDriverType {
	case controller.ExtEntDriverType_EE_DRIVER_TYPE_IOSXE_SSH:

		eeOp := &EEOperation{
			ee:  ee,
			he:  he,
			op:  eeOpSFCCtlrL2EEToHESSH,
			vni: vni,
			sr:  sr,
		}

		EEOperationChannel <- eeOp

		return nil

	default:
		log.Infof("SfcCtlrL2WireExternalEntityToHostEntity: NO Driver configured: ee: %s, he: %s, vni: %d, static route: %s",
			ee.Name, he.Name, vni, sr.String())
	}
	return nil
}

// SfcCtlrL2WireExternalEntityInternals (called from the sfcctlr l2 driver) configures basic entities in prep for connecting to all hosts
func SfcCtlrL2WireExternalEntityInternals(ee controller.ExternalEntity) error {

	switch ee.EeDriverType {
	case controller.ExtEntDriverType_EE_DRIVER_TYPE_IOSXE_SSH:

		eeOp := &EEOperation{
			ee: ee,
			op: eeOpSFCCtlrL2EEInternalsSSH,
		}

		EEOperationChannel <- eeOp

		return nil

	default:
		log.Infof("SfcCtlrL2WireExternalEntityInternals: NO Driver configured: ee: %s", ee.Name)
	}
	return nil
}

func processEEOperationChannel() {
	for eeOp := range EEOperationChannel {

		if eeOp.ee.MgmntIpAddress == "0.0.0.0" || eeOp.ee.MgmntIpAddress == "" {
			log.Warn("Skipping EE Operation with null management IP address.")
			continue
		}

		switch eeOp.op {
		case eeOpSFCCtlrL2EEToHESSH:
			sfcCtlrL2WireExternalEntityToHostEntityUsingCli(&eeOp.ee, &eeOp.he, eeOp.vni, eeOp.sr)
		case eeOpSFCCtlrL2EEInternalsSSH:
			sfcCtlrL2WireExternalEntityInternalsUsingCli(&eeOp.ee)

		}
	}
}

func sfcCtlrL2WireExternalEntityToHostEntityUsingCli(ee *controller.ExternalEntity, he *controller.HostEntity,
	vni uint32, sr *l3.StaticRoutes_Route) error {

	log.Infof("sfcCtlrL2WireExternalEntityToHostEntityUsingCli: creating an ssh session (dstIP:%s) ee: %s, he: %s, vni: %d, bd: %s, static route: %s",
		ee.MgmntIpAddress, ee.Name, he.Name, vni, sr.String())

	s, err := connectToRouter(ee.MgmntIpAddress, ee.MgmntPort, ee.BasicAuthUser, ee.BasicAuthPasswd)
	if err != nil {
		log.Error(err)
		return err
	}
	defer s.Close()

	// configure static route
	ip := utils.TruncateString(sr.DstIpAddr, strings.Index(sr.DstIpAddr, "/"))
	err = s.AddStaticRoute(
		&iosxe.StaticRoute{
			ip + "/32",
			sr.NextHopAddr,
			"",
		})
	if err != nil {
		log.Error(err)
		return err
	}

	eeCfg := eeConfigCache[ee.MgmntIpAddress]

	// configure vxlan - NVE interface
	eeCfg.nveInterface.Vxlan = append(eeCfg.nveInterface.Vxlan, &iosxe.Interface_Vxlan{
		SrcInterfaceName: fmt.Sprintf("Loopback%d", VxlanSourceLoopbackID),
		Vni:              vni,
		DstAddress:       ip,
	})
	err = s.AddInterface(eeCfg.nveInterface)
	if err != nil {
		log.Error(err)
		return err
	}

	// add the VNI into the host_bd
	eeCfg.bds[ee.HostBd.Id].Vni = append(eeCfg.bds[ee.HostBd.Id].Vni, vni)
	err = s.AddBridgeDomain(eeCfg.bds[ee.HostBd.Id])
	if err != nil {
		log.Error(err)
		return err
	}

	s.CopyRunningToStartup()

	return nil
}

func sfcCtlrL2WireExternalEntityInternalsUsingCli(ee *controller.ExternalEntity) error {

	log.Infof("sfcCtlrL2WireExternalEntityInternalsUsingCli: creating an ssh session (ip:%s) ee: %s",
		ee.MgmntIpAddress, ee.Name)

	// init external entity config cache
	if eeConfigCache == nil {
		eeConfigCache = make(map[string]*eeConfig)
	}
	var eeCfg *eeConfig
	var ok bool
	if eeCfg, ok = eeConfigCache[ee.MgmntIpAddress]; !ok {
		eeCfg = &eeConfig{}
		eeCfg.bds = make(map[uint32]*iosxe.BridgeDomain)
		eeConfigCache[ee.MgmntIpAddress] = eeCfg
	}

	s, err := connectToRouter(ee.MgmntIpAddress, ee.MgmntPort, ee.BasicAuthUser, ee.BasicAuthPasswd)
	if err != nil {
		log.Error(err)
		return err
	}
	defer s.Close()

	// host_interface
	err = configureEEHostInterface(s, eeCfg, ee.HostInterface)
	if err != nil {
		log.Error(err)
		return err
	}

	// host_bd
	err = configureEEHostBD(s, eeCfg, ee.HostBd)
	if err != nil {
		log.Error(err)
		return err
	}

	// host_vxlan
	err = configureEEHostVxlan(s, eeCfg, ee.HostVxlan)
	if err != nil {
		log.Error(err)
		return err
	}

	s.CopyRunningToStartup()

	return nil
}

// connectToRouter is connecting to the router in a loop, until the connection succeeds
func connectToRouter(host string, port uint32, userName string, password string) (*iosxecfg.Session, error) {
	for {
		log.Debugf("Connecting to the router %s...", host)
		s, err := iosxecfg.NewSSHSession(host, port, userName, password)
		if err == nil {
			return s, nil
		}
		log.Debugf("Connection to the router %s failed, retrying...", host)
		time.Sleep(500 * time.Millisecond)
	}
}

// configureEEHostInterface configures external entity's host interface.
func configureEEHostInterface(s *iosxecfg.Session, eeCfg *eeConfig,
	hostIf *controller.ExternalEntity_HostInterface) error {

	// TODO: remove after resync is implemented
	s.DeleteInterface(&iosxe.Interface{Name: hostIf.IfName, Type: iosxe.InterfaceType_ETHERNET_CSMACD})

	err := s.AddInterface(&iosxe.Interface{
		Name:        hostIf.IfName,
		Type:        iosxe.InterfaceType_ETHERNET_CSMACD,
		Decription:  "host interface",
		IpAddress:   hostIf.Ipv4Addr,
		IpRedirects: true,
	})

	return err
}

// configureEEHostBD configures external entity's host bridge domain.
func configureEEHostBD(s *iosxecfg.Session, eeCfg *eeConfig, hostBd *controller.ExternalEntity_HostBD) error {

	// TODO: remove after resync is implemented
	s.DeleteBridgeDomain(&iosxe.BridgeDomain{Id: hostBd.Id})

	// create bridge domain cfg struct
	bd := &iosxe.BridgeDomain{
		Id: hostBd.Id,
	}

	// add bridge domain member interfaces
	for _, ifName := range hostBd.Interfaces {
		// configure the service instance on given interface

		// TODO: remove after resync is implemented
		s.DeleteInterface(&iosxe.Interface{Name: ifName, Type: iosxe.InterfaceType_ETHERNET_CSMACD})

		err := s.AddInterface(&iosxe.Interface{
			Name:        ifName,
			Type:        iosxe.InterfaceType_ETHERNET_CSMACD,
			IpRedirects: true,
			ServiceInstance: &iosxe.Interface_ServiceInstance{
				Id:            BridgeDomainServiceInstance,
				Encapsulation: "untagged",
			},
		})
		if err != nil {
			log.Error(err)
			return err
		}

		// set the same service instance on the bridge domain
		bd.Interfaces = append(bd.Interfaces, &iosxe.BridgeDomain_Interface{
			Name:            ifName,
			ServiceInstance: BridgeDomainServiceInstance,
		})
	}

	// configure the bridge domain
	err := s.AddBridgeDomain(bd)
	if err != nil {
		log.Error(err)
		return err
	}
	eeCfg.bds[bd.Id] = bd

	// configure the BDI interface
	if hostBd.BdiIpv4 != "" {
		// TODO: remove after resync is implemented
		s.DeleteInterface(&iosxe.Interface{Name: fmt.Sprintf("BDI%d", hostBd.Id),
			Type: iosxe.InterfaceType_BDI_INTERFACE})

		err = s.AddInterface(&iosxe.Interface{
			Name:        fmt.Sprintf("BDI%d", hostBd.Id),
			Type:        iosxe.InterfaceType_BDI_INTERFACE,
			Decription:  "bridge domain l3 interface",
			IpAddress:   hostBd.BdiIpv4,
			IpRedirects: false,
		})
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

// configureEEHostBD configures external entity's host VXLAN.
func configureEEHostVxlan(s *iosxecfg.Session, eeCfg *eeConfig, hostVxlan *controller.ExternalEntity_HostVxlan) error {

	// TODO: remove after resync is implemented
	s.DeleteInterface(&iosxe.Interface{Name: fmt.Sprintf("Loopback%d", VxlanSourceLoopbackID),
		Type: iosxe.InterfaceType_SOFTWARE_LOOPBACK})

	// create source loopback interface
	err := s.AddInterface(&iosxe.Interface{
		Name:        fmt.Sprintf("Loopback%d", VxlanSourceLoopbackID),
		Type:        iosxe.InterfaceType_SOFTWARE_LOOPBACK,
		Decription:  "source interface for the VXLAN tunnel",
		IpAddress:   hostVxlan.SourceIpv4,
		IpRedirects: true,
	})
	if err != nil {
		log.Error(err)
		return err
	}

	// TODO: remove after resync is implemented
	s.DeleteInterface(&iosxe.Interface{Name: hostVxlan.IfName, Type: iosxe.InterfaceType_NVE_INTERFACE})

	// create nve interface
	nve := &iosxe.Interface{
		Name:        hostVxlan.IfName,
		Type:        iosxe.InterfaceType_NVE_INTERFACE,
		Decription:  "VXLAN tunnel interface",
		IpRedirects: true,
	}
	err = s.AddInterface(nve)
	if err != nil {
		log.Error(err)
		return err
	}
	eeCfg.nveInterface = nve

	return nil
}

// ipv4ToIPMask converts ipv4 cidr prefix (e.g. 1.2.3.4/24) to IP address and netmask strings.
func ipv4ToIPMask(cidr string) (ip, netmask string, err error) {
	address, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return
	}
	ip = address.To4().String()
	netmask = net.IP(network.Mask).To4().String()
	return
}
