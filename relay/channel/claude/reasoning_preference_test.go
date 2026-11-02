package claude

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Exercise the same conversion and preference application used by ResponsesHelper.
func TestOPCReasoningPreferencesThroughClaudeConversion(t *testing.T) {
	for _, model := range []string{
		"claude-fable-5", "claude-fable-5-1", "claude-opus-5-5", "claude-sonnet-5-5",
		"claude-opus-5", "claude-sonnet-5", "claude-haiku-5-5",
		"claude-opus-4-8", "claude-opus-4-7", "claude-opus-4-6", "claude-sonnet-4-6",
		"claude-sonnet-4-5", "claude-haiku-4-5", "claude-3-7-sonnet-20250219",
	} {
		for _, preference := range []string{"off", "low", "medium", "high"} {
			t.Run(model+"/"+preference, func(t *testing.T) {
				maxTokens := uint(8192)
				temperature, topP := 0.3, 0.9
				converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{
					Model: model, ReasoningPreference: &preference,
					Reasoning:       &dto.Reasoning{Effort: "minimal"},
					MaxOutputTokens: &maxTokens, Temperature: &temperature, TopP: &topP,
					Input: json.RawMessage(`[{"role":"user","content":"hello"}]`),
				})
				require.NoError(t, err)
				req := converted.(*dto.ClaudeRequest)
				effort, err := helper.ApplyClaudeReasoningPreference(req, preference)
				require.NoError(t, err)
				require.Len(t, req.Messages, 1)
				require.Equal(t, model, req.Model)
				require.Equal(t, uint(8192), *req.MaxTokens)
				require.NotNil(t, req.Thinking)

				alwaysOn := model == "claude-fable-5" || model == "claude-fable-5-1" || model == "claude-opus-5-5" || model == "claude-sonnet-5-5"
				legacy := model == "claude-sonnet-4-5" || model == "claude-haiku-4-5" || model == "claude-3-7-sonnet-20250219"
				if preference == "off" && !alwaysOn {
					require.Equal(t, "disabled", req.Thinking.Type)
					require.Equal(t, "none", effort)
					require.Nil(t, req.Thinking.BudgetTokens)
				} else if legacy {
					require.Equal(t, "enabled", req.Thinking.Type)
					require.Equal(t, map[string]int{"low": 1280, "medium": 2048, "high": 4096}[preference], *req.Thinking.BudgetTokens)
					require.Empty(t, req.OutputConfig)
				} else {
					want := preference
					if want == "off" {
						want = "low"
					}
					require.Equal(t, "adaptive", req.Thinking.Type)
					require.Equal(t, want, gjson.GetBytes(req.OutputConfig, "effort").String())
					require.Nil(t, req.Thinking.BudgetTokens)
					require.Nil(t, req.Temperature)
					require.Nil(t, req.TopP)
				}
				body, err := json.Marshal(req)
				require.NoError(t, err)
				require.False(t, gjson.GetBytes(body, "reasoning_preference").Exists())
				require.False(t, gjson.GetBytes(body, "reasoning").Exists())
			})
		}
	}
}

func TestOPCLegacyClaudeThinkingBudgetFitsOutputLimit(t *testing.T) {
	maxTokens := uint(2048)
	req := &dto.ClaudeRequest{Model: "claude-sonnet-4-5", MaxTokens: &maxTokens}
	_, err := helper.ApplyClaudeReasoningPreference(req, "high")
	require.NoError(t, err)
	require.Equal(t, 2047, *req.Thinking.BudgetTokens)
	require.Equal(t, uint(2048), *req.MaxTokens)

	maxTokens = 1024
	_, err = helper.ApplyClaudeReasoningPreference(req, "low")
	require.ErrorContains(t, err, "max_output_tokens")
}

func TestClaudeWithoutOPCPreferenceKeepsExistingConversion(t *testing.T) {
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{
		Model: "claude-sonnet-4-5", Reasoning: &dto.Reasoning{Effort: "low"},
		Input: json.RawMessage(`[{"role":"user","content":"hello"}]`),
	})
	require.NoError(t, err)
	req := converted.(*dto.ClaudeRequest)
	require.Equal(t, "enabled", req.Thinking.Type)
	require.Equal(t, 1280, *req.Thinking.BudgetTokens)
	require.Empty(t, req.OutputConfig)
}
