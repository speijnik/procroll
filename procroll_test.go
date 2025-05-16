package procoll

import (
	"io"
	"log/slog"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("Successful initialization", func(t *testing.T) {
		// Create test arguments
		args := []string{"test", "arg-{{.Generation}}"}

		// Create a test logger
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		// Create a test config
		conf := Config{
			TempDir:         "/tmp",
			ShutdownTimeout: 100 * time.Millisecond,
			ReadyTimeout:    100 * time.Millisecond,
			ShutdownSignal:  syscall.SIGTERM,
		}

		// Create a new manager
		manager, err := New(args, logger, conf)
		require.NoError(t, err, "New should not return an error")
		require.NotNil(t, manager, "New should return a non-nil manager")

		// Verify the manager implements the Manager interface
		assert.Implements(t, (*Manager)(nil), manager, "New should return a Manager implementation")
	})

	t.Run("Invalid template", func(t *testing.T) {
		// Create test arguments with an invalid template (unbalanced braces)
		args := []string{"test", "arg-{{.Generation"}

		// Create a test logger
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		// Create a test config
		conf := Config{
			TempDir:         "/tmp",
			ShutdownTimeout: 100 * time.Millisecond,
			ReadyTimeout:    100 * time.Millisecond,
			ShutdownSignal:  syscall.SIGTERM,
		}

		// Create a new manager
		manager, err := New(args, logger, conf)
		assert.Error(t, err, "New should return an error for invalid template")
		assert.Nil(t, manager, "New should return nil manager for invalid template")
		assert.Contains(t, err.Error(), "failed to parse argument", "Error message should mention failed parsing")
	})
}
