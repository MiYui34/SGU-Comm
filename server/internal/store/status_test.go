package store

import "testing"

func TestUserStatus(t *testing.T) {
	s := open(t)
	if st, err := s.UserStatus(4); err != nil || st != (UserStatus{}) {
		t.Fatalf("none yet: %+v %v", st, err)
	}
	want := UserStatus{Status: "dnd", Emoji: "🍕", Text: "lunch", Until: 1_000_000}
	if err := s.SetUserStatus(4, want); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.UserStatus(4); st != want {
		t.Fatalf("got %+v", st)
	}
	if st, _ := s.UserStatus(5); st != (UserStatus{}) {
		t.Fatalf("other user: %+v", st)
	}
	// The zero status removes the row.
	if err := s.SetUserStatus(4, UserStatus{}); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM user_status`).Scan(&n)
	if st, _ := s.UserStatus(4); st != (UserStatus{}) || n != 0 {
		t.Fatalf("after clearing: %+v, %d rows", st, n)
	}
}
