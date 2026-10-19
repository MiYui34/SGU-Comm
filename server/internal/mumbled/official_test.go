package mumbled

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

var officialRuns int

// officialRun is a running official client.
type officialRun struct {
	t               *testing.T
	container, name string
}

// screenshot saves out/<name>-<label>.png.
func (o officialRun) screenshot(label string) {
	o.xdo("import", "-window", "root", fmt.Sprintf("/out/%s-%s.png", o.name, label))
}

// xdo runs a command on the client's display (xdotool, import, ...).
func (o officialRun) xdo(args ...string) {
	o.t.Helper()
	full := append([]string{"exec", "-e", "DISPLAY=:1", o.container}, args...)
	if out, err := exec.Command("podman", full...).CombinedOutput(); err != nil {
		o.t.Logf("%v: %v %s", args, err, out)
	}
}

// officialClient runs the official Mumble client in podman (see
// packages/devtools/mumble-client) for the given time.
func officialClient(t *testing.T, addr, name, password string, seconds int, env ...string) officialRun {
	t.Helper()
	needOfficialClient(t)
	dir, _ := filepath.Abs("../../../packages/devtools/mumble-client")
	image, debian := "kallisti-mumble-client", "trixie"
	if slices.Contains(env, "MUMBLE_OLD=1") {
		image, debian = "kallisti-mumble-client-old", "bookworm"
	}
	if out, err := exec.Command("podman", "build", "-q", "--build-arg", "DEBIAN="+debian, "-t", image, dir).CombinedOutput(); err != nil {
		t.Fatalf("podman build: %v\n%s", err, out)
	}
	os.MkdirAll(filepath.Join(dir, "out"), 0o755)
	_, port, _ := net.SplitHostPort(addr)
	officialRuns++
	container := fmt.Sprintf("kallisti-test-%s-%d-%d", name, os.Getpid(), officialRuns)
	args := []string{"run", "--rm", "--name", container, "-e", "MUMBLE_PASSWORD=" + password,
		"-v", filepath.Join(dir, "out") + ":/out",
		"-v", filepath.Join(dir, "run-client.sh") + ":/usr/local/bin/run-client.sh:ro"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, image, "run-client.sh", "host.containers.internal", port, name, fmt.Sprint(seconds))
	cmd := exec.Command("podman", args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		exec.Command("podman", "rm", "-f", container).Run()
		cmd.Wait()
	})
	return officialRun{t: t, container: container, name: name}
}

func needOfficialClient(t *testing.T) {
	if os.Getenv("KALLISTI_OFFICIAL_CLIENT") == "" {
		t.Skip("set KALLISTI_OFFICIAL_CLIENT=1 to test with the official client in podman")
	}
}

// waitForUser reads UserStates until the named user appears.
func (c *testClient) waitForUser(name string, d time.Duration) uint32 {
	c.t.Helper()
	return c.waitForUserState(name, d).GetSession()
}

func (c *testClient) waitForUserState(name string, d time.Duration) *pb.UserState {
	c.t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		var u pb.UserState
		proto.Unmarshal(c.expectWithin(mumble.TypeUserState, time.Until(deadline)), &u)
		if u.GetName() == name {
			return &u
		}
	}
	c.t.Fatalf("%s did not join", name)
	return nil
}

func TestOfficialClientJoinsAndTalks(t *testing.T) {
	needOfficialClient(t)
	addr := startServer(t, "0.0.0.0", Config{Welcome: "Welcome!"}, openACL)
	watcher := dial(t, addr, "watcher")
	games := watcher.createChannel(0, "Games", false)
	watcher.createChannel(games, "Lobby", false)
	official := officialClient(t, addr, "official", "", 30)
	session := watcher.waitForUser("official", 40*time.Second)
	// It transmits continuously from a silent microphone.
	deadline := time.Now().Add(10 * time.Second)
	for {
		h := parseAudio(t, watcher.expectWithin(mumble.TypeUDPTunnel, time.Until(deadline)))
		if h.sender == session {
			break
		}
	}
	// Its UDP packets decrypt on our server.
	time.Sleep(2 * time.Second)
	watcher.send(mumble.TypeUserStats, &pb.UserStats{Session: proto.Uint32(session)})
	var stats pb.UserStats
	proto.Unmarshal(watcher.expect(mumble.TypeUserStats), &stats)
	if stats.GetFromClient().GetGood() == 0 {
		t.Fatalf("no UDP packets from the official client decrypted: %v", &stats)
	}
	// In another channel it is not heard any more.
	watcher.move(session, games)
	watcher.waitForMove(session, games)
	time.Sleep(500 * time.Millisecond)
	for len(watcher.frames) > 0 {
		<-watcher.frames
	}
	watcher.expectNone(mumble.TypeUDPTunnel)
	official.screenshot("moved")
}

