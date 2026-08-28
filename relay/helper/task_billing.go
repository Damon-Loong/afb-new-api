package helper

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// ApplyTaskBillingRatios merges provider-specific ratios and applies them to
// per-call task quota. Both converted and native task endpoints use this path.
func ApplyTaskBillingRatios(info *relaycommon.RelayInfo, ratios map[string]float64) {
	if info == nil {
		return
	}
	for key, ratio := range ratios {
		info.PriceData.AddOtherRatio(key, ratio)
	}
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		return
	}
	quota := float64(info.PriceData.Quota)
	for _, ratio := range info.PriceData.OtherRatios {
		if ratio != 1.0 {
			quota *= ratio
		}
	}
	info.PriceData.Quota = int(quota)
}
