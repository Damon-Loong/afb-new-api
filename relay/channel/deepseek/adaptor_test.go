package deepseek

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestDisableThinkingForMinimalReasoning(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:           "deepseek-chat",
		ReasoningEffort: "minimal",
	}

	disableThinkingForMinimalReasoning(request)

	require.Empty(t, request.ReasoningEffort)
	var thinking map[string]string
	require.NoError(t, json.Unmarshal(request.THINKING, &thinking))
	require.Equal(t, "disabled", thinking["type"])
}

func TestDisableThinkingForMinimalReasoningKeepsSupportedEffort(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:           "deepseek-chat",
		ReasoningEffort: "low",
	}

	disableThinkingForMinimalReasoning(request)

	require.Equal(t, "low", request.ReasoningEffort)
	require.Nil(t, request.THINKING)
}
