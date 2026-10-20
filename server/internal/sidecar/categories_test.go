package sidecar

import (
	"slices"
	"strconv"
	"testing"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/mumble"
)

// manageOK sends a request and returns the channels event it brings.
func manageOK(t *testing.T, c *Conn, req string) map[string]any {
	t.Helper()
	c.limit = rate.NewLimiter(rate.Inf, 0) // many requests in a row
	c.Handle([]byte(req))
	m := next(t, c)
	if m["type"] != "channels" {
		t.Fatalf("%s: %v", req, m)
	}
	drain(c)
	return m
}

func manageRefused(t *testing.T, c *Conn, req string) {
	t.Helper()
	c.limit = rate.NewLimiter(rate.Inf, 0)
	c.Handle([]byte(req))
	if m := next(t, c); m["type"] != "error" || m["about"] != "manage" {
		t.Fatalf("%s: %v", req, m)
	}
	drain(c)
}

// categoriesOf: the names of the categories in a channels event, and per
// channel name its category's name ("": none).
func categoriesOf(m map[string]any) (names []string, of map[string]string) {
	byID := map[float64]string{}
	names = []string{}
	for _, c := range m["categories"].([]any) {
		c := c.(map[string]any)
		byID[c["id"].(float64)] = c["name"].(string)
		names = append(names, c["name"].(string))
	}
	of = map[string]string{}
	for _, c := range m["channels"].([]any) {
		c := c.(map[string]any)
		id, _ := c["category"].(float64)
		of[c["name"].(string)] = byID[id]
	}
	return names, of
}

func (h *Hub) categoryID(name string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.categories {
		if c.Name == name {
			return strconv.FormatInt(c.ID, 10)
		}
	}
	return "0"
}

func (h *Hub) channelID(name string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.channels {
		if c.Name == name {
			return strconv.FormatInt(c.ID, 10)
		}
	}
	return "0"
}

func TestStaffManageCategories(t *testing.T) {
	h := newHub(t, "general", "random")
	v := &fakeVoice{}
	h.SetVoice(v, "")
	staff := joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 10})
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 20})
	for _, req := range []string{
		`{"type":"create-category","name":"Games"}`,
		`{"type":"rename-category","category":1,"name":"x"}`,
		`{"type":"delete-category","category":1}`,
		`{"type":"order-categories","categories":[1]}`,
		`{"type":"set-category","channel":1,"category":0}`,
	} {
		manageRefused(t, bob, req)
	}
	for _, name := range []string{"", "#games", "a/b", "this name is far too long for a category"} {
		manageRefused(t, staff, `{"type":"create-category","name":"`+name+`"}`)
	}
	manageOK(t, staff, `{"type":"create-category","name":"  Games  "}`)
	manageRefused(t, staff, `{"type":"create-category","name":"games"}`)
	manageOK(t, staff, `{"type":"create-category","name":"Music"}`)
	games, music := h.categoryID("Games"), h.categoryID("Music")
	manageOK(t, staff, `{"type":"set-category","channel":`+h.channelID("random")+`,"category":`+games+`}`)
	manageOK(t, staff, `{"type":"create-channel","name":"jazz","category":`+music+`}`)
	manageOK(t, staff, `{"type":"rename-category","category":`+games+`,"name":"Gaming"}`)
	drain(bob)
	m := manageOK(t, staff, `{"type":"order-categories","categories":[`+music+`,`+games+`]}`)
	names, of := categoriesOf(m)
	if !slices.Equal(names, []string{"Music", "Gaming"}) || of["random"] != "Gaming" || of["jazz"] != "Music" || of["general"] != "" {
		t.Fatalf("got %v %v", names, of)
	}
	// bob sees them too (their channels are public), in his channels event.
	if names, _ := categoriesOf(nextOf(t, bob, "channels")); len(names) == 0 {
		t.Fatal("bob got no categories")
	}
	// The mirror tree follows.
	v.mu.Lock()
	cats, synced := slices.Clone(v.syncedCats), slices.Clone(v.synced)
	v.mu.Unlock()
	if len(cats) != 2 || cats[0].Name != "Music" || synced[1].Name != "random" || synced[1].Category != 2 || synced[2].Category != 1 {
		t.Fatalf("synced %+v %+v", cats, synced)
	}
	// A deleted category's channels are uncategorized after it.
	m = manageOK(t, staff, `{"type":"delete-category","category":`+music+`}`)
	if names, of := categoriesOf(m); !slices.Equal(names, []string{"Gaming"}) || of["jazz"] != "" {
		t.Fatalf("after delete %v %v", names, of)
	}
	manageRefused(t, staff, `{"type":"create-channel","name":"nope","category":`+music+`}`)
}

