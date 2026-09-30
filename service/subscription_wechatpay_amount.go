package service

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// SubscriptionWeChatPayTotalFen converts the USD plan price to integer CNY cents.
// The returned amount must also be stored on the order for callback validation.
func SubscriptionWeChatPayTotalFen(priceUSD, cnyPerUSD float64) (int64, error) {
	for _, value := range []float64{priceUSD, cnyPerUSD} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf("invalid subscription price or WeChat Pay exchange rate")
		}
	}
	fen := decimal.NewFromFloat(priceUSD).Mul(decimal.NewFromFloat(cnyPerUSD)).Mul(decimal.NewFromInt(100)).Round(0)
	if fen.LessThan(decimal.NewFromInt(1)) || fen.GreaterThan(decimal.NewFromInt(math.MaxInt32)) {
		return 0, fmt.Errorf("WeChat Pay subscription amount out of range")
	}
	return fen.IntPart(), nil
}
