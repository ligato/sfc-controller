package itest

import (
	"os"
	"os/signal"
	"testing"
	"github.com/ligato/sfc-controller/tests/go/itest/sfctestdata"
	//"github.com/ligato/sfc-controller/tests/go/itest/vpptestdata"
	"github.com/ligato/sfc-controller/tests/go/itest/vpptestdata"
)

// Test runs all TC methods of multiple test suites in sequence
func Test(t *testing.T) {
	doneChan := make(chan struct{}, 1)

	go func() {
		t.Run("basic_tcs", func(t *testing.T) {
			suite := &basicTCSuite{T: t}
			t.Run("TC01ResyncEmptyVpp1Agent", func(t *testing.T) {
				suite.TC01ResyncEmptyVpp1Agent(&sfctestdata.VPP1MEMIF2,
					&vpptestdata.VPP1MEMIF1,
				)
			})
			t.Run("TC02HTTPPost", func(t *testing.T) {
				suite.TC02HTTPPost(&sfctestdata.VPP1MEMIF2,
					&vpptestdata.VPP1MEMIF1,
				)
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
