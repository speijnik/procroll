package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	procoll "github.com/speijnik/procroll"
)

var (
	debug          bool
	config         procoll.Config
	shutdownSignal string

	runCmd = &cobra.Command{
		Use:   "run <path/to/binary> [-- args]",
		Short: "Wrap command inside procroll process manager",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ch := make(chan os.Signal, 1)

			signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
			defer func() {
				signal.Stop(ch)
				close(ch)
			}()

			logLevel := slog.LevelInfo
			if debug {
				logLevel = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: logLevel,
			}))

			if shutdownSignalInt, err := strconv.Atoi(shutdownSignal); err == nil {
				// signal supplied as integer
				if unix.SignalName(syscall.Signal(shutdownSignalInt)) == "" {
					return fmt.Errorf("signal: invalid value supplied: %d", shutdownSignalInt)
				}
				config.ShutdownSignal = syscall.Signal(shutdownSignalInt)
			} else {
				config.ShutdownSignal = unix.SignalNum(shutdownSignal)
				if config.ShutdownSignal == unix.Signal(0) {
					return fmt.Errorf("signal: invalid value supplied: %s", shutdownSignal)
				}
			}

			m, err := procoll.New(args, logger, config)
			if err != nil {
				return err
			}

			if err := m.Start(); err != nil {
				return err
			}

			go func() {
				shuttingDown := false
				for sig := range ch {
					switch sig {
					case syscall.SIGHUP:
						if err := m.Roll(); err != nil {
							_ = m.Shutdown()
						}
					case syscall.SIGINT, syscall.SIGTERM:
						if !shuttingDown {
							shuttingDown = true
							go func() {
								_ = m.Shutdown()
							}()
						} else {
							go func() {
								_ = m.Stop()
							}()
						}

					}
				}
			}()

			if err := m.Wait(); err != nil {
				return err
			}

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")
	runCmd.PersistentFlags().StringVar(&shutdownSignal, "signal", "SIGTERM", "generation shutdown signal")
	runCmd.PersistentFlags().DurationVar(&config.ShutdownTimeout, "shutdown-timeout", time.Minute, "generation shutdown timeout")
	runCmd.PersistentFlags().DurationVar(&config.ReadyTimeout, "ready-timeout", time.Second*10, "generation ready timeout")
	runCmd.PersistentFlags().StringVar(&config.TempDir, "tempdir", os.TempDir(), "base temporary directory")
}
