package slackbot

import (
	"context"
	"testing"
)

func TestHandleChainEndSuppressesIntermediateSteps(t *testing.T) {
	var finalMessages []string
	var intermediateMessages []string

	handler := &agentCallbackHandler{
		sendMessage: func(message string) {
			finalMessages = append(finalMessages, message)
		},
		sendIntermediateMessage: func(message string) {
			intermediateMessages = append(intermediateMessages, message)
		},
		suppressIntermediateSteps: true,
	}

	handler.HandleChainEnd(context.Background(), map[string]any{
		"text": "Thought: Do I need to use a tool? Yes\nJustification: I should inspect the budget\nAction: search_budget\nAction Input: ...",
	})

	if len(finalMessages) != 0 {
		t.Fatalf("expected no final messages, got %v", finalMessages)
	}
	if len(intermediateMessages) != 0 {
		t.Fatalf("expected no intermediate messages, got %v", intermediateMessages)
	}
}

func TestHandleChainEndExtractsFinalAnswerWhenSuppressed(t *testing.T) {
	var finalMessages []string

	handler := &agentCallbackHandler{
		sendMessage: func(message string) {
			finalMessages = append(finalMessages, message)
		},
		suppressIntermediateSteps: true,
	}

	handler.HandleChainEnd(context.Background(), map[string]any{
		"text": "Thought: Do I need to use a tool? No\nAI: Booking completed successfully.",
	})

	if len(finalMessages) != 1 {
		t.Fatalf("expected one final message, got %d", len(finalMessages))
	}
	if finalMessages[0] != "Booking completed successfully." {
		t.Fatalf("unexpected final message: %q", finalMessages[0])
	}
}
