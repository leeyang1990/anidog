package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	ClientID string
}

type Hub struct {
	clients    map[string]map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.ClientID] == nil {
				h.clients[client.ClientID] = make(map[*Client]bool)
			}
			h.clients[client.ClientID][client] = true
			h.mu.Unlock()
			zap.L().Info("WebSocket 客户端连接", zap.String("client_id", client.ClientID))

		case client := <-h.unregister:
			h.mu.Lock()
			if set, ok := h.clients[client.ClientID]; ok {
				delete(set, client)
				if len(set) == 0 {
					delete(h.clients, client.ClientID)
				}
			}
			h.mu.Unlock()
			close(client.Send)
			zap.L().Info("WebSocket 客户端断开", zap.String("client_id", client.ClientID))

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, set := range h.clients {
				for client := range set {
					select {
					case client.Send <- message:
					default:
						close(client.Send)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

func (h *Hub) SendToClient(clientID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if set, ok := h.clients[clientID]; ok {
		for client := range set {
			select {
			case client.Send <- message:
			default:
				zap.L().Warn("发送消息到客户端失败，缓冲区已满", zap.String("client_id", clientID))
			}
		}
	}
}

// BroadcastEvent sends a typed event to all connected clients.
func (h *Hub) BroadcastEvent(eventType string, data interface{}) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	h.Broadcast(msg)
}

// Convenience methods using BroadcastEvent.

func (h *Hub) BroadcastDownloadProgress(torrentID, fileName string, progress float64, animeID uint) {
	h.BroadcastEvent("download_progress", map[string]interface{}{
		"id":        torrentID,
		"file_name": fileName,
		"progress":  progress,
		"anime_id":  animeID,
	})
}

func (h *Hub) BroadcastDownloadComplete(torrentID, fileName string) {
	h.BroadcastEvent("download_complete", map[string]interface{}{
		"id":        torrentID,
		"file_name": fileName,
	})
}

func (h *Hub) BroadcastNewAnimeFound(animeInfo map[string]interface{}) {
	h.BroadcastEvent("new_anime", animeInfo)
}
