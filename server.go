package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingPeriod = 5 * time.Second
	pongWait   = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type SystemSettings struct {
	SystemName    string `json:"systemName"`
	AlertCooldown int    `json:"alertCooldown"` // seconds
	MaxLogEntries int    `json:"maxLogEntries"`
}

type SensorState struct {
	ID           string `json:"id"`
	AssignedRoom string `json:"assignedRoom"`
	Status       string `json:"status"`
}

type SystemState struct {
	IsArmed           bool           `json:"isArmed"`
	Sensors           []SensorState  `json:"sensors"`
	TotalMotionEvents int            `json:"totalMotionEvents"`
	TotalAlerts       int            `json:"totalAlerts"`
	EventsByRoom      map[string]int `json:"eventsByRoom"`
	HourlyActivity    [24]int        `json:"hourlyActivity"`
	ArmedDuration     int64          `json:"armedDuration"` // in seconds
	Settings          SystemSettings `json:"settings"`
}

type ClientCommand struct {
	Type         string         `json:"type"` // "toggle_arm", "assign_room", "update_settings", "reset_analytics"
	SensorID     string         `json:"sensorId,omitempty"`
	AssignedRoom string         `json:"assignedRoom,omitempty"`
	Settings     *SystemSettings `json:"settings,omitempty"`
}

type Client struct {
	conn   *websocket.Conn
	isUser bool
	room   string
	send   chan []byte
}

type Hub struct {
	clients           map[*Client]bool
	sensors           map[string]*SensorState
	isArmed           bool
	sensorCount       int
	totalMotionEvents int
	totalAlerts       int
	eventsByRoom      map[string]int
	hourlyActivity    [24]int
	armedSince        time.Time
	totalArmedSeconds int64
	settings          SystemSettings
	lastAlertTimes    map[string]time.Time
	broadcast         chan []byte
	register          chan *Client
	unregister        chan *Client
	mu                sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		sensors:      make(map[string]*SensorState),
		eventsByRoom: make(map[string]int),
		lastAlertTimes: make(map[string]time.Time),
		broadcast:    make(chan []byte, 1024),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		isArmed:      false,
		settings: SystemSettings{
			SystemName:    "Omni Sense",
			AlertCooldown: 5,
			MaxLogEntries: 10,
		},
	}
}

func (h *Hub) run() {
	log.Printf("%s Hub main loop started", h.settings.SystemName)
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if !client.isUser {
				id := client.room
				if id == "" {
					h.sensorCount++
					id = fmt.Sprintf("sensor_%d", h.sensorCount)
				}
				client.room = id
				if _, exists := h.sensors[id]; !exists {
					h.sensors[id] = &SensorState{ID: id, AssignedRoom: "", Status: "!motion"}
					if len(id) <= 7 || id[:7] != "sensor_" {
						h.sensors[id].AssignedRoom = id
					}
				} else {
					h.sensors[id].Status = "!motion"
				}
				log.Printf("%s: Sensor connected -> %s", h.settings.SystemName, id)
				h.broadcastSystemState()
			} else {
				h.sendSystemStateToClient(client)
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				if !client.isUser && client.room != "" {
					delete(h.sensors, client.room)
					log.Printf("%s: Sensor disconnected -> %s", h.settings.SystemName, client.room)
					h.broadcastSystemState()
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			h.sendToAllUsers(message)
			h.mu.Unlock()
		}
	}
}

func (h *Hub) getArmedDuration() int64 {
	duration := h.totalArmedSeconds
	if h.isArmed {
		duration += int64(time.Since(h.armedSince).Seconds())
	}
	return duration
}

func (h *Hub) sendSystemStateToClient(c *Client) {
	state := SystemState{
		IsArmed:           h.isArmed,
		Sensors:           make([]SensorState, 0, len(h.sensors)),
		TotalMotionEvents: h.totalMotionEvents,
		TotalAlerts:       h.totalAlerts,
		EventsByRoom:      h.eventsByRoom,
		HourlyActivity:    h.hourlyActivity,
		ArmedDuration:     h.getArmedDuration(),
		Settings:          h.settings,
	}
	for _, s := range h.sensors {
		state.Sensors = append(state.Sensors, *s)
	}
	msg, _ := json.Marshal(map[string]interface{}{"type": "system_state", "data": state})
	select {
	case c.send <- msg:
	default:
	}
}

