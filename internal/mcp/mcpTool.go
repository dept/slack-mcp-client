package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tuannvm/slack-mcp-client/internal/monitoring"
)

type ToolInfo struct {
	ServerName       string
	ToolName         string
	ToolDescription  string
	InputSchema      map[string]interface{}
	InputSchemaBytes []byte
	Client           MCPClientInterface
}

func (t *ToolInfo) Name() string {
	return t.ToolName
}

func (t *ToolInfo) Description() string {
	if t.InputSchemaBytes == nil {
		t.InputSchemaBytes, _ = json.Marshal(t.InputSchema)
	}
	return t.ToolDescription + "\n The input schema is: " + string(t.InputSchemaBytes)
}

func (t *ToolInfo) Call(ctx context.Context, input string) (string, error) {
	var args map[string]interface{}
	err := json.Unmarshal([]byte(input), &args)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %w", err)
	}

	isError := "false"
	defer func() {
		monitoring.ToolInvocations.With(prometheus.Labels{
			monitoring.MetricLabelTool:   t.ToolName,
			monitoring.MetricLabelServer: t.ServerName,
			monitoring.MetricLabelError:  isError,
		}).Inc()
	}()

	res, err := t.Client.CallTool(ctx, t.Name(), args)
	if err != nil {
		isError = "true"
		// Return the error as an observation string rather than a Go error so the
		// LangChain agent can handle it gracefully and inform the user, instead of
		// the executor terminating the agent and surfacing a raw error stack trace.
		return fmt.Sprintf("Tool call failed: %v", err), nil
	}

	return res, nil
}
