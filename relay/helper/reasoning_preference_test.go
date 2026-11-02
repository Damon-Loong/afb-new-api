package helper

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesReasoningPreferences(t *testing.T) {
	// Expected upstream efforts for OPC's off / low / medium / high options.
	cases := []struct {
		model string
		want  [4]string
	}{
		{"gpt-6.1-sol", [4]string{"low", "low", "medium", "high"}},
		{"openai/gpt-6.1-sol", [4]string{"low", "low", "medium", "high"}},
		{"gpt-6-astra", [4]string{"low", "low", "medium", "high"}},
		{"gpt-6.1-terra", [4]string{"none", "low", "medium", "high"}},
		{"gpt-5.6-sol", [4]string{"none", "low", "medium", "high"}},
		{"gpt-5.5", [4]string{"none", "low", "medium", "high"}},
		{"gpt-5", [4]string{"minimal", "low", "medium", "high"}},
		{"gpt-5-mini", [4]string{"minimal", "low", "medium", "high"}},
		{"deepseek-flash", [4]string{"none", "low", "high", "high"}},
		{"deepseek-v4-pro", [4]string{"none", "low", "high", "high"}},
		{"doubao-seed-2-0-pro-260215", [4]string{"minimal", "low", "medium", "high"}},
		{"doubao-seed-2-1-pro-260915", [4]string{"minimal", "low", "high", "high"}},
		{"claude-fable-5-1", [4]string{"low", "low", "medium", "high"}},
		{"claude-opus-5-5", [4]string{"low", "low", "medium", "high"}},
		{"claude-sonnet-5-5", [4]string{"low", "low", "medium", "high"}},
		{"claude-opus-4-7", [4]string{"none", "low", "medium", "high"}},
		{"claude-sonnet-4-5", [4]string{"none", "low", "medium", "high"}},
	}
	for _, tc := range cases {
		for i, preference := range []string{"off", "low", "medium", "high"} {
			t.Run(tc.model+"/"+preference, func(t *testing.T) {
				// Mimic the decoded request, including OPC's legacy effort fallback.
				request := dto.OpenAIResponsesRequest{
					Model: tc.model, ReasoningPreference: &preference,
					Reasoning: &dto.Reasoning{Effort: "minimal", Summary: "auto"},
					Input:     json.RawMessage(`[{"role":"user","content":"hello"}]`),
				}
				body, err := json.Marshal(request)
				require.NoError(t, err)
				body, err = ApplyResponsesReasoningPreference(body, tc.model)
				require.NoError(t, err)
				require.Equal(t, tc.want[i], gjson.GetBytes(body, "reasoning.effort").String())
				require.Equal(t, "auto", gjson.GetBytes(body, "reasoning.summary").String())
				require.Equal(t, "hello", gjson.GetBytes(body, "input.0.content").String())
				require.False(t, gjson.GetBytes(body, "reasoning_preference").Exists())
			})
		}
	}
}

func TestResponsesReasoningPreferenceIsOptIn(t *testing.T) {
	body := []byte(`{"model":"gpt-6.1-sol","reasoning":{"effort":"max"},"custom":{"keep":true}}`)
	got, err := ApplyResponsesReasoningPreference(body, "gpt-6.1-sol")
	require.NoError(t, err)
	require.Equal(t, body, got)

	// Other providers retain their old behavior, but no private field leaks.
	got, err = ApplyResponsesReasoningPreference([]byte(`{"reasoning_preference":"off","reasoning":{"effort":"minimal"},"custom":{"keep":true}}`), "unknown-model")
	require.NoError(t, err)
	require.JSONEq(t, `{"reasoning":{"effort":"minimal"},"custom":{"keep":true}}`, string(got))
}

func TestResponsesReasoningPreferenceUsesRoutedModelAndPreservesRawFields(t *testing.T) {
	// Auto-routing or model mapping has already selected the actual upstream model.
	body, err := ApplyResponsesReasoningPreference([]byte(`{"model":"auto","reasoning_preference":"off","extra_upstream_field":{"keep":42}}`), "gpt-6.1-sol")
	require.NoError(t, err)
	require.Equal(t, "low", gjson.GetBytes(body, "reasoning.effort").String())
	require.Equal(t, int64(42), gjson.GetBytes(body, "extra_upstream_field.keep").Int())
	require.False(t, gjson.GetBytes(body, "reasoning_preference").Exists())
}

func TestResponsesReasoningPreferenceRejectsInvalidChoices(t *testing.T) {
	for _, body := range []string{
		`{"reasoning_preference":"max"}`, `{"reasoning_preference":""}`,
		`{"reasoning_preference":true}`, `{"reasoning_preference":null}`,
	} {
		_, err := ApplyResponsesReasoningPreference([]byte(body), "gpt-6.1-sol")
		require.Error(t, err)
	}
}
