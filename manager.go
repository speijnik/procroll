package procoll

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"syscall"
	"text/template"
)

// Manager defines the procroll manager interface.
type Manager interface {
	// Start causes the wrapped process to be executed and the overall manager started
	Start() error

	// Wait for manager to shut down
	Wait() error

	// Roll executes the rolling update logic
	Roll() error

	// Shutdown causes the manager to shut down cleanly
	Shutdown() error

	// Stop causes the manager to shut down immediately
	Stop() error
}

type manager struct {
	argTemplates        []*template.Template
	logger              *slog.Logger
	generation          uint64
	generations         map[uint64]generation
	generationsMutex    sync.Mutex
	shutdownComplete    context.CancelFunc
	shutdownCompleteCtx context.Context
	conf                Config
	tempDir             string
	execer              execer
	newGeneration       func(logger *slog.Logger, identifier uint64, args []string, socketPath string, execer execer) generation
}

func (m *manager) Start() error {
	m.logger.Info("rolling initial processGeneration...")
	m.shutdownCompleteCtx, m.shutdownComplete = context.WithCancel(context.Background())

	var err error
	m.tempDir, err = os.MkdirTemp(m.conf.TempDir, fmt.Sprintf("procoll-%d-", os.Getpid()))
	if err != nil {
		m.logger.Error("failed to create tempdir", "error", err)
		return err
	}

	firstGen, err := m.spawnGeneration(atomic.AddUint64(&m.generation, 1))
	if err != nil {
		m.logger.Error("failed to spawn initial processGeneration", "error", err)
		return err
	}

	if err = firstGen.waitReady(m.conf.ReadyTimeout); err != nil {
		m.logger.Error("first processGeneration failed to confirm readiness", "error", err)
		return err
	}
	m.logger.Info("first processGeneration running")
	if notifyErr := SDNotify(NotifyStateReady); notifyErr != nil {
		m.logger.Info("failed to send ready status to systemd", "error", notifyErr)
	}

	return nil
}

func (m *manager) spawnGeneration(genIdentifier uint64) (generation, error) {
	notifySocketPath := path.Join(m.tempDir, fmt.Sprintf("generation%d", genIdentifier))
	args, err := processArgs(genIdentifier, notifySocketPath, m.argTemplates)
	if err != nil {
		return nil, err
	}
	gen := m.newGeneration(m.logger, genIdentifier, args, notifySocketPath, m.execer)

	if err = gen.spawn(); err != nil {
		return nil, err
	}

	m.generationsMutex.Lock()
	m.generations[genIdentifier] = gen
	m.generationsMutex.Unlock()

	return gen, nil
}

func (m *manager) Wait() error {
	<-m.shutdownCompleteCtx.Done()
	if m.tempDir != "" {
		if err := os.RemoveAll(m.tempDir); err != nil {
			m.logger.Error("failed to remove tempdir", "error", err)
			// error is intentionally not returned
		}
	}
	return nil
}

func (m *manager) Roll() error {
	genIdentifier := atomic.AddUint64(&m.generation, 1)

	if notifyErr := SDNotifyReloading(); notifyErr != nil {
		m.logger.Info("failed to send reloading status to systemd", "error", notifyErr)
	} else {
		m.logger.Debug("sent reloading status to systemd", "processGeneration", genIdentifier)
	}

	m.logger.Info("spawning new processGeneration", "processGeneration", genIdentifier)
	gen, err := m.spawnGeneration(genIdentifier)
	if err != nil {
		m.logger.Error("failed to spawn processGeneration", "processGeneration", genIdentifier, "error", err)
		return err
	}

	previousGeneration := m.generations[genIdentifier-1]

	if err = gen.waitReady(m.conf.ReadyTimeout); err != nil {
		m.logger.Error("processGeneration failed to confirm readiness", "processGeneration", genIdentifier, "error", err)
		_ = gen.shutdown(syscall.SIGKILL, m.conf.ShutdownTimeout)
		return err
	}

	if notifyErr := SDNotify(NotifyStateReady); notifyErr != nil {
		m.logger.Info("failed to send ready status to systemd", "error", notifyErr)
	} else {
		m.logger.Debug("sent ready status to systemd", "processGeneration", genIdentifier)
	}

	m.logger.Info("new processGeneration ready", "processGeneration", genIdentifier)
	if notifyErr := SDNotify(NotifyStateStatusPrefix + fmt.Sprintf("processGeneration %d running", genIdentifier)); notifyErr != nil {
		m.logger.Info("failed to send status to systemd", "error", notifyErr)
	}

	go func() {
		m.logger.Info("shutting down previous processGeneration", "processGeneration", genIdentifier-1)
		if shutdownErr := previousGeneration.shutdown(syscall.SIGTERM, m.conf.ShutdownTimeout); shutdownErr != nil {
			m.logger.Warn("failed to shutdown previous processGeneration", "processGeneration", genIdentifier-1, "error", shutdownErr)
			return
		}
		m.logger.Info("previous processGeneration has been shut down", "processGeneration", genIdentifier-1)

		m.generationsMutex.Lock()
		defer m.generationsMutex.Unlock()
		delete(m.generations, genIdentifier-1)
	}()

	return nil
}

func (m *manager) shutdown(signal os.Signal) error {
	m.generationsMutex.Lock()
	runningGenerations := make([]generation, 0, len(m.generations))
	for _, gen := range m.generations {
		runningGenerations = append(runningGenerations, gen)
	}
	m.generationsMutex.Unlock()

	if notifyErr := SDNotify(NotifyStateStopping); notifyErr != nil {
		m.logger.Info("failed to send ready status to systemd", "error", notifyErr)
	} else {
		m.logger.Debug("sent stopping status to systemd")
	}

	m.logger.Info("shutting down remaining generations...", "count", len(runningGenerations))
	wg := &sync.WaitGroup{}
	wg.Add(len(runningGenerations))

	for _, gen := range runningGenerations {
		go func() {
			defer wg.Done()
			_ = gen.shutdown(signal, m.conf.ShutdownTimeout)
			m.logger.Info("processGeneration shutdown complete", "processGeneration", gen.id())
		}()
	}

	go func() {
		wg.Wait()
		m.shutdownComplete()
	}()
	return nil
}

func (m *manager) Shutdown() error {
	return m.shutdown(syscall.SIGTERM)
}

func (m *manager) Stop() error {
	return m.shutdown(syscall.SIGKILL)
}
