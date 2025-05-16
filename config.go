package procoll

import (
	"os"
	"time"
)

// Config contains the procroll configuration.
type Config struct {
	TempDir         string
	ShutdownTimeout time.Duration
	ReadyTimeout    time.Duration
	ShutdownSignal  os.Signal
}
