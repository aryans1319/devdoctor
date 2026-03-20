package analyzer

import (
	"github.com/aryans1319/devdoctor/ai"
	"github.com/aryans1319/devdoctor/models"
)

// enrichWithAI is called by the pipeline after all analyzers run.
// It delegates to the ai package which handles all Gemini logic.
func enrichWithAI(result models.ScanResult) models.ScanResult {
	client := ai.NewClient()
	return client.EnrichWithAI(result)
}