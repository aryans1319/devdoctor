package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/aryans1319/devdoctor/analyzer"
	"github.com/aryans1319/devdoctor/formatter"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a project directory for DevOps issues",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		// Check if path exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("❌ Path does not exist: %s\n", path)
			os.Exit(1)
		}

		fmt.Println("🔍 DevDoctor scanning", path, "...")
		fmt.Println()

		// Run the scan
		result, err := analyzer.ScanProject(path)
		if err != nil {
			fmt.Printf("❌ Scan failed: %s\n", err)
			os.Exit(1)
		}

		// Print results
		formatter.PrintResults(result)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}