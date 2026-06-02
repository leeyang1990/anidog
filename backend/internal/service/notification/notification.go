package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	// CoverURL 番剧封面图 URL（可空）。支持的 provider（telegram/discord/bark）
	// 会带图片发送；不支持图片的 provider 会忽略此字段。
	CoverURL string
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

// TelegramProvider Telegram 通知。
//
// 字段 JSON tag 同时支持两套键名：
//   - 简短键 (token / chat_id)：前端表单实际使用
//   - 长键   (bot_token)：旧文档/代码使用
//
// JSON Unmarshal 会按字段顺序赋值，所以两个 BotToken 字段都存在时谁先出现在 JSON
// 谁生效。这里把 BotToken 用 `json:"bot_token,omitempty"`，TokenAlias 用 `json:"token"`，
// 反序列化时 token 先到，再用 BotToken（如果 token 为空）兜底。
type TelegramProvider struct {
	BotToken   string `json:"bot_token,omitempty"`
	TokenAlias string `json:"token,omitempty"` // 前端简短键
	ChatID     string `json:"chat_id"`
}

// resolveTelegram 取出真正要用的 token，兼容前端简短键。
func (p *TelegramProvider) resolveToken() string {
	if p.TokenAlias != "" {
		return p.TokenAlias
	}
	return p.BotToken
}

func (p *TelegramProvider) Send(ctx context.Context, info *NotificationInfo) error {
	tok := p.resolveToken()
	if tok == "" {
		return fmt.Errorf("Telegram bot token 未配置（config 里需要 token 或 bot_token 字段）")
	}
	if p.ChatID == "" {
		return fmt.Errorf("Telegram chat_id 未配置（先发一条消息给 bot 然后用 @userinfobot 拿到自己 id）")
	}
	msg := formatMessage(info)

	// 有封面图就走 sendPhoto，文本当 caption；没有就走 sendMessage 纯文本。
	// sendPhoto 直接传 photo URL（Telegram 服务器会自己去拉图），不需要本地下载。
	// caption 限长 1024 字符，updateNotification 文本远短于此，不需要截断。
	var apiURL string
	form := url.Values{"chat_id": {p.ChatID}}
	if info != nil && info.CoverURL != "" {
		apiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", tok)
		form.Set("photo", info.CoverURL)
		form.Set("caption", msg)
	} else {
		apiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tok)
		form.Set("text", msg)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Telegram 发送失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// 把 Telegram 的人话错误描述带出来（如 "Bad Request: chat not found"
		// 或 "wrong file identifier/HTTP URL specified" 等图片下载失败原因）
		var tgErr struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(body, &tgErr); err == nil && tgErr.Description != "" {
			// 图片 URL 不可达时，降级到纯文本重发一次（避免一张坏图把整条通知吞掉）
			if info != nil && info.CoverURL != "" {
				fallback := *info
				fallback.CoverURL = ""
				return p.Send(ctx, &fallback)
			}
			return fmt.Errorf("Telegram API %d: %s", resp.StatusCode, tgErr.Description)
		}
		return fmt.Errorf("Telegram API 返回 %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}
	return nil
}

func (p *TelegramProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{
		Message:  "AniDog - Telegram 通知测试",
		CoverURL: "https://lain.bgm.tv/pic/icon/l/000/00/00/0.jpg", // 占位图，验证封面通道
	})
}

// truncateStr 把过长字符串截短，避免错误信息过长污染日志/UI
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// BarkProvider Bark 通知
//
// 同样兼容前端简短键 (url/key) 和长键 (server_url/device_key)。
type BarkProvider struct {
	ServerURL string `json:"server_url,omitempty"`
	URLAlias  string `json:"url,omitempty"`
	DeviceKey string `json:"device_key,omitempty"`
	KeyAlias  string `json:"key,omitempty"`
}

func (p *BarkProvider) resolveServer() string {
	if p.URLAlias != "" {
		return p.URLAlias
	}
	return p.ServerURL
}
func (p *BarkProvider) resolveKey() string {
	if p.KeyAlias != "" {
		return p.KeyAlias
	}
	return p.DeviceKey
}

