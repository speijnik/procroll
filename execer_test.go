package procoll

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemExecer_Command(t *testing.T) {
	execer := systemExecer{}

	// Test creating a command
	cmd := execer.Command("echo", "hello")

	// Verify it's the correct type
	assert.IsType(t, &systemCommand{}, cmd)

	// Verify the underlying command is set correctly
	systemCmd := cmd.(*systemCommand)
	// The Path might be resolved to the full path (e.g., /bin/echo)
	assert.Contains(t, systemCmd.Cmd.Path, "echo")
	assert.Equal(t, []string{"echo", "hello"}, systemCmd.Cmd.Args)
}

func TestSystemCommand_SetEnv(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	systemCmd := &systemCommand{Cmd: cmd}

	// Set environment variables
	env := []string{"FOO=bar", "BAZ=qux"}
	systemCmd.SetEnv(env)

	// Verify environment variables are set correctly
	assert.Equal(t, env, systemCmd.Cmd.Env)
}

func TestSystemCommand_SetStdout(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	systemCmd := &systemCommand{Cmd: cmd}

	// Create a buffer for stdout
	var stdout bytes.Buffer

	// Set stdout
	systemCmd.SetStdout(&stdout)

	// Verify stdout is set correctly
	assert.Equal(t, &stdout, systemCmd.Cmd.Stdout)
}

func TestSystemCommand_SetStderr(t *testing.T) {
	cmd := exec.Command("echo", "hello")
	systemCmd := &systemCommand{Cmd: cmd}

	// Create a buffer for stderr
	var stderr bytes.Buffer

	// Set stderr
	systemCmd.SetStderr(&stderr)

	// Verify stderr is set correctly
	assert.Equal(t, &stderr, systemCmd.Cmd.Stderr)
}

func TestSystemCommand_GetProcess(t *testing.T) {
	// This test is a bit tricky because we need a running process
	// We'll create a command but not start it, so Process will be nil
	cmd := exec.Command("echo", "hello")
	systemCmd := &systemCommand{Cmd: cmd}

	// Before starting, Process should be nil
	assert.Nil(t, systemCmd.GetProcess())

	// Start the command
	err := systemCmd.Start()
	require.NoError(t, err)

	// After starting, Process should not be nil
	assert.NotNil(t, systemCmd.GetProcess())

	// Wait for the command to finish
	err = systemCmd.Wait()
	require.NoError(t, err)
}

func TestSystemCommand_Start(t *testing.T) {
	// Create a command that will succeed
	cmd := exec.Command("echo", "hello")
	systemCmd := &systemCommand{Cmd: cmd}

	// Start the command
	err := systemCmd.Start()
	assert.NoError(t, err)

	// Wait for the command to finish
	err = systemCmd.Wait()
	assert.NoError(t, err)

	// Create a command that will fail to start
	cmd = exec.Command("nonexistentcommand")
	systemCmd = &systemCommand{Cmd: cmd}

	// Start the command
	err = systemCmd.Start()
	assert.Error(t, err)
}

func TestSystemCommand_Wait(t *testing.T) {
	// Create a command that will succeed
	cmd := exec.Command("echo", "hello")
	systemCmd := &systemCommand{Cmd: cmd}

	// Start the command
	err := systemCmd.Start()
	require.NoError(t, err)

	// Wait for the command to finish
	err = systemCmd.Wait()
	assert.NoError(t, err)

	// Create a command that will fail
	cmd = exec.Command("sh", "-c", "exit 1")
	systemCmd = &systemCommand{Cmd: cmd}

	// Start the command
	err = systemCmd.Start()
	require.NoError(t, err)

	// Wait for the command to finish
	err = systemCmd.Wait()
	assert.Error(t, err)

	// Verify it's an ExitError with exit code 1
	exitErr, ok := err.(*exec.ExitError)
	assert.True(t, ok)
	assert.Equal(t, 1, exitErr.ExitCode())
}

func TestIntegration_SystemExecer(t *testing.T) {
	// This test verifies that the systemExecer and systemCommand work together correctly

	execer := systemExecer{}

	// Create a command
	cmd := execer.Command("echo", "hello")

	// Set up stdout capture
	var stdout bytes.Buffer
	cmd.SetStdout(&stdout)

	// Start the command
	err := cmd.Start()
	require.NoError(t, err)

	// Get the process
	process := cmd.GetProcess()
	assert.NotNil(t, process)
	assert.Greater(t, process.Pid, 0)

	// Wait for the command to finish
	err = cmd.Wait()
	assert.NoError(t, err)

	// Verify the output
	assert.Equal(t, "hello\n", stdout.String())
}
