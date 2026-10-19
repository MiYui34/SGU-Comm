// The stream connection's binary frames (see the server's stream package):
// up from a streamer: flags (bit 0: keyframe, bit 1: sound, bit 2: the
// camera's), an 8-byte timestamp (microseconds; picture and sound on one
// clock), the encoded data; down to viewers with the stream's id (4 bytes)
// in front.

// A stream's id: its streamer's session for a screen share, with the top
// bit set for a camera.
const CAMERA_BIT = 2 ** 31;
export const streamId = (session: number, camera: boolean) => (camera ? session + CAMERA_BIT : session);
export const isCamera = (id: number) => id >= CAMERA_BIT;
export const ownerOf = (id: number) => (isCamera(id) ? id - CAMERA_BIT : id);

export const encodeFrame = (key: boolean, timestamp: number, data: Uint8Array, sound = false, camera = false): Uint8Array<ArrayBuffer> => {
  const out = new Uint8Array(9 + data.length);
  const view = new DataView(out.buffer);
  out[0] = (key ? 1 : 0) | (sound ? 2 : 0) | (camera ? 4 : 0);
  view.setBigUint64(1, BigInt(Math.max(0, Math.round(timestamp))));
  out.set(data, 9);
  return out;
};

export type Frame = { session: number; key: boolean; sound: boolean; timestamp: number; data: Uint8Array };

export const decodeFrame = (buf: ArrayBuffer): Frame | null => {
  if (buf.byteLength < 13) return null;
  const view = new DataView(buf);
  return {
    session: view.getUint32(0),
    key: (view.getUint8(4) & 1) === 1,
    sound: (view.getUint8(4) & 2) === 2,
    timestamp: Number(view.getBigUint64(5)),
    data: new Uint8Array(buf, 13),
  };
};

// The encoded size: the source's aspect, at most 1920x1080, even sizes
// (encoders need them).
export const fitSize = (width: number, height: number, maxWidth = 1920, maxHeight = 1080) => {
  const scale = Math.min(1, maxWidth / width, maxHeight / height);
  const even = (n: number) => Math.max(2, Math.floor((n * scale) / 2) * 2);
  return { width: even(width), height: even(height) };
};

// VP8 first: every WebCodecs implementation decodes it (H.264 decoding is
// missing in browsers without proprietary codecs, though they may encode
// it and claim to decode it). H.264 (Annex B: no decoder setup) else.
export const CODECS = ["vp8", "avc1.42E01F"] as const;

export const pickCodec = async (supported: (codec: string) => Promise<boolean>): Promise<string | null> => {
  for (const c of CODECS) if (await supported(c)) return c;
  return null;
};

// Picture and sound in sync: stream timestamps (microseconds) on the audio
// context's clock (seconds). The first sound sets the offset, played lead
// seconds ahead (a small buffer); pictures show when the clock reaches
// theirs. Sound that is too late is dropped; after a few late ones in a row
// (a delay that stays) or far ahead (clocks drifted), it starts over.
const LATE_IN_A_ROW = 5;
const FAR_AHEAD = 1; // seconds beyond the lead

export const createAvClock = (lead: number) => {
  let offset: number | null = null;
  let late = 0;
  return {
    // When to play sound with this timestamp (null: drop it).
    at: (timestamp: number, now: number): number | null => {
      const t = timestamp / 1e6;
      if (offset === null || late >= LATE_IN_A_ROW || t + offset < now - 1 || t + offset > now + lead + FAR_AHEAD) {
        offset = now + lead - t;
        late = 0;
      }
      const when = t + offset;
      if (when < now) {
        late++;
        return null;
      }
      late = 0;
      return when;
    },
    // When a picture with this timestamp is due (null: no sound yet).
    time: (timestamp: number): number | null => (offset === null ? null : timestamp / 1e6 + offset),
  };
};
