package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/samber/lo"
)

func TestCompletionTokenCompatibility(t *testing.T) {
	for _, name := range []string{"gpt-6-sol", "gpt-6-astra", "gpt-5.5", "gpt-5.6-sol", "o3", "gpt-4o"} {
		for _, both := range []bool{false, true} {
			t.Run(name+"/both="+map[bool]string{false: "false", true: "true"}[both], func(t *testing.T) {
				req := &dto.GeneralOpenAIRequest{Model: "public-alias", MaxTokens: lo.ToPtr(uint(16))}
				if both {
					req.MaxCompletionTokens = lo.ToPtr(uint(64))
				}
				info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: name}}
				result, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, req)
				if err != nil {
					t.Fatal(err)
				}
				body, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				var wire map[string]any
				if err := json.Unmarshal(body, &wire); err != nil {
					t.Fatal(err)
				}
				if name == "gpt-4o" {
					if wire["max_tokens"] != float64(16) {
						t.Fatalf("legacy limit changed: %s", body)
					}
					return
				}
				if _, ok := wire["max_tokens"]; ok {
					t.Fatalf("legacy limit leaked: %s", body)
				}
				want := float64(16)
				if both {
					want = 64
				}
				if wire["max_completion_tokens"] != want {
					t.Fatalf("wrong completion limit: %s", body)
				}
			})
		}
	}
}