func TestOfficialClientLogsInAsSuperUser(t *testing.T) {
	needOfficialClient(t)
	db := testDB(t)
	setAdminPassword(db, "adminpw")
	addr := startWith(t, "0.0.0.0", Config{DB: db})
	watcher := dial(t, addr, "watcher")
	official := officialClient(t, addr, "SuperUser", "adminpw", 30)
	u := watcher.waitForUserState("SuperUser", 40*time.Second)
	if u.GetUserId() != 0 || u.UserId == nil || u.GetHash() == "" {
		t.Fatalf("SuperUser state %v", u)
	}
	official.screenshot("superuser")
}

func TestOfficialClientIsKeptOutOfARestrictedChannel(t *testing.T) {
	needOfficialClient(t)
	db := testDB(t)
	setAdminPassword(db, "admin")
	addr := startWith(t, "0.0.0.0", Config{DB: db})
	su := superUser(t, addr)
	vip := su.createChannel(0, "VIP", false)
	su.createChannel(0, "Open", false)
	su.setACL(vip, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Enter)}, nil)
	official := officialClient(t, addr, "official", "", 30, "MUMBLE_CHANNEL=VIP")
	session := su.waitForUser("official", 40*time.Second)
	time.Sleep(3 * time.Second) // time to try to enter VIP
	official.screenshot("restricted")
	// It never got into VIP.
	for len(su.frames) > 0 {
		f := <-su.frames
		if f.typ == mumble.TypeUserState {
			var m pb.UserState
			proto.Unmarshal(f.body, &m)
			if m.GetSession() == session && m.ChannelId != nil && m.GetChannelId() == vip {
				t.Fatal("the official client entered VIP")
			}
		}
	}
}

func TestOfficialClientEditsACL(t *testing.T) {
	needOfficialClient(t)
	db := testDB(t)
	setAdminPassword(db, "admin")
	addr := startWith(t, "0.0.0.0", Config{DB: db})
	su := superUser(t, addr)
	vip := su.createChannel(0, "VIP", false)
	su.setACL(vip, true, []*pb.ACL_ChanACL{chanACL("all", true, true, 0, Enter)}, nil)
	su.conn.Close()
	watcher := dial(t, addr, "watcher")
	official := officialClient(t, addr, "SuperUser", "admin", 60)
	watcher.waitForUser("SuperUser", 40*time.Second)
	time.Sleep(2 * time.Second)
	official.screenshot("acl-1")
	official.xdo("xdotool", "key", "Escape") // the mute cue question (SuperUser may not speak)
	time.Sleep(time.Second)
	official.xdo("xdotool", "mousemove", "300", "132", "click", "3")
	time.Sleep(time.Second)
	official.screenshot("acl-2")
	official.xdo("xdotool", "key", "e") // Edit...
	time.Sleep(2 * time.Second)
	official.screenshot("acl-3")
	official.xdo("xdotool", "mousemove", "220", "26", "click", "1") // ACL tab
	time.Sleep(time.Second)
	official.xdo("xdotool", "mousemove", "60", "209", "click", "1") // VIP's own @all entry (after root's six)
	time.Sleep(time.Second)
	official.xdo("xdotool", "mousemove", "749", "159", "click", "1") // clear "Deny Enter"
	time.Sleep(time.Second)
	official.screenshot("acl-4")
	official.xdo("xdotool", "mousemove", "675", "508", "click", "1") // OK
	// The edited ACL came back intact, without the deny: VIP is open.
	for {
		var m pb.ChannelState
		proto.Unmarshal(watcher.expectWithin(mumble.TypeChannelState, 10*time.Second), &m)
		if m.GetChannelId() == vip && m.IsEnterRestricted != nil {
			if m.GetIsEnterRestricted() || !m.GetCanEnter() {
				t.Fatalf("after the edit: %v", &m)
			}
			break
		}
	}
	official.screenshot("acl-5")
}

