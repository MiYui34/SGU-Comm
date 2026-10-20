import { describe, expect, it } from "vitest";
import { backgroundAudioWanted, voiceSession } from "../src/lib/mediaSession";

describe("voiceSession", () => {
  const state = { channel: "Hangout", server: "Kallisti", lobby: true };

  it("names the voice channel and the server, with the app icon", () => {
    const s = voiceSession(state);
    expect(s.metadata.title).toBe("Voice: Hangout");
    expect(s.metadata.artist).toBe("Kallisti");
    expect(s.metadata.artwork.map((a) => a.src)).toEqual(["/icons/icon-192.png", "/icons/icon-512.png"]);
  });

  // Pause, stop and a headset's hang-up button all mean the same.
  it("leaves voice on a Kallisti server", () => {
    expect(voiceSession(state).actions).toEqual({ pause: "leave", stop: "leave", hangup: "leave" });
  });

  // Plain Mumble servers have no place outside voice: deafen instead.
  it("deafens on a plain Mumble server", () => {
    expect(voiceSession({ ...state, lobby: false }).actions).toEqual({ pause: "deafen", stop: "deafen", hangup: "deafen" });
  });
});

describe("backgroundAudioWanted", () => {
  const env = { android: true, mediaSession: true, mediaStreams: true };

  it("on Android with a Media Session and media stream output", () => {
    expect(backgroundAudioWanted(env)).toBe(true);
  });

  it("not elsewhere or where something is missing", () => {
    expect(backgroundAudioWanted({ ...env, android: false })).toBe(false);
    expect(backgroundAudioWanted({ ...env, mediaSession: false })).toBe(false);
    expect(backgroundAudioWanted({ ...env, mediaStreams: false })).toBe(false);
  });
});
