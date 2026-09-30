package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestBuildChannelTestCompletionTokens(t *testing.T) {
	for _, name := range []string{"gpt-6-sol", "gpt-6-astra", "gpt-5.5", "o3", "gpt-4o"} {
		for _, endpoint := range []string{"", string(constant.EndpointTypeOpenAI)} {
			for _, stream := range []bool{false, true} {
				req := buildTestRequest(name, endpoint, &model.Channel{}, stream).(*dto.GeneralOpenAIRequest)
				if name == "gpt-4o" {
					if req.MaxTokens == nil || req.MaxCompletionTokens != nil {
						t.Fatal("legacy test changed")
					}
				} else if req.MaxTokens != nil || req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 16 {
					t.Fatalf("wrong test limit for %s endpoint %s stream %v", name, endpoint, stream)
				}
			}
		}
	}
}
