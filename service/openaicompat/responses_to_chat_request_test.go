package openaicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func responsesRawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func TestResponsesRequestToChatCompletionsRequest_TextAndFunctionTool(t *testing.T) {
	stream := true
	maxOutputTokens := uint(1024)
	req := &dto.OpenAIResponsesRequest{
		Model:           "deepseek-chat",
		Stream:          &stream,
		MaxOutputTokens: &maxOutputTokens,
		Instructions:    responsesRawJSON(t, "Be concise"),
		Input: responsesRawJSON(t, []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "hello"},
				},
			},
		}),
		Tools: responsesRawJSON(t, []map[string]any{
			{
				"type":        "function",
				"name":        "lookup",
				"description": "Lookup info",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"q": map[string]any{"type": "string"},
					},
				},
			},
		}),
		ToolChoice:        responsesRawJSON(t, map[string]any{"type": "function", "name": "lookup"}),
		ParallelToolCalls: responsesRawJSON(t, true),
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Equal(t, "deepseek-chat", chatReq.Model)
	require.True(t, *chatReq.Stream)
	require.Equal(t, maxOutputTokens, *chatReq.MaxTokens)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Equal(t, "Be concise", chatReq.Messages[0].Content)
	require.Equal(t, "user", chatReq.Messages[1].Role)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "function", chatReq.Tools[0].Type)
	require.Equal(t, "lookup", chatReq.Tools[0].Function.Name)
	require.Equal(t, true, *chatReq.ParallelTooCalls)

	toolChoice, ok := chatReq.ToolChoice.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", toolChoice["type"])
	function, ok := toolChoice["function"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "lookup", function["name"])
}

func TestResponsesRequestToChatCompletionsRequest_RejectsBuiltInTool(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: responsesRawJSON(t, []map[string]any{
			{"role": "user", "content": "hello"},
		}),
		Tools: responsesRawJSON(t, []map[string]any{
			{"type": "web_search_preview"},
		}),
	}

	_, err := ResponsesRequestToChatCompletionsRequest(req)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "web_search_preview"))
}
