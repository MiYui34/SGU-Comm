package config

import (
	"reflect"
	"testing"
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
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAllOptions(t *testing.T) {
	got, err := Parse(env(map[string]string{
		"MUMBLE_HOST":            "localhost",
		"MUMBLE_PORT":            "1234",
		"LISTEN_PORT":            "9000",
		"ALLOWED_ORIGINS":        "https://a.example, http://localhost:5173",
		"MUMBLE_INSECURE_TLS":    "1",
		"MAX_CONNECTIONS":        "5",
		"DB_PATH":                "/data/k.db",
		"TEXT_CHANNELS":          " general, links ,,general, memes",
		"MAX_CONNECTIONS_PER_IP": "3",
		"TRUST_FORWARDED_FOR":    "1",
		"UPLOAD_DIR":             "/data/files",
		"UPLOAD_MAX_MB":          "25",
		"WEB_DIR":                "/app/web",
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
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MUMBLE_PORT": "abc"}, "MUMBLE_PORT must be a port number"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "LISTEN_PORT": "70000"}, "LISTEN_PORT must be a port number"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MAX_CONNECTIONS": "0"}, "MAX_CONNECTIONS must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "MAX_CONNECTIONS_PER_IP": "-1"}, "MAX_CONNECTIONS_PER_IP must be a positive integer"},
		{map[string]string{"MUMBLE_HOST": "x", "ALLOWED_ORIGINS": "*", "UPLOAD_MAX_MB": "big"}, "UPLOAD_MAX_MB must be a positive integer"},
	}
	for _, c := range cases {
		_, err := Parse(env(c.vars))
		if err == nil || err.Error() != c.want {
			t.Errorf("Parse(%v): got error %v, want %q", c.vars, err, c.want)
		}
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
	want := &Builtin{Listen: ":64738", ServerName: "Kallisti", MaxUsers: 100, CertDir: "/data"}
	if !reflect.DeepEqual(got.Builtin, want) {
		t.Errorf("got %+v, want %+v", got.Builtin, want)
	}
	got, _ = Parse(env(map[string]string{
		"ALLOWED_ORIGINS": "*", "MUMBLE_LISTEN": "0.0.0.0:7000", "SERVER_NAME": "Our place",
		"MUMBLE_WELCOME": "Hi!", "MUMBLE_PASSWORD": "pw", "MUMBLE_MAX_USERS": "20",
		"MUMBLE_MAX_PER_IP": "5", "CERT_DIR": "/certs",
	}))
	want = &Builtin{Listen: "0.0.0.0:7000", ServerName: "Our place", Welcome: "Hi!", Password: "pw",
		MaxUsers: 20, MaxPerIP: 5, CertDir: "/certs"}
	if !reflect.DeepEqual(got.Builtin, want) {
		t.Errorf("got %+v, want %+v", got.Builtin, want)
	}
}
