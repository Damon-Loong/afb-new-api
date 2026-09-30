package service

import (
	"math"
	"testing"
)

func TestSubscriptionWeChatPayTotalFen(t *testing.T) {
	for _, tc := range []struct {
		name        string
		price, rate float64
		want        int64
	}{
		{"150 USD at 7 CNY", 150, 7, 105000},
		{"fractional exchange rate", 9.99, 6.74, 6733},
		{"half cent rounds up", 0.335, 7, 235},
		{"minimum cent", 0.01, 1, 1},
		{"zero rate", 150, 0, 0},
		{"negative rate", 150, -1, 0},
		{"negative price", -1, 7, 0},
		{"below minimum", 0.001, 1, 0},
		{"invalid number", math.NaN(), 7, 0},
		{"infinite rate", 1, math.Inf(1), 0},
		{"overflow", 1e20, 7, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SubscriptionWeChatPayTotalFen(tc.price, tc.rate)
			if tc.want == 0 {
				if err == nil {
					t.Fatal("expected invalid amount to be rejected")
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("got %d, %v; want %d cents", got, err, tc.want)
			}
		})
	}
}
