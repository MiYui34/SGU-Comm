#!/bin/sh
# Runs the official Mumble client on a virtual display and connects it.
# Usage (in the container): run-client.sh HOST PORT NAME [SECONDS]
# MUMBLE_PASSWORD: log in with this password (simple characters only).
# Screenshots go to /out/NAME-*.png, the client log to /out/NAME.log.
set -e
host=$1 port=$2 name=$3 seconds=${4:-30}

Xvfb :1 -screen 0 1280x800x24 >/dev/null 2>&1 &
export DISPLAY=:1
# A null sink as the microphone: Mumble transmits continuously from it.
pulseaudio --start --exit-idle-time=-1 >/dev/null 2>&1
pactl load-module module-null-sink sink_name=mic >/dev/null
pactl set-default-source mic.monitor

# Each client has its own certificate: no certificate wizard, and servers
# can tell clients apart.
openssl req -x509 -newkey rsa:2048 -nodes -subj "/CN=$name" -days 30 \
  -keyout key.pem -out cert.pem 2>/dev/null
openssl pkcs12 -export -passout pass: -in cert.pem -inkey key.pem -out cert.p12
mkdir -p ~/.config/Mumble/Mumble
cat > ~/.config/Mumble/Mumble/mumble_settings.json <<JSON
{
  "settings_version": 1,
  "certificate": "$(base64 -w0 cert.p12)",
  "misc": {"audio_wizard_has_been_shown": true, "viewed_server_ping_consent_message": true},
  "audio": {"transmit_mode": "Continuous", "input_system": "PulseAudio", "output_system": "PulseAudio"},
  "update": {"check_for_updates": false, "check_for_plugin_updates": false, "auto_update_plugins": false}
}
JSON

user=$name
[ -n "$MUMBLE_PASSWORD" ] && user="$name:$MUMBLE_PASSWORD"
mumble "mumble://$user@$host:$port/" >"/out/$name.log" 2>&1 &
sleep 4
# Accept the server's self-signed certificate.
xdotool key alt+y || true
sleep 3
import -window root "/out/$name-connected.png"
sleep "$seconds"
import -window root "/out/$name-end.png"
