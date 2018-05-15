package itest

import (
	"os"
	"os/signal"
	"testing"

)

// Test runs all TC methods of multiple test suites in sequence
func Test(t *testing.T) {
	doneChan := make(chan struct{}, 1)

	go func() {
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
