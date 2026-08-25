package main

import (
	"encoding/json"
	"testing"
)

func TestClipboardRemoveByOwner(t *testing.T) {
	h := newHub()
	p := &Peer{ID: "peer1", Name: "Alice", hub: h}

	h.handleMessage(p, []byte(`{"type":"clipboard-add","text":"hello"}`))
	if len(h.clipboards) != 1 {
		t.Fatalf("expected 1 clipboard item, got %d", len(h.clipboards))
	}
	id := h.clipboards[0].ID

	h.handleMessage(p, []byte(`{"type":"clipboard-remove","id":"`+id+`"}`))
	if len(h.clipboards) != 0 {
		t.Fatalf("expected 0 clipboard items after remove, got %d", len(h.clipboards))
	}
}

func TestClipboardRemoveByNonOwner(t *testing.T) {
	h := newHub()
	owner := &Peer{ID: "peer1", Name: "Alice", hub: h}
	other := &Peer{ID: "peer2", Name: "Bob", hub: h}

	h.handleMessage(owner, []byte(`{"type":"clipboard-add","text":"secret"}`))
	id := h.clipboards[0].ID

	h.handleMessage(other, []byte(`{"type":"clipboard-remove","id":"`+id+`"}`))
	if len(h.clipboards) != 1 {
		t.Fatalf("non-owner should not be able to remove clipboard item")
	}
}

func TestClipboardRemoveOnDisconnect(t *testing.T) {
	h := newHub()
	p := &Peer{ID: "peer1", Name: "Alice", hub: h, sendCh: make(chan []byte, 8)}

	h.handleMessage(p, []byte(`{"type":"clipboard-add","text":"hello"}`))
	h.handleMessage(p, []byte(`{"type":"clipboard-add","text":"world"}`))
	if len(h.clipboards) != 2 {
		t.Fatalf("expected 2 clipboard items, got %d", len(h.clipboards))
	}

	h.unregister(p)
	if len(h.clipboards) != 0 {
		t.Fatalf("expected clipboard items removed on disconnect, got %d", len(h.clipboards))
	}
}

func TestClipboardRemovedBroadcast(t *testing.T) {
	h := newHub()
	owner := &Peer{ID: "peer1", Name: "Alice", hub: h, sendCh: make(chan []byte, 8)}
	other := &Peer{ID: "peer2", Name: "Bob", hub: h, sendCh: make(chan []byte, 8)}
	h.peers["peer1"] = owner
	h.peers["peer2"] = other

	h.handleMessage(owner, []byte(`{"type":"clipboard-add","text":"hello"}`))
	id := h.clipboards[0].ID

	select {
	case <-other.sendCh:
	default:
	}

	h.handleMessage(owner, []byte(`{"type":"clipboard-remove","id":"`+id+`"}`))

	select {
	case data := <-other.sendCh:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal broadcast: %v", err)
		}
		if msg.Type != "clipboard-removed" || msg.ID != id {
			t.Fatalf("expected clipboard-removed broadcast for %s, got %+v", id, msg)
		}
	default:
		t.Fatal("expected clipboard-removed broadcast")
	}
}
