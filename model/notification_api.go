package model

type NotificationForm struct {
	Name              string `json:"name,omitempty" minLength:"1"`
	Channel           uint8  `json:"channel,omitempty"`
	URL               string `json:"url,omitempty"`
	RequestMethod     uint8  `json:"request_method,omitempty"`
	RequestType       uint8  `json:"request_type,omitempty"`
	RequestHeader     string `json:"request_header,omitempty"`
	RequestBody       string `json:"request_body,omitempty"`
	VerifyTLS         bool   `json:"verify_tls,omitempty" validate:"optional"`
	SkipCheck         bool   `json:"skip_check,omitempty" validate:"optional"`
	FormatMetricUnits bool   `json:"format_metric_units,omitempty" validate:"optional"`

	DingtalkWebhookURL string `json:"dingtalk_webhook_url,omitempty"`
	DingtalkSecret     string `json:"dingtalk_secret,omitempty"`
	// ClearDingtalkSecret 为 true 时显式清除已保存的加签密钥（从加签切回关键词
	// 模式）。普通编辑留空 dingtalk_secret 表示"不修改"，不会误清密钥。
	ClearDingtalkSecret bool   `json:"clear_dingtalk_secret,omitempty" validate:"optional"`
	ServerchanSendKey   string `json:"serverchan_sendkey,omitempty"`
}
