package formatter

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/aryans1319/devdoctor/models"
)

// Colors
var (
	red    = color.New(color.FgRed, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	cyan   = color.New(color.FgCyan, color.Bold)
	green  = color.New(color.FgGreen, color.Bold)
	white  = color.New(color.FgWhite, color.Bold)
	gray   = color.New(color.FgWhite)
)

func PrintResults(result models.ScanResult) {
	printHeader()

	if len(result.Results) == 0 {
		green.Println("✅ No Dockerfile or docker-compose.yml found in this project.")
		return
	}

	// Print each file result
	for _, fileResult := range result.Results {
		printFileResult(fileResult)
	}

	// Print overall summary
	printSummary(result)
}

func printHeader() {
	fmt.Println()
	cyan.Println("╔══════════════════════════════════════╗")
	cyan.Println("║         🩺 DevDoctor Report           ║")
	cyan.Println("╚══════════════════════════════════════╝")
	fmt.Println()
}

func printFileResult(result models.FileResult) {
	// File header
	white.Printf("📄 %s", result.FilePath)
	fmt.Printf("  (%s)\n", result.FileType)
	printScoreLine(result.Score)
	fmt.Println(strings.Repeat("─", 50))

	if len(result.Issues) == 0 {
		green.Println("  ✅ No issues found")
		fmt.Println()
		return
	}

	// Print each issue
	for _, issue := range result.Issues {
		printIssue(issue)
	}

	// Print AI summary for this file
	if result.AISummary != "" {
		fmt.Println()
		cyan.Print("  🤖 AI Summary: ")
		gray.Println(result.AISummary)
	}

	fmt.Println()
}

func printIssue(issue models.Issue) {
	fmt.Println()

	// Severity icon + message
	switch issue.Severity {
	case models.SeverityError:
		red.Print("  ❌ ERROR")
	case models.SeverityWarning:
		yellow.Print("  ⚠️  WARNING")
	case models.SeverityInfo:
		cyan.Print("  ℹ️  INFO")
	}

	if issue.Line > 0 {
		gray.Printf("  (line %d)", issue.Line)
	}
	fmt.Println()

	// Rule name
	gray.Printf("     Rule    : %s\n", issue.Rule)

	// Message
	white.Printf("     Issue   : ")
	fmt.Println(issue.Message)

	// AI suggestion
	if issue.Suggestion != "" {
		green.Printf("     Fix     : ")
		fmt.Println(issue.Suggestion)
	}
}

func printScoreLine(score int) {
	fmt.Print("  Score: ")
	switch {
	case score >= 80:
		green.Printf("%d/100\n", score)
	case score >= 50:
		yellow.Printf("%d/100\n", score)
	default:
		red.Printf("%d/100\n", score)
	}
}

func printSummary(result models.ScanResult) {
	fmt.Println(strings.Repeat("═", 50))
	white.Println("📊 Overall Summary")
	fmt.Println(strings.Repeat("═", 50))

	fmt.Printf("  Files scanned : %d\n", len(result.Results))
	fmt.Printf("  Total issues  : %d\n", result.TotalIssues)

	fmt.Print("  Overall score : ")
	switch {
	case result.OverallScore >= 80:
		green.Printf("%d/100\n", result.OverallScore)
	case result.OverallScore >= 50:
		yellow.Printf("%d/100\n", result.OverallScore)
	default:
		red.Printf("%d/100\n", result.OverallScore)
	}

	fmt.Println()

	// Health verdict
	switch {
	case result.OverallScore >= 80:
		green.Println("  ✅ Your project looks healthy!")
	case result.OverallScore >= 50:
		yellow.Println("  ⚠️  Some issues need attention.")
	default:
		red.Println("  ❌ Critical issues found. Fix these before deploying.")
	}

	fmt.Println()
}