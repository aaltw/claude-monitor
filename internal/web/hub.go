package web

import "sync"

type Hub struct {
	mu         sync.RWMutex
	clients    map[chan []byte]struct{}
	register   chan chan []byte
	unregister chan chan []byte
	broadcast  chan []byte
	stop       chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan []byte]struct{}),
		register:   make(chan chan []byte),
		unregister: make(chan chan []byte),
		broadcast:  make(chan []byte, 64),
		stop:       make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case ch := <-h.register:
			h.mu.Lock()
			h.clients[ch] = struct{}{}
			h.mu.Unlock()
		case ch := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, ch)
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- msg:
				default:
				}
			}
			h.mu.RUnlock()
		case <-h.stop:
			return
		}
	}
}

func (h *Hub) Register(ch chan []byte)   { h.register <- ch }
func (h *Hub) Unregister(ch chan []byte) { h.unregister <- ch }
func (h *Hub) Broadcast(msg []byte)      { h.broadcast <- msg }

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Stop() { close(h.stop) }
