package config

import (
	"reflect"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestDefaults(t *testing.T) {
	got, err := Parse(env(map[string]string{"MUMBLE_HOST": "voice.example.org", "ALLOWED_ORIGINS": "*"}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		MumbleHost:         "voice.example.org",
		MumblePort:         64738,
		ListenPort:         8080,
		AllowedOrigins:     []string{"*"},
		RejectUnauthorized: true,
		MaxConnections:     100,
		DBPath:             "kallisti.db",
		TextChannels:       []string{"general"},
		MaxPerIP:           10,
		UploadDir:          "uploads",
		UploadMaxBytes:     10 << 20,
		UploadQuotaBytes:   100 << 20,
		BackupKeep:         7,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAllOptions(t *testing.T) {
	got, err := Parse(env(map[string]string{
		"MUMBLE_HOST":                 "localhost",
		"MUMBLE_PORT":                 "1234",
		"LISTEN_PORT":                 "9000",
		"ALLOWED_ORIGINS":             "https://a.example, http://localhost:5173",
		"MUMBLE_INSECURE_TLS":         "1",
		"MAX_CONNECTIONS":             "5",
		"DB_PATH":                     "/data/k.db",
		"TEXT_CHANNELS":               " general, links ,,general, memes",
		"MAX_CONNECTIONS_PER_IP":      "3",
		"TRUST_FORWARDED_FOR":         "1",
		"UPLOAD_DIR":                  "/data/files",
		"UPLOAD_MAX_MB":               "25",
		"WEB_DIR":                     "/app/web",
		"BACKUP_DIR":                  "/data/backups",
		"BACKUP_KEEP":                 "3",
		"LINK_PREVIEWS":               "0",
		"MEDIA_PREVIEWS":              "0",
		"FFMPEG_PATH":                 "/opt/ffmpeg",
		"PUSH":                        "0",
		"VAPID_PUBLIC_KEY":            "pub",
		"VAPID_PRIVATE_KEY":           "priv",
		"PUSH_SUBJECT":                "mailto:admin@a.example",
		"FEDI_LOGIN":                  "0",
		"FEDI_INSECURE_DEV_INSTANCES": "1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		MumbleHost:         "localhost",
		MumblePort:         1234,
		ListenPort:         9000,
		AllowedOrigins:     []string{"https://a.example", "http://localhost:5173"},
		RejectUnauthorized: false,
		MaxConnections:     5,
		DBPath:             "/data/k.db",
		TextChannels:       []string{"general", "links", "memes"},
		MaxPerIP:           3,
		TrustForwardedFor:  true,
		UploadDir:          "/data/files",
		UploadMaxBytes:     25 << 20,
		UploadQuotaBytes:   100 << 20,
		WebDir:             "/app/web",
		BackupDir:          "/data/backups",
		BackupKeep:         3,
		NoLinkPreviews:     true,
		NoMediaPreviews:    true,
		FFmpegPath:         "/opt/ffmpeg",
		Push:               Push{Off: true, PublicKey: "pub", PrivateKey: "priv", Subject: "mailto:admin@a.example"},
		NoFediLogin:        true,
		FediInsecureDev:    true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		vars map[string]string
		want string
	}{
		{map[string]string{"MUMBLE_HOST": "x"}, "ALLOWED_ORIGINS is required (use * to allow every origin)"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "MUMBLE_LISTEN": "nope"}, "MUMBLE_LISTEN must be host:port"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "MUMBLE_MAX_USERS": "0"}, "MUMBLE_MAX_USERS must be a positive integer"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "WHIP_UDP_PORT": "udp"}, "WHIP_UDP_PORT must be a port number"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "MUMBLE_CERT_FILE": "/c.crt"}, "set both MUMBLE_CERT_FILE and MUMBLE_KEY_FILE, or neither"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MUMBLE_PORT": "abc"}, "MUMBLE_PORT must be a port number"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "LISTEN_PORT": "70000"}, "LISTEN_PORT must be a port number"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MAX_CONNECTIONS": "0"}, "MAX_CONNECTIONS must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MAX_CONNECTIONS_PER_IP": "-1"}, "MAX_CONNECTIONS_PER_IP must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "UPLOAD_MAX_MB": "big"}, "UPLOAD_MAX_MB must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "BACKUP_KEEP": "0"}, "BACKUP_KEEP must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "VAPID_PUBLIC_KEY": "pub"}, "set both VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY, or neither"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "PUSH_SUBJECT": "admin@a.example"}, "PUSH_SUBJECT must be a mailto: or https: URL"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "RECORDING_MAX_MINUTES": "0"}, "RECORDING_MAX_MINUTES must be a positive integer"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "RECORDING_MAX_MB": "x"}, "RECORDING_MAX_MB must be a positive integer"},
		{map[string]string{"ALLOWED_ORIGINS": "*", "RECORDING_KEEP_DAYS": "-1"}, "RECORDING_KEEP_DAYS must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MC_STATUS_ADDR": "play.example.org"}, "MC_STATUS_ADDR must be host:port"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MC_STATUS_ADDR": "play.example.org:0"}, "MC_STATUS_ADDR must be host:port"},
	}
	for _, c := range cases {
		_, err := Parse(env(c.vars))
		if err == nil || err.Error() != c.want {
			t.Errorf("Parse(%v): got error %v, want %q", c.vars, err, c.want)
		}
	}
}

