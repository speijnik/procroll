package procoll

import (
	"context"
	"io"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGeneration implements the generation interface for testing.
type mockGeneration struct {
	identifier uint64

	spawnFunc     func() error
	waitReadyFunc func(timeout time.Duration) error
	shutdownFunc  func(signal os.Signal, timeout time.Duration) error
	idFunc        func() uint64

	// Add fields to track state and avoid nil pointer dereferences
	cmdExited    context.CancelFunc
	cmdExitedCtx context.Context
}

func (m *mockGeneration) spawn() error {
	// Create context for cmdExited if not already created
	if m.cmdExitedCtx == nil {
		m.cmdExitedCtx, m.cmdExited = context.WithCancel(context.Background())
	}

	if m.spawnFunc != nil {
		return m.spawnFunc()
	}
	return nil
}

func (m *mockGeneration) waitReady(timeout time.Duration) error {
	if m.waitReadyFunc != nil {
		return m.waitReadyFunc(timeout)
	}
	return nil
}

func (m *mockGeneration) shutdown(signal os.Signal, timeout time.Duration) error {
	if m.shutdownFunc != nil {
		return m.shutdownFunc(signal, timeout)
	}
	return nil
}

func (m *mockGeneration) id() uint64 {
	if m.idFunc != nil {
		return m.idFunc()
	}
	return m.identifier
}

// setupTestManager creates a manager with mocked dependencies for testing.
func setupTestManager(t *testing.T) (*manager, func()) {
	t.Helper()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "procroll-manager-test-") //nolint:usetesting // test tempdir too long for socket on macOS
	require.NoError(t, err)

	// Create a test logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a mock execer
	mockExec := &mockExecer{}

	// Create test arg templates
	argTemplates := []*template.Template{
		template.Must(template.New("arg0").Parse("test")),
		template.Must(template.New("arg1").Parse("arg-{{.Generation}}")),
	}

	// Create a test config
	conf := Config{
		TempDir:         tempDir,
		ShutdownTimeout: 100 * time.Millisecond,
		ReadyTimeout:    100 * time.Millisecond,
		ShutdownSignal:  syscall.SIGTERM,
	}

	// Create the manager
	m := &manager{
		argTemplates:  argTemplates,
		logger:        logger,
		generation:    0,
		generations:   make(map[uint64]generation),
		conf:          conf,
		execer:        mockExec,
		newGeneration: nil, // Will be set in tests
	}

	// Return cleanup function
	cleanup := func() {
		// Cancel shutdown context if it exists
		if m.shutdownComplete != nil {
			m.shutdownComplete()
		}

		// Cancel any generation contexts to prevent goroutine leaks
		m.generationsMutex.Lock()
		defer m.generationsMutex.Unlock()
		for _, g := range m.generations {
			if mockG, ok := g.(*mockGeneration); ok && mockG.cmdExited != nil {
				mockG.cmdExited()
			}
		}

		// Remove the temporary directory
		os.RemoveAll(tempDir)
	}

	return m, cleanup
}

