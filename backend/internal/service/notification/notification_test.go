package notification

import (
	"testing"
)

func TestCreateProvider_AllTypes(t *testing.T) {
	types := []string{"telegram", "bark", "discord", "webhook", "server_chan", "wecom"}
	configs := map[string]string{
		"telegram":    `{"bot_token":"123","chat_id":"456"}`,
		"bark":       `{"server_url":"https://bark.example.com","device_key":"abc"}`,
		"discord":    `{"webhook_url":"https://discord.com/webhook"}`,
		"webhook":    `{"url":"https://example.com/hook"}`,
		"server_chan": `{"send_key":"sct123"}`,
		"wecom":      `{"corp_id":"id","corp_secret":"sec","agent_id":"1"}`,
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

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		info NotificationInfo
		want string
	}{
		{NotificationInfo{Message: "自定义消息"}, "自定义消息"},
		{NotificationInfo{OfficialTitle: "芙莉莲", Season: 1, Episode: 5}, "芙莉莲 S01E05 已更新"},
		{NotificationInfo{}, "御宅追番通知"},
	}
	for _, tt := range tests {
		got := formatMessage(&tt.info)
		if got != tt.want {
			t.Errorf("formatMessage() = %q; want %q", got, tt.want)
		}
	}
}
