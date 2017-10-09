package itest

import (
	"encoding/json"
	"github.com/ligato/cn-infra/core"
	etcdmock "github.com/ligato/cn-infra/db/keyval/etcdv3/mocks"
	httpmock "github.com/ligato/cn-infra/rpc/rest/mock"
	"github.com/onsi/gomega"
	"testing"
	//etcdmock "github.com/ligato/cn-infra/db/keyval/etcdv3/mocks"
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	agent_api "github.com/ligato/cn-infra/core"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval/etcdv3"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/health/probe"
	"github.com/ligato/cn-infra/logging/logmanager"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/servicelabel"
	sfccore "github.com/ligato/sfc-controller/controller/core"
	"github.com/ligato/sfc-controller/controller/model/controller"
	"github.com/ligato/sfc-controller/plugins/vnfdriver"
	vppiface "github.com/ligato/vpp-agent/plugins/defaultplugins/ifplugin/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"io/ioutil"
	"github.com/ligato/cn-infra/logging/logroot"
	"time"
)

// AgentTestHelper is similar to what testing.T is in golang packages.
type AgentTestHelper struct {
	// agent for sfcFlavor
	sfcAgent *core.Agent
	// sfc controller plugins with it's own connectivty to ETCD
	sfcFalvor *Flavor
	// testing purposes only connectivity to ETCD
	tFlavor *TestingConFlavor
	// agent for tFlavor
	tAgent *core.Agent

	httpMock *httpmock.HTTPMock

	golangT *testing.T
	stopDB  func()
}

// Given is composition of multiple test step methods (see BDD Given keyword)
type Given struct {
	agentT *AgentTestHelper
}

// When is composition of multiple test step methods (see BDD When keyword)
type When struct {
}

// Then is composition of multiple test step methods (see BDD Then keyword)
type Then struct {
	agentT *AgentTestHelper
}

// DefaultSetup initializes the SFC Controller with embedded ETCD
func (t *AgentTestHelper) DefaultSetup(golangT *testing.T) {
	gomega.RegisterTestingT(golangT)
	t.golangT = golangT

	tFlavorLocal := &local.FlavorLocal{ServiceLabel: servicelabel.Plugin{MicroserviceLabel: "test-utils"}}
	etcdPlug, embedETCD := StartEmbeddedETCD(golangT, tFlavorLocal)
	t.tFlavor = &TestingConFlavor{
		FlavorLocal: tFlavorLocal,
		ETCD:        *etcdPlug,
	}
	t.stopDB = embedETCD.Stop

	t.httpMock = MockHTTP()

	t.sfcFalvor = &Flavor{
		FlavorLocal: &local.FlavorLocal{ServiceLabel: servicelabel.Plugin{MicroserviceLabel: "sfc-controller"}},
		HTTP:        *rest.FromExistingServer(t.httpMock.SetHandler),
		ETCD:        *etcdPlug}

	t.tAgent = core.NewAgent(tFlavorLocal.LoggerFor("tAgent"), 1*time.Second, t.tFlavor.Plugins()...)
	err := t.tAgent.Start()
	if err != nil {
		panic(err)
	}

	t.sfcAgent = core.NewAgent(logroot.StandardLogger(), 2000*time.Second, t.sfcFalvor.Plugins()...)
}

// StartAgent in test (if there is error than panic => fail test)
func (t *Given) StartAgent() {
	err := t.agentT.sfcAgent.Start()
	if err != nil {
		t.agentT.golangT.Fatal("error starting sfcAgent ", err)
	}
}

// EmptyETCD deletes all keys in ETCD
func (t *Given) EmptyETCD() {
	db := t.agentT.tFlavor.ETCD.NewBroker("" /*TODO use Root Const*/)
	db.Delete("/", datasync.WithPrefix())
}

// ConfigSFCviaETCD puts SFC config to keyvalue store (e.g. ETCD)
func (t *Given) ConfigSFCviaETCD(cfg *sfccore.YamlConfig) {
	db := t.agentT.tFlavor.ETCD.NewBroker("" /*TODO use Root Const*/)

	for _, hostEntity := range cfg.HEs {
		db.Put(controller.HostEntityNameKey(hostEntity.Name), &hostEntity)
	}

	for _, sfcEntity := range cfg.SFCs {
		db.Put(controller.SfcEntityNameKey(sfcEntity.Name), &sfcEntity)
	}

	for _, extEntity := range cfg.EEs {
		db.Put(controller.ExternalEntityNameKey(extEntity.Name), &extEntity)
	}
}

