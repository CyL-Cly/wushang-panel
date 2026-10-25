package model

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CyL-Cly/wushang-panel/pkg/utils"
	"github.com/goccy/go-json"
)

const (
	_ = iota
	NotificationRequestTypeJSON
	NotificationRequestTypeForm
)

const (
	_ = iota
	NotificationRequestMethodGET
	NotificationRequestMethodPOST
)

// 通知渠道类型。0（webhook）为旧数据的默认值。
const (
	NotificationChannelWebhook uint8 = iota
	NotificationChannelDingtalk
	NotificationChannelServerchan
)

type NotificationServerBundle struct {
	Notification *Notification
	Server       *Server
	Loc          *time.Location
}

type Notification struct {
	Common
	Name              string `json:"name"`
	Channel           uint8  `gorm:"default:0" json:"channel"` // 0:webhook 1:钉钉 2:Server酱
	URL               string `json:"url"`
	RequestMethod     uint8  `json:"request_method"`
	RequestType       uint8  `json:"request_type"`
	RequestHeader     string `json:"request_header" gorm:"type:longtext"`
	RequestBody       string `json:"request_body" gorm:"type:longtext"`
	VerifyTLS         *bool  `json:"verify_tls,omitempty"`
	FormatMetricUnits *bool  `json:"format_metric_units,omitempty"`

	DingtalkWebhookURL string `json:"dingtalk_webhook_url"`
	DingtalkSecret     string `json:"dingtalk_secret"`
	ServerchanSendKey  string `json:"serverchan_sendkey"`
}

func (ns *NotificationServerBundle) reqURL(message string) string {
	n := ns.Notification
	return ns.replaceParamsInString(n.URL, message, func(msg string) string {
		return url.QueryEscape(msg)
	})
}

func (n *Notification) reqMethod() (string, error) {
	switch n.RequestMethod {
	case NotificationRequestMethodPOST:
		return http.MethodPost, nil
	case NotificationRequestMethodGET:
		return http.MethodGet, nil
	}
	return "", errors.New("不支持的请求方式")
}

func (ns *NotificationServerBundle) reqBody(message string) (string, error) {
	n := ns.Notification
	if n.RequestMethod == NotificationRequestMethodGET || message == "" {
		return "", nil
	}
	switch n.RequestType {
	case NotificationRequestTypeJSON:
		return ns.replaceParamsInString(n.RequestBody, message, func(msg string) string {
			msgBytes, _ := json.Marshal(msg)
			return string(msgBytes)[1 : len(msgBytes)-1]
		}), nil
	case NotificationRequestTypeForm:
		data, err := utils.GjsonIter(n.RequestBody)
		if err != nil {
			return "", err
		}
		params := url.Values{}
		for k, v := range data {
			params.Add(k, ns.replaceParamsInString(v, message, nil))
		}
		return params.Encode(), nil
	}
	return "", errors.New("不支持的请求类型")
}

