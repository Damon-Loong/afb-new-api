package volcengine

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func TestVolcengineEmbeddingUsesMultimodalURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://ark.cn-beijing.volces.com",
			UpstreamModelName: "doubao-embedding",
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	want := "https://ark.cn-beijing.volces.com/api/v3/embeddings/multimodal"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestConvertVolcengineMultimodalEmbeddingTextInputs(t *testing.T) {
	got, err := convertVolcengineMultimodalEmbeddingRequest(dto.EmbeddingRequest{
		Model: "doubao-embedding-vision-251215",
		Input: []any{"天很蓝，海很深", "第二段"},
	})
	if err != nil {
		t.Fatalf("convertVolcengineMultimodalEmbeddingRequest returned error: %v", err)
	}

	if got.Model != "doubao-embedding-vision-251215" {
		t.Fatalf("unexpected model: %q", got.Model)
	}
	if len(got.Input) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(got.Input))
	}
	if got.Input[0]["type"] != "text" || got.Input[0]["text"] != "天很蓝，海很深" {
		t.Fatalf("unexpected first input: %+v", got.Input[0])
	}
	if got.Input[1]["type"] != "text" || got.Input[1]["text"] != "第二段" {
		t.Fatalf("unexpected second input: %+v", got.Input[1])
	}
}

func TestConvertVolcengineMultimodalEmbeddingKeepsImageURLInput(t *testing.T) {
	imageURL := map[string]any{"url": "https://example.com/view.jpeg"}
	got, err := convertVolcengineMultimodalEmbeddingRequest(dto.EmbeddingRequest{
		Model: "doubao-embedding-vision-251215",
		Input: []any{
			map[string]any{"type": "image_url", "image_url": imageURL},
		},
	})
	if err != nil {
		t.Fatalf("convertVolcengineMultimodalEmbeddingRequest returned error: %v", err)
	}
	if len(got.Input) != 1 {
		t.Fatalf("expected 1 input, got %d", len(got.Input))
	}
	if got.Input[0]["type"] != "image_url" || !reflect.DeepEqual(got.Input[0]["image_url"], imageURL) {
		t.Fatalf("unexpected image input: %+v", got.Input[0])
	}
}
