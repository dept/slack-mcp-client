package slackbot

import (
	"context"
	"strings"

	"github.com/tmc/langchaingo/callbacks"
)

type sendMessageFunc func(message string)

type agentCallbackHandler struct {
	callbacks.SimpleHandler
	sendMessage               sendMessageFunc
	suppressIntermediateSteps bool
}

// isIntermediateStep returns true when the text looks like an agent reasoning step
// rather than a final user-facing response.
func isIntermediateStep(text string) bool {
	text = strings.TrimSpace(text)
	if strings.Contains(text, "Thought: Do I need to use a tool? Yes") {
		return true
	}
	if strings.Contains(text, "Action:") && strings.Contains(text, "Action Input:") {
		return true
	}
	return false
}

// cleanIntermediateStep removes verbose technical details and labels, extracting only
// the justification text so users understand the agent's reasoning without internal labels.
func cleanIntermediateStep(text string) string {
	var result []string
	lines := strings.Split(strings.TrimSpace(text), "\n")
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Extract only the text after Justification:, skip Thought lines entirely
		if strings.HasPrefix(trimmed, "Justification:") {
			justificationText := strings.TrimPrefix(trimmed, "Justification:")
			justificationText = strings.TrimSpace(justificationText)
			if justificationText != "" {
				result = append(result, justificationText)
			}
		} else if strings.HasPrefix(trimmed, "Observation:") {
			observationText := strings.TrimPrefix(trimmed, "Observation:")
			observationText = strings.TrimSpace(observationText)
			if observationText != "" {
				result = append(result, observationText)
			}
		}
	}
	
	return strings.Join(result, "\n")
}

// extractFinalAnswer extracts the AI response from a chain-end output that may
// include "Thought: Do I need to use a tool? No\nAI: <answer>" framing.
func extractFinalAnswer(text string) string {
	const aiPrefix = "AI:"
	if idx := strings.LastIndex(text, aiPrefix); idx != -1 {
		answer := strings.TrimSpace(text[idx+len(aiPrefix):])
		if answer != "" {
			return answer
		}
	}
	return text
}

func (handler *agentCallbackHandler) HandleChainEnd(_ context.Context, outputs map[string]any) {
	text, ok := outputs["text"]
	if !ok {
		return
	}
	textStr, ok := text.(string)
	if !ok {
		return
	}

	if handler.suppressIntermediateSteps {
		if isIntermediateStep(textStr) {
			// Clean intermediate steps to keep Justification but remove Action/ActionInput
			cleanedText := cleanIntermediateStep(textStr)
			if cleanedText != "" {
				handler.sendMessage(cleanedText)
			}
			return
		}
		textStr = "✅ " + extractFinalAnswer(textStr)
	}

	handler.sendMessage(textStr)
}
