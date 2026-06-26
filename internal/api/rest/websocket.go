package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"rus-uhas/internal/hal"
	"rus-uhas/internal/telemetry"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // В production ограничить доменами
	},
}

// WebSocketMessage - сообщение для WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`       // "state", "alert", "log"
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client представляет WebSocket подключение
type Client struct {
	hub        *WebSocketHub
	conn       *websocket.Conn
	send       chan []byte
	userID     string
	mu         sync.Mutex
}

// WebSocketHub управляет WebSocket подключениями
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	logger     *telemetry.Logger
	generator  hal.Generator
	
	mu sync.RWMutex
}

// NewWebSocketHub создает новый hub
func NewWebSocketHub(logger *telemetry.Logger, generator hal.Generator) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		generator:  generator,
	}
}

// Run запускает hub
func (h *WebSocketHub) Run(ctx context.Context) {
	// Запускаем горутину для отправки состояния генератора
	go h.stateBroadcaster(ctx)
	
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("WebSocket hub остановлен")
			return
			
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Info("WebSocket клиент подключен",
				"user_id", client.userID,
				"total_clients", len(h.clients))
			
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("WebSocket клиент отключен",
				"user_id", client.userID,
				"total_clients", len(h.clients))
			
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Клиент не успевает читать - отключаем
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// stateBroadcaster периодически отправляет состояние генератора
func (h *WebSocketHub) stateBroadcaster(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // 10 Hz
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
			
		case <-ticker.C:
			state, err := h.generator.GetState(ctx)
			if err != nil {
				h.logger.Warn("Ошибка получения состояния для WebSocket", "error", err)
				continue
			}
			
			msg := WebSocketMessage{
				Type:      "state",
				Timestamp: time.Now(),
				Payload: map[string]interface{}{
					"is_firing":       state.IsFiring,
					"power_watts":     state.PowerWatts,
					"frequency_hz":    state.FrequencyHz,
					"tip_temp_c":      state.TipTempC,
					"impedance_ohms":  state.ImpedanceOhms,
					"aspiration_bar":  state.AspirationBar,
					"irrigation_ml":   state.IrrigationMl,
				},
			}
			
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			
			h.broadcast <- data
		}
	}
}

// BroadcastAlert отправляет алерт всем подключенным клиентам
func (h *WebSocketHub) BroadcastAlert(severity, message string) {
	msg := WebSocketMessage{
		Type:      "alert",
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"severity": severity,
			"message":  message,
		},
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	
	h.broadcast <- data
}

// BroadcastLog отправляет лог всем подключенным клиентам
func (h *WebSocketHub) BroadcastLog(level, message string, fields map[string]interface{}) {
	msg := WebSocketMessage{
		Type:      "log",
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"level":   level,
			"message": message,
			"fields":  fields,
		},
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	
	h.broadcast <- data
}

// wsHandler обрабатывает WebSocket подключения
func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем аутентификацию
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Валидируем токен (упрощенно)
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	
	claims, err := s.auth.ValidateToken(parts[1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	
	// Upgrader WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Ошибка WebSocket upgrade", "error", err)
		return
	}
	
	// Создаем клиента
	client := &Client{
		hub:    s.wsHub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: claims.UserID,
	}
	
	// Регистрируем клиента
	s.wsHub.register <- client
	
	// Запускаем горутину для чтения сообщений от клиента
	go client.readPump()
	
	// Запускаем горутину для отправки сообщений клиенту
	go client.writePump()
}

// readPump читает сообщения от клиента
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Warn("Ошибка чтения WebSocket", "error", err)
			}
			break
		}
		
		// Обрабатываем команды от клиента
		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		
		// TODO: Обработка команд (например, acknowledge alert)
		c.hub.logger.Debug("Получено сообщение от клиента",
			"user_id", c.userID,
			"type", msg.Type)
	}
}

// writePump отправляет сообщения клиенту
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second) // Ping каждые 30 секунд
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
			
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
