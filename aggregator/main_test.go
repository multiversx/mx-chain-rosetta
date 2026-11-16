package aggregator

import (
	"os"
	"testing"

	logger "github.com/multiversx/mx-chain-logger-go"
)

func TestMain(m *testing.M) {
	// Keep the output of the tests readable (failure scenarios are logged by the aggregator, on purpose).
	_ = logger.SetLogLevel("*:NONE")
	os.Exit(m.Run())
}
