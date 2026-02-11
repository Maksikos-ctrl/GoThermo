package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var globalHub *Hub

type Client struct {
	ID       string
	Username string
	Conn     *websocket.Conn
	Hub      *Hub
	Send     chan []byte
	Channels []string
}

type Hub struct {
	clients    map[string]*Client // Username -> Client
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type StatusUpdate struct {
	Username string `json:"username"`
	Status   string `json:"status"` // online, away, offline
}

type ChannelMessage struct {
	Channel string  `json:"channel"`
	Message Message `json:"message"`
}

func NewHub() *Hub {
	hub := &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	// ✅ ДОБАВЬТЕ ЭТУ СТРОКУ
	globalHub = hub

	return hub
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client.Username] = client
			h.mutex.Unlock()
			log.Printf("✓ WebSocket клиент подключен: %s", client.Username)

			welcomeMsg := WSMessage{
				Type: "connected",
				Payload: map[string]string{
					"message": "Connected to WebSocket",
				},
			}
			data, _ := json.Marshal(welcomeMsg)
			client.Send <- data

			users := userManager.GetAllUsers()
			usersListMsg := WSMessage{
				Type:    "users_list",
				Payload: users,
			}
			usersData, _ := json.Marshal(usersListMsg)
			client.Send <- usersData

			statusMsg := WSMessage{
				Type: "status_update",
				Payload: StatusUpdate{
					Username: client.Username,
					Status:   "online",
				},
			}
			statusData, _ := json.Marshal(statusMsg)
			h.broadcastMessage(statusData, client.Username)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client.Username]; ok {
				delete(h.clients, client.Username)
				close(client.Send)
				log.Printf("✗ WebSocket клиент отключен: %s", client.Username)

				statusMsg := WSMessage{
					Type: "status_update",
					Payload: StatusUpdate{
						Username: client.Username,
						Status:   "offline",
					},
				}
				data, _ := json.Marshal(statusMsg)
				h.broadcastMessage(data, client.Username)
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.RLock()
			for _, client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client.Username)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

func (h *Hub) BroadcastToChannel(channel string, msg Message) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	channelMsg := WSMessage{
		Type: "channel_message",
		Payload: ChannelMessage{
			Channel: channel,
			Message: msg,
		},
	}

	data, err := json.Marshal(channelMsg)
	if err != nil {
		log.Printf("Ошибка маршалинга сообщения для канала: %v", err)
		return
	}

	log.Printf("📢 Вещаем в канал #%s: %s", channel, truncateText(msg.Text, 50))

	sentCount := 0
	for username, client := range h.clients {

		if len(client.Channels) == 0 || contains(client.Channels, channel) {
			select {
			case client.Send <- data:
				sentCount++
			default:
				close(client.Send)
				delete(h.clients, username)
			}
		}
	}

	log.Printf("✓ Сообщение отправлено %d клиентам в канале #%s", sentCount, channel)
}

func (h *Hub) AddChannelToClient(username, channel string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if client, exists := h.clients[username]; exists {
		if !contains(client.Channels, channel) {
			client.Channels = append(client.Channels, channel)
			log.Printf("✅ Клиент %s подписан на канал #%s", username, channel)
		}
	}
}

func (h *Hub) RemoveChannelFromClient(username, channel string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if client, exists := h.clients[username]; exists {
		for i, ch := range client.Channels {
			if ch == channel {
				client.Channels = append(client.Channels[:i], client.Channels[i+1:]...)
				log.Printf("❌ Клиент %s отписан от канала #%s", username, channel)
				break
			}
		}
	}
}

func (h *Hub) BroadcastStatusUpdate(username, status string) {
	statusMsg := WSMessage{
		Type: "status_update",
		Payload: StatusUpdate{
			Username: username,
			Status:   status,
		},
	}

	data, err := json.Marshal(statusMsg)
	if err != nil {
		log.Printf("Ошибка маршалинга статуса: %v", err)
		return
	}

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for _, client := range h.clients {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.clients, client.Username)
		}
	}

	log.Printf("📢 Статус обновлен: %s -> %s (отправлено %d клиентам)",
		username, status, len(h.clients))
}

func (h *Hub) BroadcastNewMessage(channel string, message Message) {
	h.BroadcastToChannel(channel, message)
}

func (h *Hub) broadcastMessage(message []byte, excludeUsername string) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for username, client := range h.clients {
		if username == excludeUsername {
			continue
		}

		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(h.clients, username)
		}
	}
}

func (h *Hub) HandleClient(conn *websocket.Conn, username string) {
	client := &Client{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Username: username,
		Conn:     conn,
		Hub:      h,
		Send:     make(chan []byte, 256),
		Channels: []string{},
	}

	h.register <- client

	go h.autoSubscribeChannels(username)

	go client.readPump()

	go client.writePump()
}

func (h *Hub) autoSubscribeChannels(username string) {

	time.Sleep(100 * time.Millisecond)

	channels, err := GetAllChannels()
	if err != nil {
		log.Printf("Ошибка получения каналов для автоматической подписки: %v", err)
		return
	}

	for _, channel := range channels {
		if !channel.IsPrivate {
			h.AddChannelToClient(username, channel.Name)
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024) // 512KB
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Ошибка чтения WebSocket: %v", err)
			}
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err == nil {
			c.handleMessage(wsMsg)
		}
	}
}

func (c *Client) handleMessage(msg WSMessage) {
	switch msg.Type {
	case "ping":
		pongMsg := WSMessage{Type: "pong"}
		pongData, _ := json.Marshal(pongMsg)
		c.Send <- pongData

	case "subscribe_channel":
		if channel, ok := msg.Payload.(string); ok {
			c.Hub.AddChannelToClient(c.Username, channel)
			response := WSMessage{
				Type: "subscribed",
				Payload: map[string]interface{}{
					"channel": channel,
					"success": true,
				},
			}
			data, _ := json.Marshal(response)
			c.Send <- data
		}

	case "unsubscribe_channel":
		if channel, ok := msg.Payload.(string); ok {
			c.Hub.RemoveChannelFromClient(c.Username, channel)
		}

	case "status_change":

		payloadBytes, err := json.Marshal(msg.Payload)
		if err != nil {
			log.Printf("Ошибка маршалинга payload: %v", err)
			return
		}

		var statusPayload struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(payloadBytes, &statusPayload); err != nil {
			log.Printf("Ошибка парсинга status_change: %v", err)
			return
		}

		if userManager.UpdateUserStatus(c.Username, statusPayload.Status) {

			c.Hub.BroadcastStatusUpdate(c.Username, statusPayload.Status)
			log.Printf("🔄 Статус изменен через WebSocket: %s -> %s",
				c.Username, statusPayload.Status)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {

				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
