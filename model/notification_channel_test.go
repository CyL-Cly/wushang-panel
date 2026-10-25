package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

// TestSendDingtalkMissingWebhookURL and TestSendServerchanMissingSendKey pin
// the credential-presence guards on the dedicated channels.
func TestSendDingtalkMissingWebhookURL(t *testing.T) {
	bundle := &NotificationServerBundle{
		Notification: &Notification{Channel: NotificationChannelDingtalk},
		Loc:          time.UTC,
	}
	if err := bundle.Send("hello"); err == nil {
		t.Fatal("expected missing webhook url error")
	}
}

func TestSendServerchanMissingSendKey(t *testing.T) {
	bundle := &NotificationServerBundle{
		Notification: &Notification{Channel: NotificationChannelServerchan},
		Loc:          time.UTC,
	}
	err := bundle.Send("hello")
	if err == nil {
		t.Fatal("expected missing sendkey error")
	}
	if !strings.Contains(err.Error(), "SendKey") {
		t.Fatalf("error should mention SendKey, got %q", err.Error())
	}
}

// TestDingtalkSignedURL pins the official signing recipe used by
// sendDingtalk: appending timestamp + url-escaped base64
// HmacSHA256(key=secret, msg=timestamp+"\n"+secret) to the webhook URL.
func TestDingtalkSignedURL(t *testing.T) {
	const secret = "SECtest-secret"
	const ts = int64(1719999999999)

	wantMAC := hmac.New(sha256.New, []byte(secret))
	wantMAC.Write([]byte("1719999999999\n" + secret))
	wantSign := base64.StdEncoding.EncodeToString(wantMAC.Sum(nil))

	n := &Notification{
		Channel:            NotificationChannelDingtalk,
		DingtalkWebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=abc",
		DingtalkSecret:     secret,
	}
	got := n.dingtalkSignedURL(ts)

	wantPrefix := "https://oapi.dingtalk.com/robot/send?access_token=abc&timestamp=1719999999999&sign="
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("signed URL prefix mismatch:\n got %q\nwant prefix %q", got, wantPrefix)
	}
	if !strings.HasSuffix(got, url.QueryEscape(wantSign)) {
		t.Fatalf("sign mismatch: got %q want %q", got, url.QueryEscape(wantSign))
	}

	// keyword-mode（无 Secret）不得携带 timestamp/sign。
	bare := &Notification{DingtalkWebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=abc"}
	if bareURL := bare.dingtalkSignedURL(ts); bareURL != bare.DingtalkWebhookURL {
		t.Fatalf("unsigned robot URL must stay untouched, got %q", bareURL)
	}

	// URL 无查询串时必须用 ? 而不是 & 拼接。
	noQuery := &Notification{DingtalkWebhookURL: "https://oapi.dingtalk.com/robot/send", DingtalkSecret: secret}
	signed := noQuery.dingtalkSignedURL(ts)
	if !strings.Contains(signed, "send?timestamp=") {
		t.Fatalf("missing query separator '?': %q", signed)
	}
	if strings.Contains(signed, "&timestamp=") {
		t.Fatalf("first parameter must use '?': %q", signed)
	}
}

// TestServerchanTitle pins the title extraction: first line, trimmed,
// truncated to 32 runes.
func TestServerchanTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"first line wins", "主机宕机告警\n详情见正文", "主机宕机告警"},
		{"crlf handled", "标题\r\n第二行", "标题"},
		{"trimmed", "   空白标题   ", "空白标题"},
		{"32 runes kept", strings.Repeat("字", 32), strings.Repeat("字", 32)},
		{"33 runes truncated", strings.Repeat("字", 33), strings.Repeat("字", 32)},
		{"multiline truncated to first line", strings.Repeat("a", 40) + "\nrest", strings.Repeat("a", 32)},
		{"empty message", "", ""},
	}
	for _, tc := range cases {
		if got := serverchanTitle(tc.in); got != tc.want {
			t.Errorf("%s: serverchanTitle(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestServerchanResponseUnmarshal pins that sendServerchan's business-error
// contract (code != 0 means failure even with HTTP 200).
func TestServerchanResponseUnmarshal(t *testing.T) {
	var ok serverchanResponse
	if err := json.Unmarshal([]byte(`{"code":0,"message":"success"}`), &ok); err != nil {
		t.Fatal(err)
	}
	if ok.Code != 0 {
		t.Fatalf("code=0 expected, got %d", ok.Code)
	}

	var fail serverchanResponse
	if err := json.Unmarshal([]byte(`{"code":40001,"message":"bad sendkey"}`), &fail); err != nil {
		t.Fatal(err)
	}
	if fail.Code == 0 {
		t.Fatal("non-zero business code must not parse as success")
	}
	if fail.Message != "bad sendkey" {
		t.Fatalf("message mismatch: %q", fail.Message)
	}
}

// TestDingtalkPayload pins the request body shape: text message with a
// #NEZHA# prefix so keyword-secured robots match out of the box.
func TestDingtalkPayload(t *testing.T) {
	bundle := &NotificationServerBundle{
		Notification: &Notification{Channel: NotificationChannelDingtalk, Name: "n"},
		Loc:          time.UTC,
	}
	payload, err := bundle.dingtalkTextPayload("host down")
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("payload is not valid dingtalk JSON: %v (%s)", err, payload)
	}
	if decoded.MsgType != "text" {
		t.Fatalf("expected msgtype text, got %q", decoded.MsgType)
	}
	if decoded.Text.Content != "#NEZHA# host down" {
		t.Fatalf("unexpected content %q", decoded.Text.Content)
	}
}
