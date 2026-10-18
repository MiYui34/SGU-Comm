package sidecar

import (
	"errors"
	"testing"
)

// Session 1 is an admin.
func (f *fakeVoice) Connections(session uint32) ([]Connection, error) {
	if session != 1 {
		return nil, errors.New("only admins see the connections")
	}
	ping := 42.0
	return []Connection{{Session: 2, Name: "bob", Transport: "udp", Ping: &ping, Online: 60}}, nil
}

func TestConnectionsAreForAdmins(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	admin := readyAs(t, h, 1, "admin")
	bob := readyAs(t, h, 2, "bob")
	bob.Handle([]byte(`{"type":"connections"}`))
	if m := next(t, bob); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	admin.Handle([]byte(`{"type":"connections"}`))
	m := next(t, admin)
	conns, _ := m["connections"].([]any)
	if m["type"] != "connections" || len(conns) != 1 {
		t.Fatalf("admin got %v", m)
	}
	c := conns[0].(map[string]any)
	if c["name"] != "bob" || c["ping"] != 42.0 || c["fromClient"] != nil || c["online"] != 60.0 || c["address"] != nil {
		t.Fatalf("connection %v", c)
	}
}
