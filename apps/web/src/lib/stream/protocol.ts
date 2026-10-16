// The stream connection's binary frames (see the server's stream package):
// up from a streamer: flags (bit 0: keyframe), an 8-byte timestamp
// (microseconds), the encoded data; down to viewers with the streamer's
// session (4 bytes) in front.

export const encodeFrame = (key: boolean, timestamp: number, data: Uint8Array): Uint8Array<ArrayBuffer> => {
  const out = new Uint8Array(9 + data.length);
  const view = new DataView(out.buffer);
  out[0] = key ? 1 : 0;
  view.setBigUint64(1, BigInt(Math.max(0, Math.round(timestamp))));
  out.set(data, 9);
  return out;
};

export type Frame = { session: number; key: boolean; timestamp: number; data: Uint8Array };

export const decodeFrame = (buf: ArrayBuffer): Frame | null => {
  if (buf.byteLength < 13) return null;
  const view = new DataView(buf);
  return {
    session: view.getUint32(0),
    key: (view.getUint8(4) & 1) === 1,
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
