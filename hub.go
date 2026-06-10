package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	To    string          `json:"to,omitempty"`
	From  string          `json:"from,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
	Peer  *PeerInfo       `json:"peer,omitempty"`
	Peers []PeerInfo      `json:"peers,omitempty"`
	Error string          `json:"error,omitempty"`
}

type PeerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Peer struct {
	ID        string
	Name      string
	conn      *wsConn
	sendCh    chan []byte
	hub       *Hub
	createdAt time.Time
}

type Hub struct {
	mu    sync.RWMutex
	peers map[string]*Peer
}

func newHub() *Hub {
	return &Hub{
		peers: make(map[string]*Peer),
	}
}

func (h *Hub) run() {}

func (h *Hub) register(p *Peer) {
	h.mu.Lock()
	h.peers[p.ID] = p
	peers := h.peerListLocked(p.ID)
	h.mu.Unlock()

	joined := Message{
		Type: "peer-joined",
		Peer: &PeerInfo{ID: p.ID, Name: p.Name},
	}
	h.broadcastExcept(p.ID, joined)

	welcome := Message{
		Type:  "welcome",
		ID:    p.ID,
		Peers: peers,
	}
	data, _ := json.Marshal(welcome)
	p.send(data)
}

func (h *Hub) unregister(p *Peer) {
	h.mu.Lock()
	_, ok := h.peers[p.ID]
	if ok {
		delete(h.peers, p.ID)
	}
	h.mu.Unlock()

	if ok {
		left := Message{
			Type: "peer-left",
			ID:   p.ID,
		}
		h.broadcastExcept(p.ID, left)
	}
}

func (h *Hub) handleMessage(p *Peer, raw []byte) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "hello":
		if msg.Name != "" {
			p.Name = msg.Name
		}
		h.register(p)

	case "signal":
		h.mu.RLock()
		target, ok := h.peers[msg.To]
		h.mu.RUnlock()
		if !ok {
			errMsg, _ := json.Marshal(Message{Type: "error", Error: "peer not found"})
			p.send(errMsg)
			return
		}
		msg.From = p.ID
		data, _ := json.Marshal(msg)
		target.send(data)
	}
}

func (h *Hub) broadcastExcept(excludeID string, msg Message) {
	data, _ := json.Marshal(msg)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, peer := range h.peers {
		if id != excludeID {
			peer.send(data)
		}
	}
}

func (h *Hub) peerListLocked(excludeID string) []PeerInfo {
	list := make([]PeerInfo, 0)
	for id, p := range h.peers {
		if id != excludeID {
			list = append(list, PeerInfo{ID: p.ID, Name: p.Name})
		}
	}
	return list
}

func (p *Peer) send(data []byte) {
	select {
	case p.sendCh <- data:
	default:
	}
}

func newPeerID() string {
	return uuid.New().String()[:8]
}
