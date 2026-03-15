package github

import (
	"fmt"
	"strings"
)

// formatComment builds the markdown PR comment from scan results
func formatComment(results []analyzerResult) string {
	if len(results) == 0 {
		return "## 🩺 DevDoctor Report\n\nNo infrastructure files found in this PR."
	}

	overallScore := calculateOverallScore(results)

	var sb strings.Builder

	// Header
	sb.WriteString("## 🩺 DevDoctor Report\n\n")

	// Overall score badge
	scoreEmoji := scoreEmoji(overallScore)
	sb.WriteString(fmt.Sprintf("### Overall Score: **%d/100** %s\n\n", overallScore, scoreEmoji))
	sb.WriteString(scoreBar(overallScore))
	sb.WriteString("\n\n---\n\n")

	// Per file results
	for _, result := range results {
		sb.WriteString(formatFileSection(result))
	}

	// Footer
	sb.WriteString("\n\n---\n")
	sb.WriteString("*Powered by [DevDoctor](https://github.com/aryans1319/devdoctor) — AI-powered DevOps health checker*")

	return sb.String()
}

// formatFileSection formats a single file's results
func formatFileSection(result analyzerResult) string {
	var sb strings.Builder

	// File header with score
	fileEmoji := "📄"
	sb.WriteString(fmt.Sprintf("### %s `%s` — Score: **%d/100**\n\n", fileEmoji, result.Filename, result.Score))

	if len(result.Issues) == 0 {
		sb.WriteString("✅ No issues found\n\n")
		return sb.String()
	}

	// Issues table
	sb.WriteString("| Severity | Rule | Issue |\n")
	sb.WriteString("|----------|------|-------|\n")

	for _, issue := range result.Issues {
		severityBadge := severityBadge(issue.Severity)
		location := ""
		if issue.Line > 0 {
			location = fmt.Sprintf(" *(line %d)*", issue.Line)
		}
		sb.WriteString(fmt.Sprintf("| %s | `%s` | %s%s |\n",
			severityBadge, issue.Rule, issue.Message, location))
	}

	sb.WriteString("\n")

	// AI suggestions
	hasSuggestions := false
	for _, issue := range result.Issues {
		if issue.Suggestion != "" {
			hasSuggestions = true
			break
		}
	}

	if hasSuggestions {
		sb.WriteString("<details>\n<summary>🤖 AI-powered Fix Suggestions</summary>\n\n")
		for _, issue := range result.Issues {
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("**`%s`** — %s\n\n", issue.Rule, issue.Suggestion))
			}
		}
		sb.WriteString("</details>\n\n")
	}

	// AI Summary
	if result.AISummary != "" {
		sb.WriteString(fmt.Sprintf("> 🤖 **AI Summary:** %s\n\n", result.AISummary))
	}

	return sb.String()
}

func scoreEmoji(score int) string {
	switch {
	case score >= 80:
		return "✅"
	case score >= 50:
		return "⚠️"
	default:
		return "❌"
	}
}

func scoreBar(score int) string {
	filled := score / 10
	empty := 10 - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("`%s` %d%%", bar, score)
}

func severityBadge(severity string) string {
	switch severity {
	case "ERROR":
		return "❌ Error"
	case "WARNING":
		return "⚠️ Warning"
	case "INFO":
		return "ℹ️ Info"
	default:
		return severity
	}
}