func (h *Hub) broadcastSystemState() {
	state := SystemState{
		IsArmed:           h.isArmed,
		Sensors:           make([]SensorState, 0, len(h.sensors)),
		TotalMotionEvents: h.totalMotionEvents,
		TotalAlerts:       h.totalAlerts,
		EventsByRoom:      h.eventsByRoom,
		HourlyActivity:    h.hourlyActivity,
		ArmedDuration:     h.getArmedDuration(),
		Settings:          h.settings,
	}
	for _, s := range h.sensors {
		state.Sensors = append(state.Sensors, *s)
	}
	msg, _ := json.Marshal(map[string]interface{}{"type": "system_state", "data": state})
	h.sendToAllUsers(msg)
}

func (h *Hub) sendToAllUsers(message []byte) {
	for client := range h.clients {
		if client.isUser {
			select {
			case client.send <- message:
			default:
				client.conn.Close()
				delete(h.clients, client)
			}
		}
	}
}

func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &Client{conn: conn, isUser: r.URL.Query().Get("type") == "user", room: r.URL.Query().Get("room"), send: make(chan []byte, 256)}
	h.register <- client
	go client.writePump()
	go h.readPump(client)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() { ticker.Stop(); c.conn.Close() }()
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

func (h *Hub) readPump(c *Client) {
	defer func() { h.unregister <- c }()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		if c.isUser {
			var cmd ClientCommand
			if err := json.Unmarshal(message, &cmd); err == nil {
				h.mu.Lock()
				switch cmd.Type {
				case "toggle_arm":
					if !h.isArmed {
						h.armedSince = time.Now()
					} else {
						h.totalArmedSeconds += int64(time.Since(h.armedSince).Seconds())
					}
					h.isArmed = !h.isArmed
					h.broadcastSystemState()
				case "assign_room":
					if s, ok := h.sensors[cmd.SensorID]; ok {
						s.AssignedRoom = cmd.AssignedRoom
						h.broadcastSystemState()
					}
				case "update_settings":
					if cmd.Settings != nil {
						h.settings = *cmd.Settings
						log.Printf("Settings updated: %+v", h.settings)
						h.broadcastSystemState()
					}
				case "reset_analytics":
					h.totalMotionEvents = 0
					h.totalAlerts = 0
					h.eventsByRoom = make(map[string]int)
					h.hourlyActivity = [24]int{}
					h.totalArmedSeconds = 0
					if h.isArmed {
						h.armedSince = time.Now()
					}
					log.Println("Analytics reset")
					h.broadcastSystemState()
				}
				h.mu.Unlock()
			}
			continue
		}
		msgStr := string(message)
		if msgStr == "motion" || msgStr == "!motion" {
			h.mu.Lock()
			if s, ok := h.sensors[c.room]; ok {
				s.Status = msgStr
				if msgStr == "motion" {
					h.totalMotionEvents++
					h.hourlyActivity[time.Now().Hour()]++
					roomName := s.AssignedRoom
					if roomName == "" { roomName = s.ID }
					h.eventsByRoom[roomName]++
					
					// Handle alert cooldown
					if h.isArmed {
						lastAlert := h.lastAlertTimes[s.ID]
						if time.Since(lastAlert) > time.Duration(h.settings.AlertCooldown)*time.Second {
							h.totalAlerts++
							h.lastAlertTimes[s.ID] = time.Now()
							log.Printf("ALERT! Motion detected in %s while ARMED!", roomName)
						}
					}
				}
				h.broadcastSystemState()
			}
			h.mu.Unlock()
		}
	}
}

func main() {
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			hub.handleWebSocket(w, r)
			return
		}
		if r.URL.Path != "/" {
			http.FileServer(http.Dir(".")).ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, "index.html")
	})
	log.Println("Omni Sense Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}
