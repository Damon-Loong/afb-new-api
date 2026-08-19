package deepseek

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLResponsesUsesNativeEndpoint(t *testing.T) {
	url, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		ChannelBaseUrl: "https://api.deepseek.com",
		RelayMode:      constant.RelayModeResponses,
	})

	require.NoError(t, err)
	require.Equal(t, "https://api.deepseek.com/v1/responses", url)
}

func TestGetRequestURLChatCompletionsIsUnchanged(t *testing.T) {
	url, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		ChannelBaseUrl: "https://api.deepseek.com",
		RelayMode:      constant.RelayModeChatCompletions,
	})

	require.NoError(t, err)
	require.Equal(t, "https://api.deepseek.com/v1/chat/completions", url)
}

func TestGetRequestURLFIMCompletionsIsUnchanged(t *testing.T) {
	url, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		ChannelBaseUrl: "https://api.deepseek.com",
		RelayMode:      constant.RelayModeCompletions,
	})

	require.NoError(t, err)
	require.Equal(t, "https://api.deepseek.com/beta/completions", url)
}

func TestConvertOpenAIResponsesRequestPassesThrough(t *testing.T) {
	request := dto.OpenAIResponsesRequest{
		Model: "deepseek-v4-flash",
		Text:  json.RawMessage(`{"format":{"type":"json_schema"}}`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, request)
	require.NoError(t, err)
	require.Equal(t, request, converted)
}

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
