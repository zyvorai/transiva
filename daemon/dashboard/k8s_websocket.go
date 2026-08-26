// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// K8sWebSocketHub manages WebSocket connections for Kubernetes metrics
type K8sWebSocketHub struct {
	dashboard  *K8sDashboard
	clients    map[*websocket.Conn]bool
	clientsMu  sync.RWMutex
	broadcast  chan []byte
	upgrader   websocket.Upgrader
	maxClients int
}

// NewK8sWebSocketHub creates a new WebSocket hub
func NewK8sWebSocketHub(dashboard *K8sDashboard, maxClients int) *K8sWebSocketHub {
	return &K8sWebSocketHub{
		dashboard: dashboard,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte, 256),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		maxClients: maxClients,
	}
}

// Start starts the WebSocket hub
func (hub *K8sWebSocketHub) Start(ctx context.Context) {
	// Start broadcast goroutine
	go hub.handleBroadcast()

	// Start metrics broadcaster
	go hub.broadcastMetrics(ctx)
}

// handleBroadcast broadcasts messages to all connected clients
func (hub *K8sWebSocketHub) handleBroadcast() {
	for data := range hub.broadcast {
		hub.clientsMu.RLock()
		for client := range hub.clients {
			err := client.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				if cerr := client.Close(); cerr != nil {
					fmt.Printf("WebSocket close error: %v\n", cerr)
				}
				delete(hub.clients, client)
			}
		}
		hub.clientsMu.RUnlock()
	}
}

// broadcastMetrics periodically broadcasts metrics to all clients
func (hub *K8sWebSocketHub) broadcastMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := hub.dashboard.GetMetrics()
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}

			select {
			case hub.broadcast <- data:
			default:
				// Channel full, skip this update
			}
		}
	}
}

// HandleWebSocket handles WebSocket upgrade and client management
func (hub *K8sWebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check client limit
	hub.clientsMu.RLock()
	clientCount := len(hub.clients)
	hub.clientsMu.RUnlock()

	if clientCount >= hub.maxClients {
		http.Error(w, "Too many clients", http.StatusServiceUnavailable)
		return
	}

	// Upgrade connection
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade error: %v\n", err)
		return
	}

	// Register client
	hub.clientsMu.Lock()
	hub.clients[conn] = true
	hub.clientsMu.Unlock()

	// Send initial metrics
	metrics := hub.dashboard.GetMetrics()
	data, _ := json.Marshal(metrics)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		fmt.Printf("WebSocket write error: %v\n", err)
		hub.clientsMu.Lock()
		delete(hub.clients, conn)
		hub.clientsMu.Unlock()
		if cerr := conn.Close(); cerr != nil {
			fmt.Printf("WebSocket close error: %v\n", cerr)
		}
		return
	}

	// Handle client messages
	go hub.handleClient(conn)
}

// handleClient handles individual client connections
func (hub *K8sWebSocketHub) handleClient(conn *websocket.Conn) {
	defer func() {
		hub.clientsMu.Lock()
		delete(hub.clients, conn)
		hub.clientsMu.Unlock()
		if err := conn.Close(); err != nil {
			fmt.Printf("WebSocket close error: %v\n", err)
		}
	}()

	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		fmt.Printf("WebSocket set read deadline error: %v\n", err)
	}

	// Set pong handler to reset read deadline
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Read loop (mostly for detecting disconnects)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// GetClientCount returns the number of connected clients
func (hub *K8sWebSocketHub) GetClientCount() int {
	hub.clientsMu.RLock()
	defer hub.clientsMu.RUnlock()
	return len(hub.clients)
}

// BroadcastUpdate immediately broadcasts an update to all clients
func (hub *K8sWebSocketHub) BroadcastUpdate() {
	metrics := hub.dashboard.GetMetrics()
	data, err := json.Marshal(metrics)
	if err != nil {
		return
	}

	select {
	case hub.broadcast <- data:
	default:
		// Channel full, skip
	}
}
