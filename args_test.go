package procoll

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessArgs(t *testing.T) {
	t.Run("Valid templates", func(t *testing.T) {
		// Create some valid templates
		tpl1, err := template.New("tpl1").Parse("arg1-{{.Generation}}")
		require.NoError(t, err)

		tpl2, err := template.New("tpl2").Parse("arg2-{{.PID}}")
		require.NoError(t, err)

		tpl3, err := template.New("tpl3").Parse("arg3-{{.NotifySocket}}")
		require.NoError(t, err)

		// Process the templates
		args, err := processArgs(42, "/tmp/notify.sock", []*template.Template{tpl1, tpl2, tpl3})

		// Verify the results
		require.NoError(t, err)
		require.Len(t, args, 3)
		assert.Equal(t, "arg1-42", args[0])
		assert.Contains(t, args[1], "arg2-") // PID will vary, so just check the prefix
		assert.Equal(t, "arg3-/tmp/notify.sock", args[2])
	})

	t.Run("Empty template slice", func(t *testing.T) {
		// Process an empty template slice
		args, err := processArgs(42, "/tmp/notify.sock", []*template.Template{})

		// Verify the results
		require.NoError(t, err)
		assert.Empty(t, args)
	})

	t.Run("Template execution error", func(t *testing.T) {
		// Create a template that will fail to execute
		tpl, err := template.New("tpl").Parse("arg-{{.NonExistentField}}")
		require.NoError(t, err)

		// Process the template
		args, err := processArgs(42, "/tmp/notify.sock", []*template.Template{tpl})

		// Verify the results
		assert.Error(t, err)
		assert.Nil(t, args)
		assert.Contains(t, err.Error(), "failed to execute arg 0 template")
	})

	t.Run("Multiple templates with one failing", func(t *testing.T) {
		// Create some valid templates and one that will fail
		tpl1, err := template.New("tpl1").Parse("arg1-{{.Generation}}")
		require.NoError(t, err)

		tpl2, err := template.New("tpl2").Parse("arg2-{{.NonExistentField}}") // This will fail
		require.NoError(t, err)

		tpl3, err := template.New("tpl3").Parse("arg3-{{.NotifySocket}}")
		require.NoError(t, err)

		// Process the templates
		args, err := processArgs(42, "/tmp/notify.sock", []*template.Template{tpl1, tpl2, tpl3})

		// Verify the results
		assert.Error(t, err)
		assert.Nil(t, args)
		assert.Contains(t, err.Error(), "failed to execute arg 1 template")
	})

	t.Run("Template with all context fields", func(t *testing.T) {
		// Create a template that uses all context fields
		tpl, err := template.New("tpl").Parse("gen={{.Generation}} pid={{.PID}} socket={{.NotifySocket}}")
		require.NoError(t, err)

		// Process the template
		args, err := processArgs(42, "/tmp/notify.sock", []*template.Template{tpl})

		// Verify the results
		require.NoError(t, err)
		require.Len(t, args, 1)
		assert.Contains(t, args[0], "gen=42")
		assert.Contains(t, args[0], "pid=")
		assert.Contains(t, args[0], "socket=/tmp/notify.sock")
	})
}
