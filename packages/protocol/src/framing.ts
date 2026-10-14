// Mumble TCP framing: u16 type, u32 payload length (big endian), payload.

export const HEADER_SIZE = 6;
// The reference server drops clients that send frames above 8 MiB.
export const MAX_PAYLOAD = 8 * 1024 * 1024 - 1;

export type Frame = { type: number; payload: Uint8Array };

export const encodeFrame = (type: number, payload: Uint8Array): Uint8Array => {
  const out = new Uint8Array(HEADER_SIZE + payload.length);
  const view = new DataView(out.buffer);
  view.setUint16(0, type);
  view.setUint32(2, payload.length);
  out.set(payload, HEADER_SIZE);
  return out;
};

export const decodeFrames = (
  buf: Uint8Array,
): { frames: Frame[]; rest: Uint8Array } => {
  const view = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  const frames: Frame[] = [];
  let offset = 0;
  while (buf.length - offset >= HEADER_SIZE) {
    const type = view.getUint16(offset);
    const length = view.getUint32(offset + 2);
    if (length > MAX_PAYLOAD) throw new Error(`frame too large: ${length} bytes`);
    const end = offset + HEADER_SIZE + length;
    if (end > buf.length) break;
    frames.push({ type, payload: buf.slice(offset + HEADER_SIZE, end) });
    offset = end;
  }
  return { frames, rest: buf.slice(offset) };
};
