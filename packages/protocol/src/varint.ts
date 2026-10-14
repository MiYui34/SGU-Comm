// Mumble's varint (PacketDataStream), used by the legacy voice packet format.
// The length is in the prefix bits of the first byte, not in a continuation bit.

const need = (buf: Uint8Array, end: number) => {
  if (end > buf.length) throw new Error("varint truncated");
};

const readBE = (buf: Uint8Array, start: number, length: number) => {
  need(buf, start + length);
  let value = 0;
  for (let i = 0; i < length; i++) value = value * 256 + buf[start + i]!;
  return value;
};

export const readVarint = (buf: Uint8Array, offset: number): { value: number; offset: number } => {
  need(buf, offset + 1);
  const b = buf[offset]!;
  const tail = (prefixMask: number, extra: number) => ({
    value: (b & prefixMask) * 256 ** extra + readBE(buf, offset + 1, extra),
    offset: offset + 1 + extra,
  });
  if ((b & 0x80) === 0x00) return { value: b, offset: offset + 1 };
  if ((b & 0xc0) === 0x80) return tail(0x3f, 1);
  if ((b & 0xe0) === 0xc0) return tail(0x1f, 2);
  if ((b & 0xf0) === 0xe0) return tail(0x0f, 3);
  switch (b & 0xfc) {
    case 0xf0:
      return tail(0, 4);
    case 0xf4:
      return tail(0, 8);
    case 0xf8: {
      const inner = readVarint(buf, offset + 1);
      return { value: -inner.value - 1, offset: inner.offset };
    }
    default:
      return { value: -(b & 0x03) - 1, offset: offset + 1 };
  }
};

const bytesBE = (value: number, length: number) =>
  Array.from({ length }, (_, i) => Math.floor(value / 256 ** (length - 1 - i)) % 256);

// Only non-negative values: the client never needs to send negative ones.
export const writeVarint = (value: number): Uint8Array => {
  if (!Number.isSafeInteger(value) || value < 0) throw new Error(`bad varint: ${value}`);
  if (value < 0x80) return Uint8Array.of(value);
  if (value < 0x4000) return Uint8Array.of(0x80 | (value >> 8), ...bytesBE(value, 1));
  if (value < 0x200000) return Uint8Array.of(0xc0 | (value >> 16), ...bytesBE(value, 2));
  if (value < 0x10000000) return Uint8Array.of(0xe0 | (value >>> 24), ...bytesBE(value, 3));
  if (value < 2 ** 32) return Uint8Array.of(0xf0, ...bytesBE(value, 4));
  return Uint8Array.of(0xf4, ...bytesBE(value, 8));
};