func TestOfficialClientIsMutedAndBanned(t *testing.T) {
	needOfficialClient(t)
	os.Remove("../../../packages/devtools/mumble-client/out/certs/banned.p12")
	db := testDB(t)
	setAdminPassword(db, "admin")
	addr := startWith(t, "0.0.0.0", Config{DB: db})
	su := superUser(t, addr)
	official := officialClient(t, addr, "official", "", 60, "MUMBLE_CERT=banned")
	session := su.waitForUser("official", 40*time.Second)
	// Heard, until a server mute.
	for {
		if parseAudio(t, su.expectWithin(mumble.TypeUDPTunnel, 10*time.Second)).sender == session {
			break
		}
	}
	su.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(session), Mute: proto.Bool(true)})
	time.Sleep(time.Second)
	drain(su)
	su.expectNone(mumble.TypeUDPTunnel)
	official.screenshot("muted")
	// Banned: kicked now, and refused when it comes back. The ban is set
	// to the certificate only (the address may be the test's own).
	su.send(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(session), Reason: proto.String("testing bans"), Ban: proto.Bool(true)})
	su.send(mumble.TypeBanList, &pb.BanList{Query: proto.Bool(true)})
	var list pb.BanList
	proto.Unmarshal(su.expect(mumble.TypeBanList), &list)
	list.Bans[0].Address, list.Bans[0].Mask = make([]byte, 16), proto.Uint32(0)
	su.send(mumble.TypeBanList, &pb.BanList{Bans: list.Bans})
	time.Sleep(2 * time.Second)
	official.screenshot("banned")
	again := officialClient(t, addr, "official", "", 20, "MUMBLE_CERT=banned")
	time.Sleep(15 * time.Second)
	again.screenshot("refused")
	for _, f := range drain(su) {
		var m pb.UserState
		if f.typ == mumble.TypeUserState && proto.Unmarshal(f.body, &m) == nil && m.GetName() == "official" {
			t.Fatal("the banned client joined again")
		}
	}
}

func TestOfficialClientListensToAChannel(t *testing.T) {
	needOfficialClient(t)
	addr := startServer(t, "0.0.0.0", Config{}, openACL)
	watcher := dial(t, addr, "watcher")
	games := watcher.createChannel(0, "Games", false)
	official := officialClient(t, addr, "official", "", 60)
	session := watcher.waitForUser("official", 40*time.Second)
	time.Sleep(2 * time.Second)
	// Rows: Root, official, watcher, Games.
	official.xdo("xdotool", "mousemove", "300", "132", "click", "3")
	time.Sleep(time.Second)
	official.xdo("xdotool", "key", "l") // Listen To Channel
	for {
		var m pb.UserState
		proto.Unmarshal(watcher.expectWithin(mumble.TypeUserState, 10*time.Second), &m)
		if m.GetSession() == session && len(m.ListeningChannelAdd) > 0 {
			if m.ListeningChannelAdd[0] != games {
				t.Fatalf("listening %v", m.ListeningChannelAdd)
			}
			break
		}
	}
	official.screenshot("listening")
}

func TestOfficialOldClientTalksWithNewClients(t *testing.T) {
	needOfficialClient(t)
	addr := startServer(t, "0.0.0.0", Config{}, nil)
	watcher := dial(t, addr, "watcher")
	old := officialClient(t, addr, "old13", "", 60, "MUMBLE_OLD=1")
	session := watcher.waitForUser("old13", 50*time.Second)
	// Mumble 1.3 sends the legacy format; the 1.5 test client gets protobuf.
	for {
		if parseAudio(t, watcher.expectWithin(mumble.TypeUDPTunnel, 10*time.Second)).sender == session {
			break
		}
	}
	time.Sleep(2 * time.Second)
	watcher.send(mumble.TypeUserStats, &pb.UserStats{Session: proto.Uint32(session)})
	var stats pb.UserStats
	proto.Unmarshal(watcher.expect(mumble.TypeUserStats), &stats)
	if stats.GetFromClient().GetGood() == 0 {
		t.Fatalf("no UDP packets from the old client decrypted: %v", &stats)
	}
	// The other way: the old client shows the watcher talking.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 150 {
			watcher.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audioTo(0, uint64(i), silentOpus)))
			time.Sleep(20 * time.Millisecond)
		}
	}()
	time.Sleep(1500 * time.Millisecond)
	old.screenshot("hears-watcher")
	<-done
}

// silentOpus is a real Opus packet (20 ms of silence, CELT mode), which
// clients decode and show as talking.
var silentOpus = []byte{0xf8, 0xff, 0xfe}

func TestOfficialClientFetchesALongComment(t *testing.T) {
	needOfficialClient(t)
	addr := startServer(t, "0.0.0.0", Config{}, openACL)
	watcher := dial(t, addr, "watcher")
	comment := "This is a long comment that the server sends as a hash first. " +
		"The official client asks for it with RequestBlob when it needs to show it."
	watcher.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(watcher.session), Comment: proto.String(comment)})
	official := officialClient(t, addr, "official", "", 60)
	watcher.waitForUser("official", 40*time.Second)
	time.Sleep(2 * time.Second)
	// Rows: Root, official, watcher. Hovering shows the comment.
	official.xdo("xdotool", "mousemove", "330", "112", "click", "3")
	time.Sleep(time.Second)
	official.xdo("xdotool", "key", "w") // View Comment
	time.Sleep(2 * time.Second)
	official.screenshot("comment")
}
