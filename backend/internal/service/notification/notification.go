package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NotificationInfo 通知信息
type NotificationInfo struct {
	OfficialTitle string
	Season        int
	Episode       int
	Message       string
}

// NotificationProvider 通知提供者接口
type NotificationProvider interface {
	Send(ctx context.Context, info *NotificationInfo) error
	Test(ctx context.Context) error
}

func formatMessage(info *NotificationInfo) string {
	if info.Message != "" {
		return info.Message
	}
	if info.OfficialTitle != "" {
		return fmt.Sprintf("%s S%02dE%02d 已更新", info.OfficialTitle, info.Season, info.Episode)
	}
	return "AniDog通知"
}

// TelegramProvider Telegram 通知
type TelegramProvider struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func (p *TelegramProvider) Send(ctx context.Context, info *NotificationInfo) error {
	msg := formatMessage(info)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", p.BotToken)
	form := url.Values{"chat_id": {p.ChatID}, "text": {msg}}

	resp, err := http.PostForm(apiURL, form)
	if err != nil {
		return fmt.Errorf("Telegram 发送失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API 返回 %d", resp.StatusCode)
	}
	return nil
}

func (p *TelegramProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Telegram 通知测试"})
}

// BarkProvider Bark 通知
type BarkProvider struct {
	ServerURL string `json:"server_url"`
	DeviceKey string `json:"device_key"`
}

func (p *BarkProvider) Send(ctx context.Context, info *NotificationInfo) error {
	msg := formatMessage(info)
	payload := map[string]string{"device_key": p.DeviceKey, "title": "AniDog", "body": msg}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(p.ServerURL+"/"+p.DeviceKey, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("Bark 发送失败: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (p *BarkProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Bark 通知测试"})
}

// DiscordProvider Discord 通知
type DiscordProvider struct {
	WebhookURL string `json:"webhook_url"`
}

func (p *DiscordProvider) Send(ctx context.Context, info *NotificationInfo) error {
	msg := formatMessage(info)
	payload := map[string]string{"content": msg}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(p.WebhookURL, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("Discord 发送失败: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (p *DiscordProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Discord 通知测试"})
}

// WebhookProvider 通用 Webhook 通知
type WebhookProvider struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (p *WebhookProvider) Send(ctx context.Context, info *NotificationInfo) error {
	msg := formatMessage(info)
	payload := map[string]string{"message": msg, "title": "AniDog"}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, p.URL, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Webhook 发送失败: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (p *WebhookProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Webhook 通知测试"})
}

// ServerChanProvider Server酱通知
type ServerChanProvider struct {
	SendKey string `json:"send_key"`
}

func (p *ServerChanProvider) Send(ctx context.Context, info *NotificationInfo) error {
	msg := formatMessage(info)
	apiURL := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", p.SendKey)
	form := url.Values{"title": {"AniDog"}, "desp": {msg}}

	resp, err := http.PostForm(apiURL, form)
	if err != nil {
		return fmt.Errorf("Server酱发送失败: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (p *ServerChanProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Server酱通知测试"})
}

// WeComProvider 企业微信通知
type WeComProvider struct {
	CorpID     string `json:"corp_id"`
	CorpSecret string `json:"corp_secret"`
	AgentID    string `json:"agent_id"`
}

func (p *WeComProvider) Send(ctx context.Context, info *NotificationInfo) error {
	// 获取 access_token
	tokenURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", p.CorpID, p.CorpSecret)
	resp, err := http.Get(tokenURL)
	if err != nil {
		return fmt.Errorf("企业微信获取 token 失败: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}
	if tokenResp.ErrCode != 0 || tokenResp.AccessToken == "" {
		return fmt.Errorf("企业微信获取 token 失败: errcode=%d", tokenResp.ErrCode)
	}

	// 发送消息
	msg := formatMessage(info)
	payload := map[string]interface{}{
		"touser":  "@all",
		"msgtype": "text",
		"agentid": p.AgentID,
		"text":    map[string]string{"content": msg},
	}
	data, _ := json.Marshal(payload)

	sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", tokenResp.AccessToken)
	resp2, err := http.Post(sendURL, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("企业微信发送失败: %w", err)
	}
	defer resp2.Body.Close()
	return nil
}

func (p *WeComProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - 企业微信通知测试"})
}

// CreateProvider 根据类型创建通知提供者
func CreateProvider(providerType, configJSON string) (NotificationProvider, error) {
	switch providerType {
	case "telegram":
		var p TelegramProvider
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("解析 Telegram 配置失败: %w", err)
		}
		return &p, nil
	case "bark":
		var p BarkProvider
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("解析 Bark 配置失败: %w", err)
		}
		return &p, nil
	case "discord":
		var p DiscordProvider
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("解析 Discord 配置失败: %w", err)
		}
		return &p, nil
	case "webhook":
		var p WebhookProvider
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("解析 Webhook 配置失败: %w", err)
		}
		return &p, nil
	case "server_chan":
		var p ServerChanProvider
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("解析 Server酱配置失败: %w", err)
		}
		return &p, nil
	case "wecom":
		var p WeComProvider
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("解析企业微信配置失败: %w", err)
		}
		return &p, nil
	default:
		return nil, fmt.Errorf("不支持的通知类型: %s", providerType)
	}
}
