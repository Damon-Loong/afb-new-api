package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestImageCountRatio(t *testing.T) {
	tests := []struct {
		name        string
		count       uint
		usePrice    bool
		tiered      bool
		usage       *dto.Usage
		actualCount float64
		wantCount   float64
	}{
		{name: "token single image", count: 1, usage: &dto.Usage{TotalTokens: 1000}},
		{name: "token multiple images", count: 2, usage: &dto.Usage{PromptTokens: 100, CompletionTokens: 2000, TotalTokens: 2100}},
		{name: "token adaptor count removed", count: 3, actualCount: 2, usage: &dto.Usage{TotalTokens: 2100}},
		{name: "token missing total", count: 2, usage: &dto.Usage{CompletionTokens: 2000}},
		{name: "per image requested count", count: 3, usePrice: true, usage: &dto.Usage{TotalTokens: 2100}, wantCount: 3},
		{name: "per image actual count", count: 3, actualCount: 2, usePrice: true, usage: &dto.Usage{TotalTokens: 2100}, wantCount: 2},
		{name: "per image without usage", count: 2, usePrice: true, wantCount: 2},
		{name: "tiered unchanged", count: 2, tiered: true, usage: &dto.Usage{TotalTokens: 2100}, wantCount: 2},
		{name: "missing usage unchanged", count: 2, wantCount: 2},
		{name: "empty usage unchanged", count: 3, usage: &dto.Usage{}, wantCount: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{PriceData: types.PriceData{UsePrice: tt.usePrice, OtherRatios: map[string]float64{"quality": 1.5}}}
			if tt.actualCount > 0 {
				info.PriceData.OtherRatios["n"] = tt.actualCount
			}
			if tt.tiered {
				info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr"}
			}
			SetImageCountRatio(info, tt.usage, tt.count)
			if got := info.PriceData.OtherRatios["n"]; got != tt.wantCount {
				t.Fatalf("image count ratio = %v, want %v", got, tt.wantCount)
			}
			if info.PriceData.OtherRatios["quality"] != 1.5 {
				t.Fatal("unrelated ratio changed")
			}
		})
	}
}
