package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

func TestParseAutoRouteEmbeddingResponseSupportsNestedEmbedding(t *testing.T) {
	body := []byte(`{
		"object": "list",
		"data": [
			{"index": 1, "embedding": [[0.3, 0.4]]},
			{"index": 0, "embedding": [[0.1, 0.2]]}
		],
		"usage": {"prompt_tokens": 9, "total_tokens": 9}
	}`)

	embeddings, usage, err := parseAutoRouteEmbeddingResponse(body)
	if err != nil {
		t.Fatalf("parseAutoRouteEmbeddingResponse returned error: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if embeddings[0][0] != 0.1 || embeddings[0][1] != 0.2 {
		t.Fatalf("unexpected first embedding: %+v", embeddings[0])
	}
	if embeddings[1][0] != 0.3 || embeddings[1][1] != 0.4 {
		t.Fatalf("unexpected second embedding: %+v", embeddings[1])
	}
	if usage.PromptTokens != 9 || usage.TotalTokens != 9 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestParseAutoRouteEmbeddingResponseSupportsVolcengineSingleDataObject(t *testing.T) {
	body := []byte(`{
		"created": 1743575029,
		"data": {
			"embedding": [-0.123046875, -0.35546875, -0.318359375],
			"object": "embedding"
		},
		"id": "021743575029461acbe49a31755bec77b2f09448eb15fa9a88e47",
		"model": "doubao-embedding-vision-250615",
		"object": "list",
		"usage": {
			"prompt_tokens": 13987,
			"prompt_tokens_details": {
				"image_tokens": 13800,
				"text_tokens": 187
			},
			"total_tokens": 13987
		}
	}`)

	embeddings, usage, err := parseAutoRouteEmbeddingResponse(body)
	if err != nil {
		t.Fatalf("parseAutoRouteEmbeddingResponse returned error: %v", err)
	}
	if len(embeddings) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(embeddings))
	}
	if embeddings[0][0] != -0.123046875 || embeddings[0][2] != -0.318359375 {
		t.Fatalf("unexpected embedding: %+v", embeddings[0])
	}
	if usage.PromptTokens != 13987 || usage.TotalTokens != 13987 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if usage.PromptTokensDetails.ImageTokens != 13800 || usage.PromptTokensDetails.TextTokens != 187 {
		t.Fatalf("unexpected token details: %+v", usage.PromptTokensDetails)
	}
}

func TestExtractAutoRouteEmbeddingVectorSupportsNamedDenseVector(t *testing.T) {
	got := extractAutoRouteEmbeddingVector(map[string]any{
		"dense": []any{0.1, 0.2, 0.3},
	})
	if len(got) != 3 || got[0] != 0.1 || got[2] != 0.3 {
		t.Fatalf("unexpected vector: %+v", got)
	}
}

func TestAutoRouteEmbeddingTiersHaveSeedSamples(t *testing.T) {
	if len(autoRouteEmbeddingTiers) != 6 {
		t.Fatalf("expected 6 embedding tiers, got %d", len(autoRouteEmbeddingTiers))
	}
	for _, tier := range autoRouteEmbeddingTiers {
		if tier.Score <= 0 || tier.Label == "" {
			t.Fatalf("invalid tier metadata: %+v", tier)
		}
		if len(tier.Samples) < 18 {
			t.Fatalf("tier %s has too few samples: %d", tier.Label, len(tier.Samples))
		}
		if autoRouteEmbeddingTierText(tier) == tier.Text {
			t.Fatalf("tier %s text did not include samples", tier.Label)
		}
	}
}

func TestAutoRouteScoringTextIncludesPreviousTaskForContinuation(t *testing.T) {
	latest, previous := latestAndPreviousChatUserText([]dto.Message{
		{Role: "user", Content: "分析这个 Go 并发代码的数据竞争并给出修复方案"},
		{Role: "assistant", Content: "可以。"},
		{Role: "user", Content: "继续"},
	})
	if latest != "继续" || previous == "" {
		t.Fatalf("unexpected latest/previous: latest=%q previous=%q", latest, previous)
	}

	scoringText := autoRouteScoringText(latest, previous, autoRouteFeatures{})
	if !strings.Contains(scoringText, "当前用户请求: 继续") || !strings.Contains(scoringText, "Go 并发代码") {
		t.Fatalf("continuation scoring text did not include previous task: %q", scoringText)
	}

	inputs := autoRouteEmbeddingInputs(autoRouteFeatures{LatestText: latest, ScoringText: scoringText})
	if first, _ := inputs[0].(string); !strings.Contains(first, "Go 并发代码") {
		t.Fatalf("embedding input did not use scoring text: %#v", inputs[0])
	}
}