func TestManager_Start(t *testing.T) {
	t.Run("Successful start", func(t *testing.T) {
		m, cleanup := setupTestManager(t)
		defer cleanup()

		// Mock the newGeneration function
		mockGen := &mockGeneration{identifier: 1}
		genCreatedCtx, genCreated := context.WithCancel(t.Context())
		m.newGeneration = func(_ *slog.Logger, identifier uint64, args []string, _ string, _ execer) generation {
			genCreated()
			assert.Equal(t, uint64(1), identifier)
			assert.Equal(t, []string{"test", "arg-1"}, args)
			return mockGen
		}

		// Start the manager
		err := m.Start()
		assert.NoError(t, err)
		select {
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "newGeneration was not called")
		case <-genCreatedCtx.Done():
			// fall-through
		}

		// Verify that the generation was added to the map
		assert.Len(t, m.generations, 1)
		assert.Equal(t, mockGen, m.generations[1])
	})

	t.Run("Failed to spawn processGeneration", func(t *testing.T) {
		m, cleanup := setupTestManager(t)
		defer cleanup()

		// Mock the newGeneration function to return a generation that fails to spawn
		mockGen := &mockGeneration{
			identifier: 1,
			spawnFunc: func() error {
				return context.Canceled
			},
		}
		m.newGeneration = func(_ *slog.Logger, _ uint64, _ []string, _ string, _ execer) generation {
			return mockGen
		}

		// Start the manager
		err := m.Start()
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Failed to wait ready", func(t *testing.T) {
		m, cleanup := setupTestManager(t)
		defer cleanup()

		// Mock the newGeneration function to return a generation that fails to wait ready
		mockGen := &mockGeneration{
			identifier: 1,
			waitReadyFunc: func(_ time.Duration) error {
				return context.Canceled
			},
		}
		m.newGeneration = func(_ *slog.Logger, _ uint64, _ []string, _ string, _ execer) generation {
			return mockGen
		}

		// Start the manager
		err := m.Start()
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestManager_Wait(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a context for shutdown
	m.shutdownCompleteCtx, m.shutdownComplete = context.WithCancel(t.Context())

	// Start a goroutine to wait for shutdown
	waitDone := make(chan struct{})
	go func() {
		err := m.Wait()
		assert.NoError(t, err)
		close(waitDone)
	}()

	// Trigger shutdown
	m.shutdownComplete()

	// Wait for the Wait method to return
	select {
	case <-waitDone:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait did not return after shutdown")
	}
}

func TestManager_Roll(t *testing.T) {
	t.Run("Successful roll", func(t *testing.T) {
		m, cleanup := setupTestManager(t)
		defer cleanup()

		// Create a previous generation
		prevGen := &mockGeneration{identifier: 1}
		m.generations[1] = prevGen
		m.generation = 1

		// Mock the newGeneration function
		newGen := &mockGeneration{identifier: 2}
		m.newGeneration = func(_ *slog.Logger, identifier uint64, _ []string, _ string, _ execer) generation {
			assert.Equal(t, uint64(2), identifier)
			return newGen
		}

		// Track shutdown of previous generation
		shutdownCalledCtx, shutdownCalled := context.WithCancel(t.Context())
		prevGen.shutdownFunc = func(signal os.Signal, timeout time.Duration) error {
			shutdownCalled()
			assert.Equal(t, syscall.SIGTERM, signal)
			assert.Equal(t, m.conf.ShutdownTimeout, timeout)
			return nil
		}

		// Roll the manager
		err := m.Roll()
		assert.NoError(t, err)

		// Verify that the new generation was added to the map
		assert.Contains(t, m.generations, uint64(2))
		assert.Equal(t, newGen, m.generations[2])

		// Verify that the previous generation was shut down
		select {
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "shutdown was not called on previous processGeneration")
		case <-shutdownCalledCtx.Done():
		}
	})

	t.Run("Failed to spawn new processGeneration", func(t *testing.T) {
		m, cleanup := setupTestManager(t)
		defer cleanup()

		// Create a previous generation
		prevGen := &mockGeneration{identifier: 1}
		m.generations[1] = prevGen
		m.generation = 1

		// Mock the newGeneration function to return a generation that fails to spawn
		newGen := &mockGeneration{
			identifier: 2,
			spawnFunc: func() error {
				return context.Canceled
			},
		}
		m.newGeneration = func(_ *slog.Logger, _ uint64, _ []string, _ string, _ execer) generation {
			return newGen
		}

		// Roll the manager
		err := m.Roll()
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)

		// Verify that only the previous generation is in the map
		assert.Len(t, m.generations, 1)
		assert.Contains(t, m.generations, uint64(1))
	})

	t.Run("Failed to wait ready", func(t *testing.T) {
		m, cleanup := setupTestManager(t)
		defer cleanup()

		// Create a previous generation
		prevGen := &mockGeneration{identifier: 1}
		m.generations[1] = prevGen
		m.generation = 1

		// Mock the newGeneration function to return a generation that fails to wait ready
		newGen := &mockGeneration{
			identifier: 2,
			waitReadyFunc: func(_ time.Duration) error {
				return context.Canceled
			},
		}
		m.newGeneration = func(_ *slog.Logger, _ uint64, _ []string, _ string, _ execer) generation {
			return newGen
		}

		// Track shutdown of new generation
		shutdownCalled := false
		newGen.shutdownFunc = func(signal os.Signal, _ time.Duration) error {
			shutdownCalled = true
			assert.Equal(t, syscall.SIGKILL, signal)
			return nil
		}

		// Roll the manager
		err := m.Roll()
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)

		// Verify that the new generation was killed
		assert.True(t, shutdownCalled, "shutdown was not called on new processGeneration")
	})
}

