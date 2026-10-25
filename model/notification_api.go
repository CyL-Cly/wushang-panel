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
	ServerchanSendKey  string `json:"serverchan_sendkey,omitempty"`
}