func TestAutoRouteScoringTextKeepsContinuationChainAndFindsRealTask(t *testing.T) {
	latest, previous := latestAndPreviousChatUserText([]dto.Message{
		{Role: "user", Content: "谁是老夫子？"},
		{Role: "assistant", Content: "老夫子是..."},
		{Role: "user", Content: "继续"},
		{Role: "assistant", Content: "继续补充..."},
		{Role: "user", Content: "继续"},
	})
	if latest != "继续" {
		t.Fatalf("unexpected latest: %q", latest)
	}
	if !strings.Contains(previous, "谁是老夫子") || !strings.Contains(previous, "中间续写指令: 继续") {
		t.Fatalf("previous context did not keep continuation chain and real task: %q", previous)
	}

	scoringText := autoRouteScoringText(latest, previous, autoRouteFeatures{})
	if !strings.Contains(scoringText, "当前用户请求: 继续") ||
		!strings.Contains(scoringText, "谁是老夫子") ||
		!strings.Contains(scoringText, "中间续写指令") {
		t.Fatalf("unexpected scoring text: %q", scoringText)
	}
}

func TestScoreAutoRouteFromEmbeddingsAppliesContinuationFloor(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	embeddings := [][]float64{
		{1, 0},
		{1, 0}, // light wins clearly
		{0.6, 0.8},
		{0.2, 0.98},
		{0.1, 0.99},
		{0, 1},
		{-1, 0},
	}

	result, err := scoreAutoRouteFromEmbeddings(c, embeddings, autoRouteFeatures{
		LatestText:   "继续",
		HistoryTurns: 1,
	})
	if err != nil {
		t.Fatalf("scoreAutoRouteFromEmbeddings returned error: %v", err)
	}
	if result.Difficulty != 30 {
		t.Fatalf("expected continuation floor to score 30, got %d", result.Difficulty)
	}
}

func TestScoreAutoRouteFromEmbeddingsKeepsAmbiguousHighTierConservative(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	embeddings := [][]float64{
		{1, 0},
		{0.9800, 0.1990}, // light, very close to query
		{0.20, 0.98},
		{0.10, 0.99},
		{0.15, 0.98},
		{0.30, 0.95},
		{0.9850, 0.1725}, // flagship, slightly closer but ambiguous
	}

	result, err := scoreAutoRouteFromEmbeddings(c, embeddings, autoRouteFeatures{})
	if err != nil {
		t.Fatalf("scoreAutoRouteFromEmbeddings returned error: %v", err)
	}
	if result.Difficulty != 15 {
		t.Fatalf("expected ambiguous high-tier match to stay conservative at 15, got %d", result.Difficulty)
	}
}

func TestScoreAutoRouteFromEmbeddingsUsesClearNearestTier(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	embeddings := [][]float64{
		{0, 1},
		{1, 0},
		{0.6, 0.8},
		{0.7, 0.7},
		{0, 1},
		{0.2, 0.98},
		{-1, 0},
	}

	result, err := scoreAutoRouteFromEmbeddings(c, embeddings, autoRouteFeatures{})
	if err != nil {
		t.Fatalf("scoreAutoRouteFromEmbeddings returned error: %v", err)
	}
	if result.Difficulty != 60 {
		t.Fatalf("expected clear advanced match to score 60, got %d", result.Difficulty)
	}
}

func TestSelectAutoRouteCandidatePrefersNearestAdequateTierThenCost(t *testing.T) {
	candidates := []autoRouteCandidate{
		{ModelName: "flagship-cheap", RouterScore: 90, EstimatedCost: 1},
		{ModelName: "daily-expensive", RouterScore: 30, EstimatedCost: 5},
		{ModelName: "standard-cheaper", RouterScore: 45, EstimatedCost: 2},
		{ModelName: "light", RouterScore: 15, EstimatedCost: 0.5},
	}

	selected, underpowered := selectAutoRouteCandidate(candidates, 30)
	if underpowered {
		t.Fatalf("expected adequate candidate")
	}
	if selected.ModelName != "daily-expensive" {
		t.Fatalf("expected nearest adequate tier, got %+v", selected)
	}
}

func TestFilterAutoRouteCandidatesForTokenKeepsBaseCandidatesImmutable(t *testing.T) {
	base := []autoRouteCandidate{
		{ModelName: "model-a", RouterScore: 20, Group: "default", EstimatedCost: 2},
		{ModelName: "model-b", RouterScore: 80, Group: "default", EstimatedCost: 1},
	}

	filtered := filterAutoRouteCandidatesForToken(base, map[string]bool{"model-b": true}, true)
	if len(filtered) != 1 || filtered[0].ModelName != "model-b" {
		t.Fatalf("unexpected filtered candidates: %+v", filtered)
	}
	filtered[0].ModelName = "mutated"
	if base[1].ModelName != "model-b" {
		t.Fatalf("base candidates were mutated: %+v", base)
	}

	unlimited := filterAutoRouteCandidatesForToken(base, nil, false)
	if len(unlimited) != len(base) {
		t.Fatalf("expected all candidates without token limit, got %+v", unlimited)
	}
	unlimited[0].ModelName = "mutated-again"
	if base[0].ModelName != "model-a" {
		t.Fatalf("base candidates were mutated by unlimited copy: %+v", base)
	}
}
