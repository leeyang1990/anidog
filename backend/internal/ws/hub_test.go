package ws

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer func() {
		// Hub.Run() blocks forever, so we can't stop it cleanly.
		// Just let it be GC'd after test.
	}()

	client := &Client{
		Hub:      hub,
		Send:     make(chan []byte, 256),
		ClientID: "test-client",
	}

	hub.Register(client)

	// Wait for register to process
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	set, ok := hub.clients["test-client"]
	hub.mu.RUnlock()
	if !ok || len(set) != 1 {
		t.Fatalf("expected 1 client; got set=%v, ok=%v", set, ok)
	}

	hub.Unregister(client)
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	_, ok = hub.clients["test-client"]
	hub.mu.RUnlock()
	if ok {
		t.Error("client should be removed after unregister")
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:      hub,
		Send:     make(chan []byte, 256),
		ClientID: "broadcast-test",
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"test","data":"hello"}`)
	hub.Broadcast(msg)

	select {
	case received := <-client.Send:
		if string(received) != string(msg) {
			t.Errorf("received = %s; want %s", received, msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for broadcast message")
	}
}

func TestHub_BroadcastEvent(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:      hub,
		Send:     make(chan []byte, 256),
		ClientID: "event-test",
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastEvent("download_progress", map[string]interface{}{
		"id":       "torrent_123",
		"progress": 50.0,
	})

	select {
	case msg := <-client.Send:
		var event map[string]interface{}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if event["type"] != "download_progress" {
			t.Errorf("type = %v; want download_progress", event["type"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for broadcast event")
	}
}

func TestHub_SendToClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		Hub:      hub,
		Send:     make(chan []byte, 256),
		ClientID: "direct-test",
	}

	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"direct"}`)
	hub.SendToClient("direct-test", msg)

	select {
	case received := <-client.Send:
		if string(received) != string(msg) {
			t.Errorf("received = %s; want %s", received, msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for direct message")
	}
}

func TestHub_SendToNonexistentClient(t *testing.T) {
	hub := NewHub()
	// Should not panic
	hub.SendToClient("nonexistent", []byte("test"))
}

func TestHub_MultipleClientsSameID(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := &Client{Hub: hub, Send: make(chan []byte, 256), ClientID: "multi"}
	client2 := &Client{Hub: hub, Send: make(chan []byte, 256), ClientID: "multi"}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	set := hub.clients["multi"]
	hub.mu.RUnlock()
	if len(set) != 2 {
		t.Errorf("expected 2 clients; got %d", len(set))
	}

	hub.Broadcast([]byte("test"))
	time.Sleep(50 * time.Millisecond)

	// Both should receive
	select {
	case <-client1.Send:
	default:
		t.Error("client1 should receive broadcast")
	}
	select {
	case <-client2.Send:
	default:
		t.Error("client2 should receive broadcast")
	}
}

func TestHub_DownloadProgress(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{Hub: hub, Send: make(chan []byte, 256), ClientID: "dl-test"}
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastDownloadProgress("torrent_abc", "test.mkv", 75.5, 0)

	select {
	case msg := <-client.Send:
		var event map[string]interface{}
		json.Unmarshal(msg, &event)
		if event["type"] != "download_progress" {
			t.Errorf("type = %v; want download_progress", event["type"])
		}
		data := event["data"].(map[string]interface{})
		if data["progress"] != 75.5 {
			t.Errorf("progress = %v; want 75.5", data["progress"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout")
	}
}

func TestHub_DownloadComplete(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{Hub: hub, Send: make(chan []byte, 256), ClientID: "dl-complete"}
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	hub.BroadcastDownloadComplete("torrent_xyz", "done.mkv")

	select {
	case msg := <-client.Send:
		var event map[string]interface{}
		json.Unmarshal(msg, &event)
		if event["type"] != "download_complete" {
			t.Errorf("type = %v; want download_complete", event["type"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout")
	}
}
