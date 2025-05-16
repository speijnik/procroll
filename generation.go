package procoll

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

type osProcess interface {
	Signal(sig os.Signal) error
}

type generation interface {
	spawn() error
	waitReady(timeout time.Duration) error
	shutdown(signal os.Signal, timeout time.Duration) error
	id() uint64
}

type processGeneration struct {
	args       []string
	logger     *slog.Logger
	identifier uint64
	socketPath string
	execer     execer

	process osProcess

	cmdExited    context.CancelFunc
	cmdExitedCtx context.Context

	cmdReady    context.CancelFunc
	cmdReadyCtx context.Context

	notifySocket *net.UnixConn
}

func newGeneration(logger *slog.Logger, identifier uint64, args []string, socketPath string, execer execer) generation {
	cmdReadyCtx, cmdReady := context.WithCancel(context.Background())
	cmdExitedCtx, cmdExited := context.WithCancel(context.Background())

	return &processGeneration{
		logger:       logger,
		identifier:   identifier,
		args:         args,
		socketPath:   socketPath,
		execer:       execer,
		cmdReady:     cmdReady,
		cmdReadyCtx:  cmdReadyCtx,
		cmdExited:    cmdExited,
		cmdExitedCtx: cmdExitedCtx,
	}
}

func (gen *processGeneration) id() uint64 {
	return gen.identifier
}

func (gen *processGeneration) spawn() error {
	var err error
	gen.notifySocket, err = net.ListenUnixgram("unixgram", &net.UnixAddr{
		Name: gen.socketPath,
		Net:  "unixgram",
	})
	if err != nil {
		gen.logger.Error("failed to create notify socket", "processGeneration", gen.identifier, "path", gen.socketPath, "err", err)
		return err
	}

	cmd := gen.execer.Command(gen.args[0], gen.args[1:]...)
	osEnviron := os.Environ()
	env := make([]string, 0, len(osEnviron))
	for _, envVar := range osEnviron {
		if !strings.HasPrefix(envVar, "NOTIFY_SOCKET=") {
			env = append(env, envVar)
		}
	}
	env = append(env, "NOTIFY_SOCKET="+gen.socketPath)
	cmd.SetEnv(env)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stdout)

	if err = cmd.Start(); err != nil {
		gen.notifySocket.Close()
		return err
	}
	gen.process = cmd.GetProcess()

	go func() {
		defer gen.notifySocket.Close()
		defer gen.cmdExited()
		defer gen.cmdReady()
		if err := cmd.Wait(); err != nil {
			gen.logger.Error("failed to wait for process", "processGeneration", gen.identifier, "error", err)
		}
	}()

	go func() {
		for {
			select {
			case <-gen.cmdExitedCtx.Done():
				return
			default:
			}

			gen.handleNotifySocket()
		}
	}()

	return nil
}

func (gen *processGeneration) handleNotifySocket() {
	readBuffer := make([]byte, 1024)
	gen.logger.Debug("waiting for message on notify socket", "processGeneration", gen.identifier)
	readLen, err := gen.notifySocket.Read(readBuffer)
	if err != nil && errors.Is(err, net.ErrClosed) {
		// expected behavior during shutdown
		gen.logger.Info("notify socket closed", "processGeneration", gen.identifier)
		return
	} else if err != nil {
		gen.logger.Error("failed to accept connection on notify socket", "processGeneration", gen.identifier, "error", err)
		return
	}
	readBuffer = readBuffer[:readLen]

	line := string(readBuffer)
	gen.logger.Debug("notify socket line received", "processGeneration", gen.identifier, "line", line)

	if line == NotifyStateReady {
		gen.logger.Info("processGeneration signaled readiness", "processGeneration", gen.identifier)
		gen.cmdReady()
	} else if strings.HasPrefix(line, NotifyStateStatusPrefix) {
		if err := SDNotify(line); err != nil {
			gen.logger.Warn("failed to forward status notification", "processGeneration", gen.identifier, "status", line[len(NotifyStateStatusPrefix):], "error", err)
		}
	}
}

func (gen *processGeneration) waitReady(timeout time.Duration) error {
	select {
	case <-time.After(timeout):
		return errors.New("timed out waiting for readiness")
	case <-gen.cmdReadyCtx.Done():
		// fall-through
	}

	return nil
}

func (gen *processGeneration) shutdown(signal os.Signal, timeout time.Duration) error {
	err := gen.process.Signal(signal)
	if err != nil {
		return err
	}

	select {
	case <-time.After(timeout):
		gen.logger.Warn("failed to shut down within timeout, killing processGeneration", "processGeneration", gen.identifier, "timeout", timeout)
		if err := gen.process.Signal(syscall.SIGKILL); err != nil {
			return err
		}
		<-gen.cmdExitedCtx.Done()

	case <-gen.cmdExitedCtx.Done():
		// fall-through
	}

	return nil
}
