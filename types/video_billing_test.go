package types

import (
	"math"
	"testing"
)

func TestVideoQuota(t *testing.T) {
	p := VideoTokenPricing{ExchangeRate: 7, LowNoVideo: 70, LowWithVideo: 42, HighNoVideo: 77, HighWithVideo: 46, ReserveTokens: 300000}
	for _, tc := range []struct {
		resolution string
		video      bool
		quota      int
	}{
		{"480p", false, 5000000}, {"720p", false, 5000000}, {"720p", true, 3000000},
		{"1080p", false, 5500000}, {"1080p", true, 3285714},
	} {
		s := VideoBillingSnapshot{Pricing: p, HasVideoInput: tc.video, GroupRatio: 1, QuotaPerUnit: 500000}
		got, err := s.Quota(1000000, tc.resolution)
		if err != nil || got != tc.quota {
			t.Fatalf("%+v: got %d, %v", tc, got, err)
		}
		s.GroupRatio = 0.6
		got, err = s.Quota(1000000, tc.resolution)
		price, _ := p.Price(tc.resolution, tc.video)
		want := int(math.Round(price / 7 * 500000 * 0.6))
		if err != nil || got != want {
			t.Fatalf("discount: got %d want %d", got, want)
		}
		s.GroupRatio = 0
		got, err = s.Quota(1000000, tc.resolution)
		if err != nil || got != 0 {
			t.Fatal("free group must stay free")
		}
	}
	s := VideoBillingSnapshot{Pricing: p, GroupRatio: 1, QuotaPerUnit: 500000}
	for _, tokens := range []int{0, -1} {
		if _, err := s.Quota(tokens, "720p"); err == nil {
			t.Fatal("missing usage accepted")
		}
	}
	if _, err := s.Quota(100, ""); err == nil {
		t.Fatal("missing resolution accepted")
	}
	p.ExchangeRate = 0
	if p.Validate() == nil {
		t.Fatal("zero exchange rate accepted")
	}
}