func (p *BarkProvider) Send(ctx context.Context, info *NotificationInfo) error {
	server := strings.TrimRight(p.resolveServer(), "/")
	key := p.resolveKey()
	if server == "" {
		return fmt.Errorf("Bark URL 未配置")
	}
	if key == "" {
		return fmt.Errorf("Bark device key 未配置")
	}
	msg := formatMessage(info)
	payload := map[string]string{"device_key": key, "title": "AniDog", "body": msg}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(server+"/"+key, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("Bark 发送失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bark API 返回 %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}
	return nil
}

func (p *BarkProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Bark 通知测试"})
}

// DiscordProvider Discord 通知
type DiscordProvider struct {
	WebhookURL string `json:"webhook_url,omitempty"`
	URLAlias   string `json:"url,omitempty"`
}

func (p *DiscordProvider) resolveURL() string {
	if p.URLAlias != "" {
		return p.URLAlias
	}
	return p.WebhookURL
}

func (p *DiscordProvider) Send(ctx context.Context, info *NotificationInfo) error {
	u := p.resolveURL()
	if u == "" {
		return fmt.Errorf("Discord webhook URL 未配置")
	}
	msg := formatMessage(info)
	payload := map[string]string{"content": msg}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(u, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("Discord 发送失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Discord API 返回 %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}
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
	SendKey      string `json:"send_key,omitempty"`
	SendKeyAlias string `json:"sendkey,omitempty"`
}

func (p *ServerChanProvider) resolveKey() string {
	if p.SendKeyAlias != "" {
		return p.SendKeyAlias
	}
	return p.SendKey
}

func (p *ServerChanProvider) Send(ctx context.Context, info *NotificationInfo) error {
	key := p.resolveKey()
	if key == "" {
		return fmt.Errorf("Server酱 sendkey 未配置")
	}
	msg := formatMessage(info)
	apiURL := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", key)
	form := url.Values{"title": {"AniDog"}, "desp": {msg}}

	resp, err := http.PostForm(apiURL, form)
	if err != nil {
		return fmt.Errorf("Server酱发送失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Server酱 API 返回 %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}
	return nil
}

func (p *ServerChanProvider) Test(ctx context.Context) error {
	return p.Send(ctx, &NotificationInfo{Message: "AniDog - Server酱通知测试"})
}

// WeComProvider 企业微信通知
type WeComProvider struct {
	CorpID         string `json:"corp_id,omitempty"`
	CorpIDAlias    string `json:"corpid,omitempty"`
	CorpSecret     string `json:"corp_secret,omitempty"`
	CorpSecretAlt  string `json:"corpsecret,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentIDAlias   string `json:"agentid,omitempty"`
}

func (p *WeComProvider) resolveCorpID() string {
	if p.CorpIDAlias != "" {
		return p.CorpIDAlias
	}
	return p.CorpID
}
func (p *WeComProvider) resolveCorpSecret() string {
	if p.CorpSecretAlt != "" {
		return p.CorpSecretAlt
	}
	return p.CorpSecret
}
func (p *WeComProvider) resolveAgentID() string {
	if p.AgentIDAlias != "" {
		return p.AgentIDAlias
	}
	return p.AgentID
}

func (p *WeComProvider) Send(ctx context.Context, info *NotificationInfo) error {
	corpID := p.resolveCorpID()
	corpSecret := p.resolveCorpSecret()
	agentID := p.resolveAgentID()
	if corpID == "" || corpSecret == "" || agentID == "" {
		return fmt.Errorf("企业微信 corp_id/corp_secret/agent_id 未配置完整")
	}
	// 获取 access_token
	tokenURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpID, corpSecret)
	resp, err := http.Get(tokenURL)
	if err != nil {
		return fmt.Errorf("企业微信获取 token 失败: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}
	if tokenResp.ErrCode != 0 || tokenResp.AccessToken == "" {
		return fmt.Errorf("企业微信获取 token 失败: errcode=%d %s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	// 发送消息
	msg := formatMessage(info)
	payload := map[string]interface{}{
		"touser":  "@all",
		"msgtype": "text",
		"agentid": agentID,
		"text":    map[string]string{"content": msg},
	}
	data, _ := json.Marshal(payload)

	sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", tokenResp.AccessToken)
	resp2, err := http.Post(sendURL, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("企业微信发送失败: %w", err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	var sendResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &sendResp); err == nil && sendResp.ErrCode != 0 {
		return fmt.Errorf("企业微信发送失败: errcode=%d %s", sendResp.ErrCode, sendResp.ErrMsg)
	}
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
