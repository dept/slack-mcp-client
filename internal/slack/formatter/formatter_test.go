package formatter

import (
	"encoding/json"
	"testing"
)

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Text with quoted strings",
			input:    "Created on \"2020-11-17T05:07:52Z\" or \"2020-11-17T05:07:54Z\"",
			expected: "Created on `2020-11-17T05:07:52Z` or `2020-11-17T05:07:54Z`",
		},
		// Add more test cases as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("FormatMarkdown() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertQuotedStringsToCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No quoted strings",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Single quoted string",
			input:    "Namespace \"kube-system\" is a system namespace",
			expected: "Namespace `kube-system` is a system namespace",
		},
		{
			name:     "Multiple quoted strings",
			input:    "All of these were created on \"2020-11-17T05:07:52Z\" or \"2020-11-17T05:07:54Z\". Among them, \"kube-node-lease\", \"kube-public\", and \"kube-system\" share the exact same creation timestamp: \"2020-11-17T05:07:52Z\", making them the oldest namespaces in your cluster.",
			expected: "All of these were created on `2020-11-17T05:07:52Z` or `2020-11-17T05:07:54Z`. Among them, `kube-node-lease`, `kube-public`, and `kube-system` share the exact same creation timestamp: `2020-11-17T05:07:52Z`, making them the oldest namespaces in your cluster.",
		},
		{
			name:     "Escaped quotes",
			input:    "The command \"echo \\\"Hello\\\"\" prints Hello",
			expected: "The command `echo \\\"Hello\\\"` prints Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertQuotedStringsToCode(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertQuotedStringsToCode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCreateBlockMessage(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		blockOptions BlockOptions
		expectBlocks bool
	}{
		{
			name: "Simple block with header",
			text: "This is a test message",
			blockOptions: BlockOptions{
				HeaderText: "Test Header",
			},
			expectBlocks: true,
		},
		{
			name: "Block with fields",
			text: "This is a test message with fields",
			blockOptions: BlockOptions{
				HeaderText: "Test Header",
				Fields: []Field{
					{Title: "Status", Value: "Success"},
					{Title: "Duration", Value: "5m 32s"},
				},
			},
			expectBlocks: true,
		},
		{
			name: "Block with actions",
			text: "This is a test message with actions",
			blockOptions: BlockOptions{
				HeaderText: "Test Header",
				Actions: []Action{
					{Text: "View Details", URL: "http://example.com"},
				},
			},
			expectBlocks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateBlockMessage(tt.text, tt.blockOptions)

			// Verify it's valid JSON
			var parsed map[string]interface{}
			err := json.Unmarshal([]byte(result), &parsed)
			if err != nil {
				t.Errorf("CreateBlockMessage() produced invalid JSON: %v", err)
				return
			}

			// Check if blocks exist
			blocks, ok := parsed["blocks"]
			if tt.expectBlocks && (!ok || blocks == nil) {
				t.Errorf("CreateBlockMessage() did not produce blocks")
			}
		})
	}
}

func TestDetectMessageType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MessageType
	}{
		{
			name:     "Plain text",
			input:    "Hello world",
			expected: PlainText,
		},
		{
			name:     "Markdown text",
			input:    "Hello *bold* _italic_ world",
			expected: MarkdownText,
		},
		{
			name: "JSON Block",
			input: `{
				"text": "Hello world",
				"blocks": [
					{
						"type": "section",
						"text": {
							"type": "mrkdwn",
							"text": "Hello world"
						}
					}
				]
			}`,
			expected: JSONBlock,
		},
		{
			name: "Structured data",
			input: `Status: Success
Duration: 5m 32s
Result: Passed`,
			expected: StructuredData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMessageType(tt.input)
			if result != tt.expected {
				t.Errorf("DetectMessageType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractStructuredData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "Simple key-value pairs",
			input: `Status: Success
Duration: 5m 32s
Result: Passed`,
			expected: map[string]string{
				"Status":   "Success",
				"Duration": "5m 32s",
				"Result":   "Passed",
			},
		},
		{
			name:     "No structured data",
			input:    "Hello world",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractStructuredData(tt.input)

			// Check if maps have the same length
			if len(result) != len(tt.expected) {
				t.Errorf("ExtractStructuredData() returned map with length %d, want %d", len(result), len(tt.expected))
				return
			}

			// Check if all expected keys are present with correct values
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("ExtractStructuredData() for key %s = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestFormatStructuredData(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectBlocks bool
	}{
		{
			name: "Structured data",
			input: `Status: Success
Duration: 5m 32s
Result: Passed`,
			expectBlocks: true,
		},
		{
			name:         "Non-structured data",
			input:        "Hello world",
			expectBlocks: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStructuredData(tt.input)

			// Check if it's JSON for structured data
			if tt.expectBlocks {
				var parsed map[string]interface{}
				err := json.Unmarshal([]byte(result), &parsed)
				if err != nil {
					t.Errorf("FormatStructuredData() produced invalid JSON: %v", err)
					return
				}

				// Check if blocks exist
				blocks, ok := parsed["blocks"]
				if !ok || blocks == nil {
					t.Errorf("FormatStructuredData() did not produce blocks")
				}
			} else {
				// For non-structured data, it should return the original content
				if result != tt.input {
					t.Errorf("FormatStructuredData() = %v, want %v", result, tt.input)
				}
			}
		})
	}
}

// TestDetectMessageTypeSlackMrkdwn verifies that mrkdwn content with numbered lists
// is classified as MarkdownText (not StructuredData). If classified as StructuredData
// the content gets routed through ExtractStructuredData which uses key-value regex
// on the entries and reassembles them in random map-iteration order.
func TestDetectMessageTypeSlackMrkdwn(t *testing.T) {
	mrkdwn := "*You have booked 3.00 hours on Wednesday, April 8, 2026:*\n" +
		"1. 1.50h on Personalisatie sprints (Budget / Activity) \u2014 \"meeting with Chris\" [rebook-> budgetId:606706, activityId:557 | entry:123]\n" +
		"2. 1.50h on New website PGGM (Budget / Activity) \u2014 \"Daily work\" [rebook-> budgetId:644094, activityId:2260 | entry:456]"

	got := DetectMessageType(mrkdwn)
	if got != MarkdownText {
		t.Errorf("DetectMessageType() = %v, want MarkdownText (%v); mrkdwn with numbered list must not be treated as StructuredData", got, MarkdownText)
	}
}

// TestFormatMarkdownPreservesQuotesInMrkdwn verifies that description quotes inside
// Slack mrkdwn content are NOT converted to backtick code-spans.
func TestFormatMarkdownPreservesQuotesInMrkdwn(t *testing.T) {
	input := "*You have booked 3.00 hours:*\n1. 1.50h on Project \u2014 \"meeting with Chris\""
	result := FormatMarkdown(input)
	if result != input {
		t.Errorf("FormatMarkdown() modified mrkdwn content:\ngot:  %v\nwant: %v", result, input)
	}
}
