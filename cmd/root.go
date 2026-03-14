package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "devdoctor",
	Short: "AI-powered DevOps health checker",
	Long: `
DevDoctor scans your project's infrastructure files —
Dockerfiles, docker-compose, and Kubernetes YAMLs —
and uses AI to suggest fixes for issues found.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}