// ConfigSFCviaREST posts SFC config via REST
func (t *Given) ConfigSFCviaREST(cfg *sfccore.YamlConfig) {
	for _, hostEntity := range cfg.HEs {
		data, _ := json.Marshal(hostEntity)
		httpResp, _ := t.agentT.httpMock.NewRequest("POST", "http://127.0.0.1"+
			controller.HostEntityNameKey(hostEntity.Name), bytes.NewReader(data))
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ = ioutil.ReadAll(httpResp.Body)
		fmt.Println("xxx httpResp.Body post HEs ", string(data))
	}

	for _, sfcEntity := range cfg.SFCs {
		data, _ := json.Marshal(sfcEntity)
		httpResp, _ := t.agentT.httpMock.NewRequest("POST", "http://127.0.0.1"+
			controller.SfcEntityNameKey(sfcEntity.Name), bytes.NewReader(data))
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ = ioutil.ReadAll(httpResp.Body)
		fmt.Println("xxx httpResp.Body post SFCs ", string(data))
	}

	for _, extEntity := range cfg.EEs {
		data, _ := json.Marshal(extEntity)
		httpResp, _ := t.agentT.httpMock.NewRequest("POST", "http://127.0.0.1"+
			controller.ExternalEntityNameKey(extEntity.Name), bytes.NewReader(data))
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ = ioutil.ReadAll(httpResp.Body)
		fmt.Println("xxx httpResp.Body post EEs ", string(data))
	}
}

// VppAgentcCfgContains
func (t *Then) VppAgentCfgContains(micorserviceLabel string, interfaceBDEtc ...proto.Message) {
	db := t.agentT.tFlavor.ETCD.NewBroker(servicelabel.GetDifferentAgentPrefix(micorserviceLabel))

	for _, expected := range interfaceBDEtc {
		switch expected.(type) {
		case *vppiface.Interfaces_Interface:
			ifaceExpected := expected.(*vppiface.Interfaces_Interface)
			ifaceExist := &vppiface.Interfaces_Interface{}
			key := vppiface.InterfaceKey(ifaceExpected.Name)
			found, _, err := db.GetValue(key, ifaceExist)
			gomega.Expect(found).Should(gomega.BeTrue(), "interface not found "+key)
			gomega.Expect(err).Should(gomega.BeNil(), "error reading "+key)
			gomega.Expect(ifaceExist).Should(gomega.BeEquivalentTo(ifaceExpected), "error reading "+key)
		case *l2.BridgeDomains_BridgeDomain:
			bdExpected := expected.(*l2.BridgeDomains_BridgeDomain)
			bdActual := &l2.BridgeDomains_BridgeDomain{}

			key := l2.BridgeDomainKey(bdExpected.Name)
			found, _, err := db.GetValue(key, bdActual)
			gomega.Expect(found).Should(gomega.BeTrue(), "bd not found "+key)
			gomega.Expect(err).Should(gomega.BeNil(), "error reading "+key)
			gomega.Expect(bdActual).Should(gomega.BeEquivalentTo(bdExpected), "error reading "+key)
		}
	}
}

// HTTPGet simulates the HTTP call
func (t *Then) HTTPGetEntities(sfcCfg *sfccore.YamlConfig) {
	{ //SFCs
		url := "http://127.0.0.1/sfc-controller/api/v1/SFCs"
		httpResp, err := t.agentT.httpMock.NewRequest("GET", url, nil)
		gomega.Expect(err).Should(gomega.BeNil(), "error reading getting SFC entities")
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ := ioutil.ReadAll(httpResp.Body)
		fmt.Println("xxx httpResp.Body SFCs ", string(data))
	}
	{ //HEs
		url := "http://127.0.0.1/sfc-controller/api/v1/HEs"
		httpResp, err := t.agentT.httpMock.NewRequest("GET", url, nil)
		gomega.Expect(err).Should(gomega.BeNil(), "error reading getting HE entities")
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ := ioutil.ReadAll(httpResp.Body)
		fmt.Println("xxx httpResp.Body HEs ", string(data))
	}
	{ //EEs
		url := "http://127.0.0.1/sfc-controller/api/v1/EEs"
		httpResp, err := t.agentT.httpMock.NewRequest("GET", url, nil)
		gomega.Expect(err).Should(gomega.BeNil(), "error reading getting EE entities")
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ := ioutil.ReadAll(httpResp.Body)
		fmt.Println("xxx httpResp.Body EEs ", string(data))
	}

	for _, expected := range sfcCfg.SFCs {
		url := "http://127.0.0.1" + controller.SfcEntityNameKey(expected.Name)
		fmt.Println("xxx url: " + url)
		httpResp, err := t.agentT.httpMock.NewRequest("GET", url, nil)
		gomega.Expect(err).Should(gomega.BeNil(), "error reading getting SFC entity")
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ := ioutil.ReadAll(httpResp.Body)
		actual := &controller.SfcEntity{}
		json.Unmarshal(data, actual)
		gomega.Expect(actual).Should(gomega.BeEquivalentTo(&expected), "not eq sfc entities")
	}
	for _, expected := range sfcCfg.HEs {
		url := "http://127.0.0.1" + controller.HostEntityNameKey(expected.Name)
		httpResp, err := t.agentT.httpMock.NewRequest("GET", url, nil)
		gomega.Expect(err).Should(gomega.BeNil(), "error reading getting SFC entity")
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ := ioutil.ReadAll(httpResp.Body)
		actual := &controller.HostEntity{}
		json.Unmarshal(data, actual)
		gomega.Expect(actual).Should(gomega.BeEquivalentTo(&expected), "not eq host entities")
	}
	for _, expected := range sfcCfg.EEs {
		url := "http://127.0.0.1" + controller.ExternalEntityNameKey(expected.Name)
		httpResp, err := t.agentT.httpMock.NewRequest("GET", url, nil)
		gomega.Expect(err).Should(gomega.BeNil(), "error reading getting SFC entity")
		gomega.Expect(httpResp.StatusCode).Should(gomega.BeEquivalentTo(200), "not HTTP 200")
		data, _ := ioutil.ReadAll(httpResp.Body)
		actual := &controller.ExternalEntity{}
		json.Unmarshal(data, actual)
		gomega.Expect(actual).Should(gomega.BeEquivalentTo(&expected), "not eq external entities")
	}
}

