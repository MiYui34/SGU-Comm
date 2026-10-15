package mumbled

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func TestUnregisterByCertificate(t *testing.T) {
	db := testDB(t)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	cert := clientCert(t)
	c := dialAs(t, addr, dialOpts{name: "visitor", cert: cert})
	c.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(c.session), UserId: proto.Uint32(0)})
	waitForUserID(t, c, c.session)
	c.conn.Close()
	if err := s.Unregister(certHashOf(cert)); err != nil {
		t.Fatal(err)
	}
	if r, _ := db.UserByName("visitor"); r != nil {
		t.Fatalf("still registered: %+v", r)
	}
	// Unknown certificates: nothing to do.
	if err := s.Unregister("feedface"); err != nil {
		t.Fatal(err)
	}
}