func (n *Notification) setContentType(req *http.Request) {
	if n.RequestMethod == NotificationRequestMethodGET {
		return
	}
	if n.RequestType == NotificationRequestTypeForm {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
}

func (n *Notification) setRequestHeader(req *http.Request) error {
	if n.RequestHeader == "" {
		return nil
	}
	m, err := utils.GjsonIter(n.RequestHeader)
	if err != nil {
		return err
	}
	for k, v := range m {
		req.Header.Set(k, v)
	}
	return nil
}

// Send 按渠道类型分流投递。webhook 为默认（兼容旧数据）。
func (ns *NotificationServerBundle) Send(message string) error {
	switch ns.Notification.Channel {
	case NotificationChannelDingtalk:
		return ns.sendDingtalk(message)
	case NotificationChannelServerchan:
		return ns.sendServerchan(message)
	default:
		return ns.sendWebhook(message)
	}
}

func (ns *NotificationServerBundle) sendWebhook(message string) error {
	n := ns.Notification
	verifyTLS := n.VerifyTLS != nil && *n.VerifyTLS

	reqBody, err := ns.reqBody(message)
	if err != nil {
		return err
	}

	reqMethod, err := n.reqMethod()
	if err != nil {
		return err
	}

	reqURL := ns.reqURL(message)
	client, err := newNotificationHTTPClient(reqURL, verifyTLS)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(reqMethod, reqURL, strings.NewReader(reqBody))
	if err != nil {
		return err
	}

	n.setContentType(req)

	if err := n.setRequestHeader(req); err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return notificationResponseError(resp)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	return nil
}

// dingtalkResponse 是钉钉机器人接口的响应体，errcode != 0 表示业务失败。
type dingtalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// dingtalkSignedURL 按钉钉加签规范追加 timestamp+sign；未配置 Secret 时原样返回。
func (n *Notification) dingtalkSignedURL(timestampMs int64) string {
	reqURL := n.DingtalkWebhookURL
	secret := strings.TrimSpace(n.DingtalkSecret)
	if secret == "" {
		return reqURL
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d\n%s", timestampMs, secret)))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sep := "&"
	if !strings.Contains(reqURL, "?") {
		sep = "?"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", reqURL, sep, timestampMs, url.QueryEscape(sign))
}

// dingtalkTextPayload 构造钉钉文本消息体，内容以 "#NEZHA# " 前缀，
// 配合「自定义关键词」安全设置时可直接以 #NEZHA# 作为关键词。
// 注意：这里不做 #NEZHA# 占位替换（前缀本身就是占位符含义），只渲染
// #DATETIME# 与 #SERVER.*# 等其余占位符，避免消息内容重复拼接。
func (ns *NotificationServerBundle) dingtalkTextPayload(message string) ([]byte, error) {
	content := "#NEZHA# " + ns.replaceServerPlaceholders(message, message, nil)
	return json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	})
}

// sendDingtalk 通过钉钉自定义机器人 Webhook 推送文本消息。
//   - 支持「加签」安全设置：Secret 非空时按钉钉规范追加 timestamp+sign。
//   - 消息内容以 "#NEZHA# " 前缀，同时继续渲染 #SERVER.*# 等占位符。
func (ns *NotificationServerBundle) sendDingtalk(message string) error {
	n := ns.Notification
	if strings.TrimSpace(n.DingtalkWebhookURL) == "" {
		return errors.New("钉钉 Webhook 地址未配置")
	}

	reqURL := n.dingtalkSignedURL(time.Now().UnixMilli())

	payload, err := ns.dingtalkTextPayload(message)
	if err != nil {
		return err
	}

	client, err := newNotificationHTTPClient(reqURL, false)
	if err != nil {
		return err
	}

	resp, err := client.Post(reqURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// 请求 URL 含 access_token，错误信息只保留状态码，不回显响应体外的任何 URL 信息。
		return fmt.Errorf("dingtalk: unexpected HTTP status %d", resp.StatusCode)
	}

	var dr dingtalkResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return fmt.Errorf("dingtalk: malformed response (%T)", err)
	}
	if dr.ErrCode != 0 {
		// 业务错误信息（如关键词不匹配）由钉钉明文给出，不含凭据，可透出。
		return fmt.Errorf("dingtalk: errcode=%d errmsg=%s", dr.ErrCode, dr.ErrMsg)
	}
	return nil
}

