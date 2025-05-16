package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "procroll",
	Short: "rolling process restarts",
	Long:  `procroll is an opinionated process manager that enables rolling restarts of processes.`,
}
