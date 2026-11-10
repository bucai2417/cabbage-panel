package panel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type NotificationChannel struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
	Token   string `json:"token,omitempty"`
	Secret  string `json:"secret,omitempty"`
	ChatID  string `json:"chatId,omitempty"`
}
type NotificationConfig struct {
	NotifyFailures bool                  `json:"notifyFailures"`
	Channels       []NotificationChannel `json:"channels"`
}

var notificationTypes = []string{"webhook", "wecom", "dingtalk", "feishu", "bark", "telegram", "pushplus", "serverchan"}

func (app *App) notificationConfig() NotificationConfig {
	config := NotificationConfig{NotifyFailures: app.store.setting("notifyFailures", "true") != "false", Channels: []NotificationChannel{}}
	value := app.store.setting("notifications", "")
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config)
	} else if address := app.store.setting("webhookURL", ""); address != "" {
		config.Channels = append(config.Channels, NotificationChannel{Type: "webhook", Name: "原有 Webhook", Enabled: true, URL: address})
	}
	config.NotifyFailures = app.store.setting("notifyFailures", "true") != "false"
	if config.Channels == nil {
		config.Channels = []NotificationChannel{}
	}
	return config
}

func (app *App) saveNotifications(config NotificationConfig) error {
	if config.Channels == nil {
		config.Channels = []NotificationChannel{}
	}
	if len(config.Channels) > 16 {
		return errors.New("最多配置 16 个通知渠道")
	}
	for _, channel := range config.Channels {
		if err := validateNotificationDestination(channel); err != nil {
			return err
		}
		if !contains(notificationTypes, channel.Type) {
			return errors.New("未知通知渠道")
		}
		if channel.URL != "" && !validHTTP(channel.URL) {
			return errors.New("通知地址必须是有效的 HTTP(S) URL")
		}
		if len(channel.Token) > 4096 || len(channel.Secret) > 4096 || len(channel.Name) > 100 {
			return errors.New("通知字段过长")
		}
		if strings.ContainsAny(channel.Token, "\n/?#&") || strings.ContainsAny(channel.Secret, "\n") {
			return errors.New("通知凭证包含无效字符")
		}
		if !channel.Enabled {
			continue
		}
		if contains([]string{"webhook", "wecom", "feishu", "bark"}, channel.Type) && channel.URL == "" && channel.Token == "" {
			return errors.New("启用的通知渠道需要 URL 或 Token")
		}
		if channel.Type == "webhook" && channel.URL == "" {
			return errors.New("Webhook 地址不能为空")
		}
		if contains([]string{"dingtalk", "telegram", "pushplus", "serverchan"}, channel.Type) && channel.Token == "" {
			return errors.New("启用的通知渠道需要 Token")
		}
		if channel.Type == "telegram" && channel.ChatID == "" {
			return errors.New("Telegram 需要 Chat ID")
		}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	transaction, err := app.store.db.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	for key, value := range map[string]string{"notifications": string(data), "notifyFailures": fmt.Sprint(config.NotifyFailures)} {
		if _, err := transaction.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (app *App) notificationsAPI(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		ok(writer, app.notificationConfig())
	case "PUT":
		var config NotificationConfig
		if err := decode(request, &config); err != nil {
			apiError(writer, err)
			return
		}
		if err := app.saveNotifications(config); err != nil {
			apiError(writer, err)
			return
		}
		ok(writer, nil)
	case "POST":
		result := app.sendNotifications(request.Context(), "大白菜面板", "这是一条测试通知。中文编码正常。", app.notificationConfig())
		ok(writer, result)
	default:
		fail(writer, 405, "方法不支持")
	}
}

func (app *App) notify(ctx context.Context, title, message string) error {
	config := app.notificationConfig()
	if !config.NotifyFailures && title != "大白菜面板" {
		return nil
	}
	results := app.sendNotifications(ctx, title, message, config)
	problems := []string{}
	for _, result := range results {
		if !boolean(result["success"]) {
			problems = append(problems, text(result["name"])+": "+text(result["error"]))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (app *App) sendNotifications(ctx context.Context, title, message string, config NotificationConfig) []Object {
	channels := []NotificationChannel{}
	for _, channel := range config.Channels {
		if channel.Enabled {
			channels = append(channels, channel)
		}
	}
	results := make([]Object, len(channels))
	var group sync.WaitGroup
	for index, channel := range channels {
		group.Add(1)
		go func(index int, channel NotificationChannel) {
			defer group.Done()
			err := app.sendChannel(ctx, channel, title, message)
			result := Object{"type": channel.Type, "name": channel.Name, "success": err == nil}
			if err != nil {
				result["error"] = err.Error()
			}
			results[index] = result
		}(index, channel)
	}
	group.Wait()
	return results
}

func channelPayload(channel NotificationChannel, title, message string) (string, Object) {
	address := channel.URL
	combined := title + "\n\n" + message
	payload := Object{"title": title, "message": message, "content": combined, "msgtype": "text", "text": Object{"content": combined}}
	switch channel.Type {
	case "wecom":
		if address == "" {
			address = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + url.QueryEscape(channel.Token)
		}
		payload = Object{"msgtype": "text", "text": Object{"content": combined}}
	case "dingtalk":
		if address == "" {
			address = "https://oapi.dingtalk.com/robot/send?access_token=" + url.QueryEscape(channel.Token)
		}
		payload = Object{"msgtype": "text", "text": Object{"content": combined}}
		if channel.Secret != "" {
			timestamp := fmt.Sprint(time.Now().UnixMilli())
			signature := hmac.New(sha256.New, []byte(channel.Secret))
			signature.Write([]byte(timestamp + "\n" + channel.Secret))
			parsed, _ := url.Parse(address)
			query := parsed.Query()
			query.Set("timestamp", timestamp)
			query.Set("sign", base64.StdEncoding.EncodeToString(signature.Sum(nil)))
			parsed.RawQuery = query.Encode()
			address = parsed.String()
		}
	case "feishu":
		if address == "" {
			address = "https://open.feishu.cn/open-apis/bot/v2/hook/" + url.PathEscape(channel.Token)
		}
		payload = Object{"msg_type": "text", "content": Object{"text": combined}}
		if channel.Secret != "" {
			timestamp := fmt.Sprint(time.Now().Unix())
			signature := hmac.New(sha256.New, []byte(timestamp+"\n"+channel.Secret))
			payload["timestamp"] = timestamp
			payload["sign"] = base64.StdEncoding.EncodeToString(signature.Sum(nil))
		}
	case "bark":
		if address == "" {
			address = "https://api.day.app/" + url.PathEscape(channel.Token)
		}
		payload = Object{"title": title, "body": message, "group": "大白菜面板"}
	case "telegram":
		if address == "" {
			address = "https://api.telegram.org"
		}
		address = strings.TrimSuffix(address, "/") + "/bot" + channel.Token + "/sendMessage"
		payload = Object{"chat_id": channel.ChatID, "text": combined, "disable_web_page_preview": true}
	case "pushplus":
		if address == "" {
			address = "https://www.pushplus.plus/send"
		}
		payload = Object{"token": channel.Token, "title": title, "content": message, "template": "txt"}
	case "serverchan":
		if address == "" {
			address = "https://sctapi.ftqq.com/" + url.PathEscape(channel.Token) + ".send"
		}
		payload = Object{"title": title, "desp": message}
	}
	return address, payload
}

func (app *App) sendChannel(ctx context.Context, channel NotificationChannel, title, message string) error {
	address, payload := channelPayload(channel, title, message)
	if !validHTTP(address) {
		return errors.New("通知地址无效")
	}
	data, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "POST", address, bytes.NewReader(data))
	if err != nil {
		return errors.New("无法创建通知请求")
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response, err := app.client.Do(request)
	if err != nil {
		return errors.New("通知网络请求失败，请检查地址和网络连接")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("通知服务返回 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return errors.New("无法读取通知响应")
	}
	if channel.Type == "webhook" {
		return nil
	}
	var result Object
	if json.Unmarshal(body, &result) != nil {
		return errors.New("通知服务未返回有效 JSON")
	}
	success := false
	switch channel.Type {
	case "telegram":
		success = boolean(result["ok"])
	case "wecom", "dingtalk":
		success = result["errcode"] != nil && integer(result["errcode"]) == 0
	case "feishu":
		success = (result["code"] != nil && integer(result["code"]) == 0) || (result["StatusCode"] != nil && integer(result["StatusCode"]) == 0)
	case "bark", "pushplus":
		success = integer(result["code"]) == 200
	case "serverchan":
		success = result["code"] != nil && integer(result["code"]) == 0
	}
	if !success {
		return errors.New("通知服务拒绝请求，请检查凭证、签名或机器人关键词设置")
	}
	return nil
}
