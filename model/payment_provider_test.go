package model

import "testing"

func TestResolvePaymentProvider(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		paymentMethod string
		expected      string
	}{
		{name: "explicit provider wins", provider: PaymentProviderStripe, paymentMethod: "alipay", expected: PaymentProviderStripe},
		{name: "legacy stripe", paymentMethod: PaymentMethodStripe, expected: PaymentProviderStripe},
		{name: "legacy creem", paymentMethod: PaymentMethodCreem, expected: PaymentProviderCreem},
		{name: "legacy waffo", paymentMethod: PaymentMethodWaffo, expected: PaymentProviderWaffo},
		{name: "legacy pancake", paymentMethod: PaymentMethodWaffoPancake, expected: PaymentProviderWaffoPancake},
		{name: "legacy direct wechat", paymentMethod: PaymentMethodWeChatPay, expected: PaymentProviderWeChatPay},
		{name: "legacy epay alipay", paymentMethod: "alipay", expected: PaymentProviderEpay},
		{name: "legacy epay wechat", paymentMethod: "wxpay", expected: PaymentProviderEpay},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := ResolvePaymentProvider(test.provider, test.paymentMethod); actual != test.expected {
				t.Fatalf("ResolvePaymentProvider() = %q, want %q", actual, test.expected)
			}
		})
	}
}

func TestPaymentProviderMatchesRejectsCrossGatewayCallbacks(t *testing.T) {
	if PaymentProviderMatches(PaymentProviderStripe, PaymentMethodStripe, PaymentProviderEpay) {
		t.Fatal("Stripe order must not accept an Epay callback")
	}
	if PaymentProviderMatches(PaymentProviderWeChatPay, PaymentMethodWeChatPay, PaymentProviderEpay) {
		t.Fatal("direct WeChat order must not accept an Epay callback")
	}
	if PaymentProviderMatches(PaymentProviderEpay, "wxpay", PaymentProviderWeChatPay) {
		t.Fatal("Epay WeChat order must not accept a direct WeChat callback")
	}
	if !PaymentProviderMatches("", "alipay", PaymentProviderEpay) {
		t.Fatal("legacy Epay orders must remain compatible")
	}
}