func TestManager_Shutdown(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a context for shutdown
	m.shutdownCompleteCtx, m.shutdownComplete = context.WithCancel(t.Context())

	// Create some generations with initialized contexts
	gen1 := &mockGeneration{identifier: 1}
	gen1.cmdExitedCtx, gen1.cmdExited = context.WithCancel(t.Context())
	gen2 := &mockGeneration{identifier: 2}
	gen2.cmdExitedCtx, gen2.cmdExited = context.WithCancel(t.Context())
	m.generations[1] = gen1
	m.generations[2] = gen2

	// Track shutdown of generations
	gen1ShutdownCtx, gen1ShutdownCalled := context.WithCancel(t.Context())
	defer gen1ShutdownCalled()

	gen2ShutdownCtx, gen2ShutdownCalled := context.WithCancel(t.Context())
	defer gen2ShutdownCalled()

	gen1.shutdownFunc = func(signal os.Signal, _ time.Duration) error {
		gen1ShutdownCalled()
		assert.Equal(t, syscall.SIGTERM, signal)
		return nil
	}

	gen2.shutdownFunc = func(signal os.Signal, _ time.Duration) error {
		gen2ShutdownCalled()
		assert.Equal(t, syscall.SIGTERM, signal)
		return nil
	}

	// Shutdown the manager
	err := m.Shutdown()
	assert.NoError(t, err)

	timeout := time.After(500 * time.Millisecond)

	select {
	case <-timeout:
		assert.Fail(t, "shutdown was not called on processGeneration 1")
	case <-gen1ShutdownCtx.Done():
		// fall-through
	}

	select {
	case <-timeout:
		assert.Fail(t, "shutdown was not called on processGeneration 2")
	case <-gen2ShutdownCtx.Done():
		// fall-through
	}
}

func TestManager_Stop(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a context for shutdown
	m.shutdownCompleteCtx, m.shutdownComplete = context.WithCancel(t.Context())

	// Create some generations with initialized contexts
	gen1 := &mockGeneration{identifier: 1}
	gen1.cmdExitedCtx, gen1.cmdExited = context.WithCancel(t.Context())
	m.generations[1] = gen1

	// Track shutdown of generations
	shutdownCalledCtx, shutdownCalled := context.WithCancel(t.Context())
	defer shutdownCalled()
	gen1.shutdownFunc = func(signal os.Signal, _ time.Duration) error {
		shutdownCalled()
		assert.Equal(t, syscall.SIGKILL, signal)
		return nil
	}

	// Stop the manager
	err := m.Stop()
	assert.NoError(t, err)

	select {
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "shutdown was not called on processGeneration")
	case <-shutdownCalledCtx.Done():
		// fall-through
	}
}

func TestManager_SpawnGeneration(t *testing.T) {
	m, cleanup := setupTestManager(t)
	defer cleanup()

	// Mock the newGeneration function
	mockGen := &mockGeneration{identifier: 1}
	m.newGeneration = func(_ *slog.Logger, identifier uint64, args []string, _ string, _ execer) generation {
		assert.Equal(t, uint64(1), identifier)
		assert.Equal(t, []string{"test", "arg-1"}, args)
		return mockGen
	}

	// Spawn a generation
	gen, err := m.spawnGeneration(1)
	assert.NoError(t, err)
	assert.Equal(t, mockGen, gen)

	// Verify that the generation was added to the map
	assert.Contains(t, m.generations, uint64(1))
	assert.Equal(t, mockGen, m.generations[1])
}
