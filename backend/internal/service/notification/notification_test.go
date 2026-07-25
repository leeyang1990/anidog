package notification

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCreateProvider_AllTypes(t *testing.T) {
	types := []string{"telegram", "bark", "discord", "webhook", "server_chan", "wecom"}
	configs := map[string]string{
		"telegram":    `{"bot_token":"123","chat_id":"456"}`,
		"bark":        `{"server_url":"https://bark.example.com","device_key":"abc"}`,
		"discord":     `{"webhook_url":"https://discord.com/webhook"}`,
		"webhook":     `{"url":"https://example.com/hook"}`,
		"server_chan": `{"send_key":"sct123"}`,
		"wecom":       `{"corp_id":"id","corp_secret":"sec","agent_id":"1"}`,
	}

	for _, typ := range types {
		p, err := CreateProvider(typ, configs[typ])
		if err != nil {
			t.Errorf("CreateProvider(%q) failed: %v", typ, err)
		}
		if p == nil {
			t.Errorf("CreateProvider(%q) returned nil", typ)
		}
	}
}

func TestCreateProvider_InvalidType(t *testing.T) {
	_, err := CreateProvider("unknown", "{}")
	if err == nil {
		t.Error("unknown type should return error")
	}
}

func TestCreateProvider_InvalidJSON(t *testing.T) {
	_, err := CreateProvider("telegram", "not-json")
	if err == nil {
		t.Error("invalid JSON should return error")
	}
}

func TestCreateProvider_InjectsHTTPClient(t *testing.T) {
	client := &http.Client{}
	provider, err := CreateProvider("telegram", `{"token":"123","chat_id":"456"}`, client)
	if err != nil {
		t.Fatal(err)
	}
	telegram := provider.(*TelegramProvider)
	if telegram.httpClient != client {
		t.Fatal("Telegram provider 未使用注入的 HTTP client")
	}
}

func TestTelegramSend_RedactsTokenFromNetworkError(t *testing.T) {
	const token = "secret-bot-token"
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("request failed for " + req.URL.String())
	})}
	provider := &TelegramProvider{
		TokenAlias: token,
		ChatID:     "456",
		httpClient: client,
	}

	err := provider.Send(context.Background(), &NotificationInfo{Message: "test"})
	if err == nil {
		t.Fatal("期望网络错误")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("错误信息泄漏 Telegram token: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("错误信息缺少脱敏标记: %v", err)
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		info NotificationInfo
		want string
	}{
		{NotificationInfo{Message: "自定义消息"}, "自定义消息"},
		{NotificationInfo{OfficialTitle: "芙莉莲", Season: 1, Episode: 5}, "芙莉莲 S01E05 已更新"},
		{NotificationInfo{}, "AniDog通知"},
	}
	for _, tt := range tests {
		got := formatMessage(&tt.info)
		if got != tt.want {
			t.Errorf("formatMessage() = %q; want %q", got, tt.want)
		}
	}
}
