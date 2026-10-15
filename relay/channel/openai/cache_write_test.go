package openai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestResponsesCacheWriteUsage(t *testing.T) {
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	for _, stream := range []bool{false, true} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		body := `{"usage":{"input_tokens":1000,"output_tokens":10,"total_tokens":1010,"input_tokens_details":{"cached_tokens":200,"cache_write_tokens":300}}}`
		if stream {
			body = "data: {\"type\":\"response.completed\",\"response\":" + body + "}\n\ndata: [DONE]\n\n"
		}
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
		var usage *dto.Usage
		var err *types.NewAPIError
		if stream {
			usage, err = OaiResponsesStreamHandler(c, &relaycommon.RelayInfo{StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-6-sol"}}, resp)
		} else {
			usage, err = OaiResponsesHandler(c, nil, resp)
		}
		if err != nil {
			t.Fatalf("stream=%v error=%v", stream, err)
		}
		if usage == nil || usage.PromptTokens != 1000 || usage.PromptTokensDetails.CachedTokens != 200 || usage.PromptTokensDetails.CachedCreationTokens != 300 || usage.ClaudeCacheCreation1hTokens != 0 {
			t.Fatalf("stream=%v usage=%+v", stream, usage)
		}
	}
}

func TestChatCacheWriteUsage(t *testing.T) {
	for _, body := range []string{
		`{"prompt_tokens_details":{"cache_write_tokens":300}}`,
		`{"input_tokens_details":{"cache_write_tokens":300}}`,
		`{"prompt_tokens_details":{"cached_creation_tokens":300,"cache_write_tokens":900}}`,
	} {
		var usage dto.Usage
		if err := json.Unmarshal([]byte(body), &usage); err != nil {
			t.Fatal(err)
		}
		applyUsagePostProcessing(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, &usage, nil)
		if usage.PromptTokensDetails.CachedCreationTokens != 300 {
			t.Fatalf("usage=%+v", usage)
		}
	}
}