// sendServerchan 通过 Server酱³ (sctapi.ftqq.com) 推送消息。
// title 取消息首行（截 32 个 rune），desp 为全文（Markdown）。
func (ns *NotificationServerBundle) sendServerchan(message string) error {
	sendKey := strings.TrimSpace(ns.Notification.ServerchanSendKey)
	if sendKey == "" {
		return errors.New("Server酱 SendKey 未配置")
	}

	rendered := ns.replaceServerPlaceholders(message, message, nil)
	title := rendered
	if idx := strings.IndexAny(title, "\r\n"); idx >= 0 {
		title = title[:idx]
	}
	title = strings.TrimSpace(title)
	if titleRunes := []rune(title); len(titleRunes) > 32 {
		title = string(titleRunes[:32])
	}

	form := url.Values{}
	form.Set("title", string(title))
	form.Set("desp", rendered)

	reqURL := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", sendKey)
	client, err := newNotificationHTTPClient(reqURL, false)
	if err != nil {
		return err
	}

	resp, err := client.PostForm(reqURL, form)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// SendKey 在 URL 路径里，错误信息不回显 URL。
		return fmt.Errorf("serverchan: unexpected HTTP status %d", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func notificationResponseError(resp *http.Response) error {
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	return fmt.Errorf("%d@%s", resp.StatusCode, resp.Status)
}

func newNotificationHTTPClient(rawURL string, verifyTLS bool) (*http.Client, error) {
	return utils.NewRestrictedHTTPClient(rawURL, !verifyTLS)
}

// replaceParamInString 替换字符串中的占位符
func (ns *NotificationServerBundle) replaceParamsInString(str string, message string, mod func(string) string) string {
	return ns.replacePlaceholders(str, message, mod, true)
}

// replaceServerPlaceholders 替换 #DATETIME#/#SERVER.*# 占位符，但不替换
// #NEZHA#（钉钉/Server酱正文自行拼接 message，占位互替换会重复展开）。
func (ns *NotificationServerBundle) replaceServerPlaceholders(str string, message string, mod func(string) string) string {
	return ns.replacePlaceholders(str, message, mod, false)
}

func (ns *NotificationServerBundle) replacePlaceholders(str string, message string, mod func(string) string, withMessage bool) string {
	if mod == nil {
		mod = func(s string) string { return s }
	}

	replacements := []string{
		"#DATETIME#", mod(time.Now().In(ns.Loc).String()),
	}
	if withMessage {
		replacements = append(replacements, "#NEZHA#", mod(message))
	}

	if ns.Server != nil {
		runtime := ns.Server.RuntimeSnapshot()
		replacements = append(replacements,
			"#SERVER.NAME#", mod(ns.Server.Name),
			"#SERVER.ID#", mod(fmt.Sprintf("%d", ns.Server.ID)),
		)
		if runtime.State != nil && runtime.Host != nil {
			state := runtime.State
			host := runtime.Host
			replacements = append(replacements,
				// Converted metrics
				"#SERVER.CPU#", mod(ns.formatUsage(false, state.CPU)),
				"#SERVER.MEM#", mod(ns.formatUsage(true, float64(state.MemUsed)/float64(host.MemTotal))),
				"#SERVER.SWAP#", mod(ns.formatUsage(true, float64(state.SwapUsed)/float64(host.SwapTotal))),
				"#SERVER.DISK#", mod(ns.formatUsage(true, float64(state.DiskUsed)/float64(host.DiskTotal))),
				"#SERVER.SPEEDIN#", mod(fmt.Sprintf("%s/s", ns.formatSize(state.NetInSpeed))),
				"#SERVER.SPEEDOUT#", mod(fmt.Sprintf("%s/s", ns.formatSize(state.NetOutSpeed))),
				"#SERVER.TRANSFERIN#", mod(ns.formatSize(state.NetInTransfer)),
				"#SERVER.TRANSFEROUT#", mod(ns.formatSize(state.NetOutTransfer)),

				// Raw metrics
				"#SERVER.CPUUSED#", mod(fmt.Sprintf("%f", state.CPU)),
				"#SERVER.MEMUSED#", mod(fmt.Sprintf("%d", state.MemUsed)),
				"#SERVER.SWAPUSED#", mod(fmt.Sprintf("%d", state.SwapUsed)),
				"#SERVER.DISKUSED#", mod(fmt.Sprintf("%d", state.DiskUsed)),
				"#SERVER.MEMTOTAL#", mod(fmt.Sprintf("%d", host.MemTotal)),
				"#SERVER.SWAPTOTAL#", mod(fmt.Sprintf("%d", host.SwapTotal)),
				"#SERVER.DISKTOTAL#", mod(fmt.Sprintf("%d", host.DiskTotal)),
				"#SERVER.NETINSPEED#", mod(fmt.Sprintf("%d", state.NetInSpeed)),
				"#SERVER.NETOUTSPEED#", mod(fmt.Sprintf("%d", state.NetOutSpeed)),
				"#SERVER.NETINTRANSFER#", mod(fmt.Sprintf("%d", state.NetInTransfer)),
				"#SERVER.NETOUTTRANSFER#", mod(fmt.Sprintf("%d", state.NetOutTransfer)),
				"#SERVER.LOAD1#", mod(fmt.Sprintf("%f", state.Load1)),
				"#SERVER.LOAD5#", mod(fmt.Sprintf("%f", state.Load5)),
				"#SERVER.LOAD15#", mod(fmt.Sprintf("%f", state.Load15)),
				"#SERVER.TCPCONNCOUNT#", mod(fmt.Sprintf("%d", state.TcpConnCount)),
				"#SERVER.UDPCONNCOUNT#", mod(fmt.Sprintf("%d", state.UdpConnCount)),
			)
		}

		var ipv4, ipv6, validIP string
		if ns.Server.GeoIP != nil {
			ip := ns.Server.GeoIP.IP
			if ip.IPv4Addr != "" && ip.IPv6Addr != "" {
				ipv4 = ip.IPv4Addr
				ipv6 = ip.IPv6Addr
				validIP = ipv4
			} else if ip.IPv4Addr != "" {
				ipv4 = ip.IPv4Addr
				validIP = ipv4
			} else {
				ipv6 = ip.IPv6Addr
				validIP = ipv6
			}

			replacements = append(replacements,
				"#SERVER.IP#", mod(validIP),
				"#SERVER.IPV4#", mod(ipv4),
				"#SERVER.IPV6#", mod(ipv6),
			)
		}
	}

	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(str)
}

func (ns *NotificationServerBundle) formatUsage(toPercentage bool, usage float64) string {
	if ns.Notification.FormatMetricUnits != nil && *ns.Notification.FormatMetricUnits {
		if toPercentage {
			usage = usage * 100
		}

		return fmt.Sprintf("%.2f %%", usage)
	}
	return fmt.Sprintf("%f", usage)
}

func (ns *NotificationServerBundle) formatSize(size uint64) string {
	if ns.Notification.FormatMetricUnits != nil && *ns.Notification.FormatMetricUnits {
		return utils.Bytes(size)
	}
	return fmt.Sprintf("%d", size)
}
