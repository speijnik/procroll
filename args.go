package procoll

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type processContext struct {
	PID          int
	Generation   uint64
	NotifySocket string
}

func processArgs(generation uint64, notifySocket string, argTemplates []*template.Template) ([]string, error) {
	ctx := &processContext{
		Generation:   generation,
		PID:          os.Getpid(),
		NotifySocket: notifySocket,
	}

	result := make([]string, len(argTemplates))
	for i, tpl := range argTemplates {
		buf := new(bytes.Buffer)
		if err := tpl.Execute(buf, ctx); err != nil {
			return nil, fmt.Errorf("failed to execute arg %d template: %v", i, err)
		}
		result[i] = buf.String()
	}

	return result, nil
}
