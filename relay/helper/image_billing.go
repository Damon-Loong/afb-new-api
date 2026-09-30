package helper

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// SetImageCountRatio is only used by the image generation/edit handler.
// Token usage returned for a request already includes all generated images.
func SetImageCountRatio(info *relaycommon.RelayInfo, usage *dto.Usage, count uint) {
	hasTokenUsage := usage != nil && (usage.TotalTokens > 0 || usage.PromptTokens > 0 || usage.CompletionTokens > 0)
	if !info.PriceData.UsePrice && info.TieredBillingSnapshot == nil && hasTokenUsage {
		delete(info.PriceData.OtherRatios, "n")
		return
	}
	// Preserve per-image pricing, tiered billing, and the legacy no-usage fallback.
	// An adaptor's actual image count takes priority over the requested count.
	if _, exists := info.PriceData.OtherRatios["n"]; !exists {
		info.PriceData.AddOtherRatio("n", float64(count))
	}
}