// catHub: general (mirror 100), category Games (Mumble 200) with
// minecraft (101); carol (session 3) moderates Games, bob is a plain
// user.
func catHub(t *testing.T) (h *Hub, v *fakeVoice, staff, carol *Conn) {
	t.Helper()
	h = newHub(t, "general")
	v = &fakeVoice{moderates: map[uint32]uint32{}, hidden: map[uint32]bool{}, members: map[uint32]bool{}}
	h.SetVoice(v, "")
	staff = joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 10})
	manageOK(t, staff, `{"type":"create-category","name":"Games"}`)
	manageOK(t, staff, `{"type":"create-channel","name":"minecraft","category":`+h.categoryID("Games")+`}`)
	v.setModerates(3, 200)
	carol = joinAs(t, h, mumble.Identity{Session: 3, Name: "carol", Registered: true, UserID: 30})
	return h, v, staff, carol
}

func TestCategoryModeratorsManageChannelsInTheirCategory(t *testing.T) {
	h, _, _, carol := catHub(t)
	games, general := h.categoryID("Games"), h.channelID("general")
	for _, req := range []string{
		`{"type":"create-channel","name":"outside"}`,
		`{"type":"create-channel","name":"club","category":` + games + `,"private":true}`,
		`{"type":"rename-channel","channel":` + general + `,"name":"mine"}`,
		`{"type":"delete-channel","channel":` + general + `}`,
		`{"type":"set-topic","channel":` + general + `,"text":"hi"}`,
		`{"type":"order-channels","channels":[` + general + `]}`,
		`{"type":"create-category","name":"More"}`,
		`{"type":"rename-category","category":` + games + `,"name":"Mine"}`,
		`{"type":"delete-category","category":` + games + `}`,
		`{"type":"set-category","channel":` + h.channelID("minecraft") + `,"category":0}`,
		`{"type":"set-channel-moderator","user":20,"voice":200,"on":true}`,
	} {
		manageRefused(t, carol, req)
	}
	manageOK(t, carol, `{"type":"create-channel","name":"valorant","category":`+games+`}`)
	mc, val := h.channelID("minecraft"), h.channelID("valorant")
	manageRefused(t, carol, `{"type":"order-channels","channels":[`+val+`,`+general+`]}`)
	manageOK(t, carol, `{"type":"order-channels","channels":[`+val+`,`+mc+`]}`)
	manageOK(t, carol, `{"type":"rename-channel","channel":`+mc+`,"name":"terraria"}`)
	m := manageOK(t, carol, `{"type":"set-topic","channel":`+mc+`,"text":"blocks"}`)
	if _, of := categoriesOf(m); of["valorant"] != "Games" || of["terraria"] != "Games" || !slices.Equal(channelNames(m), []string{"general", "valorant", "terraria"}) {
		t.Fatalf("got %v", m)
	}
	manageOK(t, carol, `{"type":"delete-channel","channel":`+val+`}`)
	// A deleted channel (it may have been private) comes back only by
	// staff, with its history.
	manageRefused(t, carol, `{"type":"create-channel","name":"valorant","category":`+games+`}`)
}

// A private channel in a category: its moderators manage it only as
// members (they see it); the category shows to those who see a channel
// in it, its moderators and staff.
func TestPrivateChannelsInCategories(t *testing.T) {
	h, v, staff, carol := catHub(t)
	games := h.categoryID("Games")
	m := manageOK(t, staff, `{"type":"create-channel","name":"club","category":`+games+`,"private":true}`)
	if _, of := categoriesOf(m); of["club"] != "Games" {
		t.Fatalf("staff got %v", m)
	}
	club := h.channelID("club")
	if !v.hidden[102] || h.mirrorOf(mustInt(club)) != 102 {
		t.Fatal("not private")
	}
	drain(carol)
	manageRefused(t, carol, `{"type":"rename-channel","channel":`+club+`,"name":"mine"}`)
	manageRefused(t, carol, `{"type":"delete-channel","channel":`+club+`}`)
	v.mu.Lock()
	v.members[3] = true
	v.mu.Unlock()
	manageOK(t, carol, `{"type":"rename-channel","channel":`+club+`,"name":"lounge"}`)
	// bob sees the category only while it has a channel he sees.
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 20})
	manageOK(t, staff, `{"type":"delete-channel","channel":`+h.channelID("minecraft")+`}`)
	if m := nextOf(t, bob, "channels"); len(m["categories"].([]any)) != 0 {
		t.Fatalf("bob sees %v", m)
	}
	if names, _ := categoriesOf(nextOf(t, carol, "channels")); len(names) != 1 {
		t.Fatalf("carol sees %v", names)
	}
}

func mustInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
