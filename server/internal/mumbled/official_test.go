package mumbled

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// officialClient runs the official Mumble client in podman (see
// packages/devtools/mumble-client) for the given time.
func officialClient(t *testing.T, addr, name string, seconds int) {
	t.Helper()
	if os.Getenv("KALLISTI_OFFICIAL_CLIENT") == "" {
		t.Skip("set KALLISTI_OFFICIAL_CLIENT=1 to test with the official client in podman")
	}
	dir, _ := filepath.Abs("../../../packages/devtools/mumble-client")
	if out, err := exec.Command("podman", "build", "-q", "-t", "kallisti-mumble-client", dir).CombinedOutput(); err != nil {
		t.Fatalf("podman build: %v\n%s", err, out)
	}
	os.MkdirAll(filepath.Join(dir, "out"), 0o755)
	_, port, _ := net.SplitHostPort(addr)
	container := fmt.Sprintf("kallisti-test-%s-%d", name, os.Getpid())
	cmd := exec.Command("podman", "run", "--rm", "--name", container,
		"-v", filepath.Join(dir, "out")+":/out",
		"-v", filepath.Join(dir, "run-client.sh")+":/usr/local/bin/run-client.sh:ro",
		"kallisti-mumble-client", "run-client.sh", "host.containers.internal", port, name, fmt.Sprint(seconds))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		exec.Command("podman", "rm", "-f", container).Run()
		cmd.Wait()
	})
}

// waitForUser reads UserStates until the named user appears.
func (c *testClient) waitForUser(name string, d time.Duration) uint32 {
	c.t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		var u pb.UserState
		proto.Unmarshal(c.expectWithin(mumble.TypeUserState, time.Until(deadline)), &u)
		if u.GetName() == name {
			return u.GetSession()
		}
	}
	c.t.Fatalf("%s did not join", name)
	return 0
}

func TestOfficialClientJoinsAndTalks(t *testing.T) {
	addr := startOn(t, "0.0.0.0")
	watcher := dial(t, addr, "watcher")
	officialClient(t, addr, "official", 20)
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
}
