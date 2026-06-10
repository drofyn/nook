package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	Type       string          `json:"type"`
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name,omitempty"`
	To         string          `json:"to,omitempty"`
	From       string          `json:"from,omitempty"`
	Text       string          `json:"text,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Peer       *PeerInfo       `json:"peer,omitempty"`
	Peers      []PeerInfo      `json:"peers,omitempty"`
	Clipboard  *ClipboardItem  `json:"clipboard,omitempty"`
	Clipboards []ClipboardItem `json:"clipboards,omitempty"`
	FileOffer  *FileOffer      `json:"fileOffer,omitempty"`
	FileOffers []FileOffer     `json:"fileOffers,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type PeerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ClipboardItem struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	FromName  string `json:"fromName"`
	Text      string `json:"text"`
	CreatedAt int64  `json:"createdAt"`
}

type FileOffer struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	FromName  string `json:"fromName"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	MIME      string `json:"mime"`
	CreatedAt int64  `json:"createdAt"`
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
	mu         sync.RWMutex
	peers      map[string]*Peer
	clipboards []ClipboardItem
	fileOffers map[string]FileOffer
}

const maxClipboardItems = 50
const maxClipboardTextBytes = 64 * 1024
const maxFileOffers = 100

func newHub() *Hub {
	return &Hub{
		peers:      make(map[string]*Peer),
		clipboards: make([]ClipboardItem, 0),
		fileOffers: make(map[string]FileOffer),
	}
}

func (h *Hub) run() {}

func (h *Hub) register(p *Peer) {
	h.mu.Lock()
	h.peers[p.ID] = p
	peers := h.peerListLocked(p.ID)
	clipboards := append([]ClipboardItem(nil), h.clipboards...)
	fileOffers := h.fileOfferListLocked()
	h.mu.Unlock()

	joined := Message{
		Type: "peer-joined",
		Peer: &PeerInfo{ID: p.ID, Name: p.Name},
	}
	h.broadcastExcept(p.ID, joined)

	welcome := Message{
		Type:       "welcome",
		ID:         p.ID,
		Peers:      peers,
		Clipboards: clipboards,
		FileOffers: fileOffers,
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
	removed := h.removeFileOffersByPeerLocked(p.ID)
	h.mu.Unlock()

	if ok {
		left := Message{
			Type: "peer-left",
			ID:   p.ID,
		}
		h.broadcastExcept(p.ID, left)
		for _, id := range removed {
			h.broadcast(Message{Type: "file-board-removed", ID: id})
		}
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

	case "clipboard-add":
		if msg.Text == "" || len([]byte(msg.Text)) > maxClipboardTextBytes {
			return
		}
		item := ClipboardItem{
			ID:        newPeerID(),
			From:      p.ID,
			FromName:  p.Name,
			Text:      msg.Text,
			CreatedAt: time.Now().UnixMilli(),
		}
		h.mu.Lock()
		h.clipboards = append([]ClipboardItem{item}, h.clipboards...)
		if len(h.clipboards) > maxClipboardItems {
			h.clipboards = h.clipboards[:maxClipboardItems]
		}
		h.mu.Unlock()
		h.broadcast(Message{Type: "clipboard-added", Clipboard: &item})

	case "file-board-add":
		if msg.Name == "" || msg.FileOffer == nil || msg.FileOffer.Size < 0 {
			return
		}
		offer := FileOffer{
			ID:        msg.ID,
			From:      p.ID,
			FromName:  p.Name,
			Name:      msg.Name,
			Size:      msg.FileOffer.Size,
			MIME:      msg.FileOffer.MIME,
			CreatedAt: time.Now().UnixMilli(),
		}
		if offer.ID == "" {
			offer.ID = newPeerID()
		}
		h.mu.Lock()
		if existing, ok := h.fileOffers[offer.ID]; ok && existing.From != p.ID {
			offer.ID = newPeerID()
		}
		if len(h.fileOffers) >= maxFileOffers {
			h.removeOldestFileOfferLocked()
		}
		h.fileOffers[offer.ID] = offer
		h.mu.Unlock()
		h.broadcast(Message{Type: "file-board-added", FileOffer: &offer})

	case "file-board-remove":
		h.mu.Lock()
		offer, ok := h.fileOffers[msg.ID]
		if ok && offer.From == p.ID {
			delete(h.fileOffers, msg.ID)
		}
		h.mu.Unlock()
		if ok && offer.From == p.ID {
			h.broadcast(Message{Type: "file-board-removed", ID: msg.ID})
		}

	case "file-board-request":
		h.mu.RLock()
		offer, ok := h.fileOffers[msg.ID]
		target, targetOK := h.peers[offer.From]
		h.mu.RUnlock()
		if !ok || !targetOK || offer.From == p.ID {
			return
		}
		data, _ := json.Marshal(Message{Type: "file-board-request", ID: msg.ID, From: p.ID})
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

func (h *Hub) broadcast(msg Message) {
	data, _ := json.Marshal(msg)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, peer := range h.peers {
		peer.send(data)
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

func (h *Hub) fileOfferListLocked() []FileOffer {
	list := make([]FileOffer, 0, len(h.fileOffers))
	for _, offer := range h.fileOffers {
		list = append(list, offer)
	}
	return list
}

func (h *Hub) removeFileOffersByPeerLocked(peerID string) []string {
	removed := make([]string, 0)
	for id, offer := range h.fileOffers {
		if offer.From == peerID {
			delete(h.fileOffers, id)
			removed = append(removed, id)
		}
	}
	return removed
}

func (h *Hub) removeOldestFileOfferLocked() {
	var oldestID string
	var oldestTime int64
	for id, offer := range h.fileOffers {
		if oldestID == "" || offer.CreatedAt < oldestTime {
			oldestID = id
			oldestTime = offer.CreatedAt
		}
	}
	if oldestID != "" {
		delete(h.fileOffers, oldestID)
	}
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
