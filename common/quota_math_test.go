package common

import (
	"math"
	"testing"
)

func TestQuotaFromFloat(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  int
	}{
		{name: "normal", value: 123.9, want: 123},
		{name: "negative", value: -123.9, want: -123},
		{name: "nan", value: math.NaN(), want: 0},
		{name: "positive overflow", value: math.MaxFloat64, want: math.MaxInt32},
		{name: "negative overflow", value: -math.MaxFloat64, want: math.MinInt32},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := QuotaFromFloat(test.value); got != test.want {
				t.Fatalf("QuotaFromFloat(%v) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestQuotaFromInt64(t *testing.T) {
	if got := QuotaFromInt64(math.MaxInt64); got != math.MaxInt32 {
		t.Fatalf("positive overflow = %d", got)
	}
	if got := QuotaFromInt64(math.MinInt64); got != math.MinInt32 {
		t.Fatalf("negative overflow = %d", got)
	}
	if got := QuotaFromInt64(123); got != 123 {
		t.Fatalf("normal = %d", got)
	}
}
