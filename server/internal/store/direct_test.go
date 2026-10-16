package store

import (
	"testing"
	"time"
)

func TestDirectChats(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	ab, err := s.DirectChat(7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := s.DirectChat(3, 7); again != ab {
		t.Fatalf("not the same chat for both orders: %d, %d", ab, again)
	}
	ac, _ := s.DirectChat(3, 9)
	if a, b, ok, _ := s.DirectPeers(ab); !ok || a != 3 || b != 7 {
		t.Fatalf("peers %d %d %v", a, b, ok)
	}
	cs, _ := s.Channels()
	if _, _, ok, _ := s.DirectPeers(cs[0].ID); ok || len(cs) != 1 {
		t.Fatalf("a text channel is direct, or direct chats are listed: %v", cs)
	}
	// Listed once they have messages.
	if got, _ := s.DirectChats(3); len(got) != 0 {
		t.Fatalf("empty chats listed: %v", got)
	}
	s.AddMessage(NewMessage{ChannelID: ab, Author: "x", Text: "hi", At: time.Now()})
	s.AddMessage(NewMessage{ChannelID: ac, Author: "x", Text: "hi", At: time.Now()})
	got, _ := s.DirectChats(3)
	if len(got) != 2 || got[ab] != 7 || got[ac] != 9 {
		t.Fatalf("chats of 3: %v", got)
	}
	// Survive EnsureChannels (which hides other channels).
	s.EnsureChannels([]string{"general"})
	if h, _ := s.History(ab, 0, 10); len(h) != 1 {
		t.Fatal("no history")
	}
	if _, err := s.DirectChat(5, 5); err == nil {
		t.Fatal("a chat with oneself")
	}
}
