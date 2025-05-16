package procoll

import (
	"fmt"
	"log/slog"
	"text/template"
)

// New initializes a new procroll process manager instance.
func New(args []string, logger *slog.Logger, conf Config) (Manager, error) {
	argTemplates := make([]*template.Template, len(args))
	for i, arg := range args {
		tmpl, err := template.New(fmt.Sprintf("arg%d", i)).Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument %d template '%s': %w", i, arg, err)
		}
		argTemplates[i] = tmpl
	}

	return &manager{
		argTemplates:  argTemplates,
		logger:        logger,
		generation:    0,
		generations:   make(map[uint64]generation, 1),
		conf:          conf,
		execer:        systemExecer{},
		newGeneration: newGeneration,
	}, nil
}
