package itest

import (
	"os"
	"os/signal"
	"testing"
	"github.com/ligato/sfc-controller/tests/go/itest/sfctestdata"
	"github.com/ligato/sfc-controller/tests/go/itest/vpptestdata"
	"github.com/ligato/sfc-controller/tests/go/itest/linuxtestdata"
	"github.com/golang/protobuf/proto"
)

// Test runs all TC methods of multiple test suites in sequence
func Test(t *testing.T) {
	doneChan := make(chan struct{}, 1)

	go func() {
		t.Run("basic_tcs", func(t *testing.T) {
			VPP1MEMIF2_Loopback_VETH := []proto.Message{&vpptestdata.VPP1MEMIF1,
				&vpptestdata.VPP1MEMIF2,
				&vpptestdata.Agent1Afpacket01,
				&vpptestdata.Agent1Loopback,
				&linuxtestdata.Agent1Veth01,
				&vpptestdata.BD_INTERNAL_EW_HOST1}

			t.Run("TC01ResyncEmptyVpp1Agent", func(t *testing.T) {
				suite := &basicTCSuite{T: t}
				suite.TC01ResyncEmptyVpp1Agent(&sfctestdata.VPP1MEMIF2_Loopback_VETH, VPP1MEMIF2_Loopback_VETH...)
			})
			t.Run("TC02HTTPPost", func(t *testing.T) {
				suite := &basicTCSuite{T: t}
				suite.TC02HTTPPost(&sfctestdata.VPP1MEMIF2_Loopback_VETH, VPP1MEMIF2_Loopback_VETH...)
			})
			t.Run("TC03CleanupAtStartupFlag", func(t *testing.T) {
				suite := &basicTCSuite{T: t}
				suite.TC03CleanupAtStartupFlag(&sfctestdata.VPP1MEMIF2_Loopback_VETH)
			})
			t.Run("TC04LoadConfigFile", func(t *testing.T) {
				suite := &basicTCSuite{T: t}
				suite.TC04LoadConfigFile(&sfctestdata.VPP1MEMIF2_Loopback_VETH)
			})
		})
		doneChan <- struct{}{}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	select {
	case <-doneChan:
		t.Log("Tests finished")
	case <-sigChan:
		t.Log("Interrupt received, returning.")
		t.Fatal("Interrupted by user")
		t.SkipNow()
		os.Exit(1) //TODO avoid this workaround
	}
}