func TestMinecraftAddr(t *testing.T) {
	got, err := Parse(env(map[string]string{
		"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MC_STATUS_ADDR": "play.example.org:25565",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.MinecraftAddr != "play.example.org:25565" {
		t.Errorf("MC_STATUS_ADDR %q", got.MinecraftAddr)
	}
}

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		allowed []string
		origin  string
		want    bool
	}{
		{[]string{"*"}, "https://any.example", true},
		{[]string{"*"}, "", true},
		{[]string{}, "https://a.example", false},
		{[]string{"https://a.example"}, "https://a.example", true},
		{[]string{"https://a.example"}, "https://b.example", false},
		{[]string{"https://a.example"}, "", false},
		// The desktop app (Tauri), on every server.
		{[]string{"https://a.example"}, "tauri://localhost", true},
		{[]string{"https://a.example"}, "http://tauri.localhost", true},
		{[]string{"https://a.example"}, "https://tauri.localhost", true},
		{[]string{"https://a.example"}, "tauri://evil.example", false},
	}
	for _, c := range cases {
		if got := OriginAllowed(c.allowed, c.origin); got != c.want {
			t.Errorf("OriginAllowed(%v, %q) = %v, want %v", c.allowed, c.origin, got, c.want)
		}
	}
}

func TestTheBuiltInServerIsTheDefault(t *testing.T) {
	got, err := Parse(env(map[string]string{"ALLOWED_ORIGINS": "*", "DB_PATH": "/data/kallisti.db"}))
	if err != nil {
		t.Fatal(err)
	}
	want := &Builtin{Listen: ":64738", ServerName: "Kallisti", MaxUsers: 100, CertDir: "/data",
		RecordingDir: "/data/recordings", RecordingMax: 4 * time.Hour, RecordingMaxBytes: 1 << 30, RecordingKeep: 7 * 24 * time.Hour}
	if !reflect.DeepEqual(got.Builtin, want) {
		t.Errorf("got %+v, want %+v", got.Builtin, want)
	}
	got, _ = Parse(env(map[string]string{
		"ALLOWED_ORIGINS": "*", "MUMBLE_LISTEN": "0.0.0.0:7000", "SERVER_NAME": "Our place",
		"MUMBLE_WELCOME": "Hi!", "MUMBLE_PASSWORD": "pw", "MUMBLE_MAX_USERS": "20",
		"MUMBLE_MAX_PER_IP": "5", "CERT_DIR": "/certs",
		"MUMBLE_CERT_FILE": "/le/c.crt", "MUMBLE_KEY_FILE": "/le/c.key",
		"WHIP_UDP_PORT": "8189", "WHIP_PUBLIC_IP": "chat.example.org",
		"RECORDING_DIR": "/rec", "RECORDING_MAX_MINUTES": "90", "RECORDING_MAX_MB": "200", "RECORDING_KEEP_DAYS": "2",
	}))
	want = &Builtin{Listen: "0.0.0.0:7000", ServerName: "Our place", Welcome: "Hi!", Password: "pw",
		MaxUsers: 20, MaxPerIP: 5, CertDir: "/certs", CertFile: "/le/c.crt", KeyFile: "/le/c.key",
		WHIPPort: 8189, WHIPPublicIP: "chat.example.org",
		RecordingDir: "/rec", RecordingMax: 90 * time.Minute, RecordingMaxBytes: 200 << 20, RecordingKeep: 2 * 24 * time.Hour}
	if !reflect.DeepEqual(got.Builtin, want) {
		t.Errorf("got %+v, want %+v", got.Builtin, want)
	}
}

func TestScreenShareURL(t *testing.T) {
	got, err := Parse(env(map[string]string{
		"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "SCREEN_SHARE_URL": "https://meet.example.org/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got.ScreenShareURL != "https://meet.example.org" {
		t.Errorf("SCREEN_SHARE_URL %q", got.ScreenShareURL)
	}
	// A path would end up half-working in a frame, so it is refused up front.
	for _, bad := range []string{"https://meet.example.org/room", "meet.example.org", "ftp://meet.example.org"} {
		_, err := Parse(env(map[string]string{
			"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "SCREEN_SHARE_URL": bad,
		}))
		if err == nil {
			t.Errorf("SCREEN_SHARE_URL %q: expected an error", bad)
		}
	}
}