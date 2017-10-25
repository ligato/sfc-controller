package itest

import (
	"os"
	"os/signal"
	"testing"
	"github.com/ligato/sfc-controller/tests/go/itest/sfctestdata"
	//"github.com/ligato/sfc-controller/tests/go/itest/vpptestdata"
	"github.com/ligato/sfc-controller/tests/go/itest/vpptestdata"
	"github.com/ligato/sfc-controller/tests/go/itest/linuxtestdata"
	"github.com/golang/protobuf/proto"
)

// Test runs all TC methods of multiple test suites in sequence
func Test(t *testing.T) {
	doneChan := make(chan struct{}, 1)

	go func() {
		t.Run("basic_tcs", func(t *testing.T) {
			VPP1MEMIF2LoopbackVETH := []proto.Message{&vpptestdata.VPP1MEMIF1,
				&vpptestdata.VPP1MEMIF2,
				&vpptestdata.Agent1Afpacket01,
				&vpptestdata.Agent1Loopback,
				&linuxtestdata.Agent1Veth01,
				&vpptestdata.BDINTERNALEWHOST1}

			t.Run("TC01ResyncEmptyVpp1Agent", func(t *testing.T) {
				suite := &basicTCSuite{T: t}
				suite.TC01ResyncEmptyVpp1Agent(&sfctestdata.VPP1MEMIF2LoopbackVETH, VPP1MEMIF2LoopbackVETH...)
			})
			t.Run("TC02HTTPPost", func(t *testing.T) {
				suite := &basicTCSuite{T: t}
				suite.TC02HTTPPost(&sfctestdata.VPP1MEMIF2LoopbackVETH, VPP1MEMIF2LoopbackVETH...)
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
