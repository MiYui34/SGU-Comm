package mumbled

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

func TestACLReplyMarksOwnGroupsAsNotInherited(t *testing.T) {
	s, a, _, _ := aclState(t)
	a.Groups = map[string]*Group{"local": newGroup("local")}
	reply, _ := s.aclReply(a)
	got := map[string]*pb.ACL_ChanGroup{}
	for _, g := range reply.Groups {
		got[g.GetName()] = g
	}
	// The field defaults to true, so it must always be sent.
	if got["local"] == nil || got["local"].Inherited == nil || got["local"].GetInherited() {
		t.Fatalf("local group %v", got["local"])
	}
	if !got["admin"].GetInherited() {
		t.Fatalf("admin group %v", got["admin"])
	}
	rootReply, _ := s.aclReply(s.Channels[0])
	for _, g := range rootReply.Groups {
		if g.Inherited == nil || g.GetInherited() {
			t.Fatalf("root group %v", g)
		}
	}
}

func TestEditedACLsHaveLimits(t *testing.T) {
	m := &pb.ACL{}
	for range maxACLEntries + 50 {
		m.Acls = append(m.Acls, &pb.ACL_ChanACL{Group: proto.String("all")})
	}
	ids := make([]uint32, 0, 3000)
	for i := range 3000 {
		ids = append(ids, uint32(i%5))
	}
	m.Groups = []*pb.ACL_ChanGroup{{Name: proto.String("g"), Add: ids}}
	acls, _ := aclsFromProto(m, nil)
	if len(acls) != maxACLEntries {
		t.Fatalf("%d entries kept", len(acls))
	}
	if got := aclUserIDs(m); len(got) != 5 {
		t.Fatalf("%d ids looked up, want 5 unique", len(got))
	}
}
