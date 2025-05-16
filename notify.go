package procoll

import (
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
)

const (
	// NotifyStateReady designates that the process is ready
	NotifyStateReady = "READY=1"

	// NotifyStateStopping designates that the process is stopping
	NotifyStateStopping = "STOPPING=1"

	// NotifyStateReloading designates that the process is reloading
	NotifyStateReloading = "RELOADING=1"

	// NotifyStateStatusPrefix is the prefix for an arbitrary status message
	NotifyStateStatusPrefix = "STATUS="

	// NotifyStateMontonicUsecPrefix is the prefix for the monotonic usec  status
	NotifyStateMontonicUsecPrefix = "MONOTONIC_USEC="
)

// SDNotify sends a systemd notification with the given state
func SDNotify(state string) error {
	socketPath := os.Getenv("NOTIFY_SOCKET")
	if socketPath == "" {
		// silently do nothing
		return nil
	}

	conn, err := net.Dial("unixgram", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state + "\n"))
	if err != nil {
		return err
	}
	return nil
}

// SDNotifyReloading sents a systemd "reloading" notification
func SDNotifyReloading() error {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return err
	}
	nsecs := ts.Sec*1e09 + ts.Nsec

	return SDNotify(NotifyStateReloading + "\n" + NotifyStateMontonicUsecPrefix + fmt.Sprintf("%d", nsecs/1000))
}
