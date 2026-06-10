package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsConn struct {
	*websocket.Conn
	mu sync.Mutex
}

func (c *wsConn) writeMessage(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.WriteMessage(websocket.TextMessage, data)
}

func handleWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	wsc := &wsConn{Conn: conn}
	peer := &Peer{
		ID:        newPeerID(),
		Name:      "",
		conn:      wsc,
		sendCh:    make(chan []byte, 32),
		hub:       hub,
		createdAt: time.Now(),
	}

	go peer.writePump()
	peer.readPump()
}

func (p *Peer) readPump() {
	defer func() {
		p.hub.unregister(p)
		p.conn.Close()
	}()

	p.conn.SetReadLimit(65536)
	p.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	p.conn.SetPongHandler(func(string) error {
		p.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, raw, err := p.conn.ReadMessage()
		if err != nil {
			break
		}
		p.hub.handleMessage(p, raw)
	}
}

func (p *Peer) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		p.conn.Close()
	}()

	for {
		select {
		case data, ok := <-p.sendCh:
			p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				p.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := p.conn.writeMessage(data); err != nil {
				return
			}
		case <-ticker.C:
			p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