// Teardown stops the sfcAgent
func (t *AgentTestHelper) Teardown() {
	if t.sfcAgent != nil {
		err := t.sfcAgent.Stop()
		if err != nil {
			t.golangT.Fatal("error stoppig sfcAgent ", err)
		}
	}
	if t.stopDB != nil {
		t.stopDB()
	}
	if t.tAgent != nil {
		err := t.tAgent.Stop()
		if err != nil {
			t.golangT.Fatal("error stoppig sfcAgent ", err)
		}
	}
}

// MockHTTP returns new instance of HTTP mock
// (usefull to avoid import aliases in other files)
func MockHTTP() *httpmock.HTTPMock {
	return &httpmock.HTTPMock{}
}

// StartEmbeddedETCD initializes embedded ETCD & returns plugin instance for accessing it
func StartEmbeddedETCD(t *testing.T, flavorLocal *local.FlavorLocal) (*etcdv3.Plugin, *etcdmock.Embedded) {
	embeddedETCD := etcdmock.Embedded{}
	embeddedETCD.Start(t)

	etcdClientLogger := flavorLocal.LoggerFor("embedEtcdClient")
	etcdBytesCon, err := etcdv3.NewEtcdConnectionUsingClient(embeddedETCD.Client(), etcdClientLogger)
	if err != nil {
		panic(err)
	}

	return etcdv3.FromExistingConnection(etcdBytesCon, &flavorLocal.ServiceLabel), &embeddedETCD
}

// Flavor is set of common used generic plugins. This flavour can be used as a base
// for different flavours. The plugins are initialized in the same order as they appear
// in the structure.
type Flavor struct {
	*local.FlavorLocal
	HTTP      rest.Plugin
	HealthRPC probe.Plugin
	LogMngRPC logmanager.Plugin
	ETCD      etcdv3.Plugin

	Sfc       sfccore.SfcControllerPluginHandler
	VNFDriver vnfdriver.Plugin

	injected bool
}

// Inject interconnects plugins - injects the dependencies. If it has been called
// already it is no op.
func (f *Flavor) Inject() bool {
	if f.injected {
		return false
	}

	f.FlavorLocal.Inject()

	httpLogDeps := f.LogDeps("http")
	f.HTTP.Deps.Log = httpLogDeps.Log
	f.HTTP.Deps.PluginName = httpLogDeps.PluginName

	logMngLogDeps := f.LogDeps("log-mng-rpc")
	f.LogMngRPC.Deps.Log = logMngLogDeps.Log
	f.LogMngRPC.Deps.PluginName = logMngLogDeps.PluginName
	f.LogMngRPC.LogRegistry = f.FlavorLocal.LogRegistry()
	f.LogMngRPC.HTTP = &f.HTTP

	f.HealthRPC.Deps.PluginLogDeps = *f.LogDeps("health-rpc")
	f.HealthRPC.Deps.HTTP = &f.HTTP
	f.HealthRPC.Deps.StatusCheck = &f.StatusCheck

	f.ETCD.Deps.PluginInfraDeps = *f.InfraDeps("etcdv3")

	f.Sfc.Etcd = &f.ETCD
	f.Sfc.HTTPmux = &f.HTTP

	f.VNFDriver.Etcd = &f.ETCD
	f.VNFDriver.HTTPmux = &f.HTTP

	f.injected = true

	return true
}

// Plugins returns all plugins from the flavour. The set of plugins is supposed
// to be passed to the sfcAgent constructor. The method calls inject to make sure that
// dependencies have been injected.
func (f *Flavor) Plugins() []*agent_api.NamedPlugin {
	f.Inject()
	return agent_api.ListPluginsInFlavor(f)
}

// TestingConFlavor - just ETCD connectivity
type TestingConFlavor struct {
	*local.FlavorLocal
	ETCD     etcdv3.Plugin
	injected bool
}

// Inject interconnects plugins - injects the dependencies. If it has been called
// already it is no op.
func (f *TestingConFlavor) Inject() bool {
	if f.injected {
		return false
	}

	f.FlavorLocal.Inject()

	f.ETCD.Deps.PluginInfraDeps = *f.InfraDeps("etcdv3")

	f.injected = true

	return true
}

// Plugins returns all plugins from the flavour. The set of plugins is supposed
// to be passed to the sfcAgent constructor. The method calls inject to make sure that
// dependencies have been injected.
func (f *TestingConFlavor) Plugins() []*agent_api.NamedPlugin {
	f.Inject()
	return agent_api.ListPluginsInFlavor(f)
}
