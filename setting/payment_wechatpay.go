package setting

// WeChat Pay (API v3) settings.
//
// Notes:
// - Sensitive fields (APIv3Key / PrivateKey) are stored via /api/option and should not be echoed back by UI.
// - Notify URL should be HTTPS in production (WeChat Pay requires it).
var (
	WeChatPayEnabled bool

	// WeChatPayAppID is the appid used when creating transactions.
	// For Native Pay, this is usually the AppID of the merchant's app / official account.
	WeChatPayAppID string

	// WeChatPayMchID is the merchant id (mchid).
	WeChatPayMchID string

	// WeChatPayMchCertSerialNo is the merchant API certificate serial number.
	WeChatPayMchCertSerialNo string

	// WeChatPayAPIv3Key is the API v3 key.
	WeChatPayAPIv3Key string

	// WeChatPayPrivateKey is the merchant private key in PEM format (apiclient_key.pem content).
	WeChatPayPrivateKey string

	// Optional overrides. If empty, server will use its default callback/return URLs.
	WeChatPayNotifyURL string
	WeChatPayReturnURL string

	// Pricing settings (same meaning as other gateways): CNY per 1 USD unit.
	WeChatPayUnitPrice float64 = 7.3
	// WeChatPayMinTopUp is the minimum recharge quantity (same unit as the top-up form; can be fractional, e.g. 0.01).
	WeChatPayMinTopUp float64 = 1
)

