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

// A decoder that fails again and again without a picture cannot decode the
// stream (see above: H.264 without proprietary codecs). Give up after so
// many errors in a row instead of showing a black picture.
export const decodeWatch = (limit: number) => {
  let errors = 0;
  return {
    decoded: () => void (errors = 0),
    failed: () => ++errors >= limit,
  };
};

// Why a stream does not play. Streams from OBS are always H.264.
export const cannotPlay = (codec: string) =>
  codec.startsWith("avc1")
    ? "This browser cannot play H.264 video (streams from OBS are H.264). Try another browser, for example Chrome, Edge or Firefox."
    : "This browser cannot play this stream.";
