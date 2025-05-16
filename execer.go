package procoll

import (
	"io"
	"os"
	"os/exec"
)

type command interface {
	SetEnv(env []string)
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	Start() error
	GetProcess() *os.Process
	Wait() error
}

type execer interface {
	Command(name string, arg ...string) command
}

type systemCommand struct {
	*exec.Cmd
}

func (s *systemCommand) SetEnv(env []string) {
	s.Cmd.Env = env
}

func (s *systemCommand) SetStdout(w io.Writer) {
	s.Cmd.Stdout = w
}

func (s *systemCommand) SetStderr(w io.Writer) {
	s.Cmd.Stderr = w
}

func (s *systemCommand) GetProcess() *os.Process {
	return s.Cmd.Process
}

type systemExecer struct{}

func (systemExecer) Command(name string, arg ...string) command {
	return &systemCommand{
		Cmd: exec.Command(name, arg...),
	}
}
