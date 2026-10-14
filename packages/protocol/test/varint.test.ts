import { describe, expect, it } from "vitest";
import { readVarint, writeVarint } from "../src/varint";

const cases: [number, number[]][] = [
  [0, [0x00]],
  [0x7f, [0x7f]],
  [0x80, [0x80, 0x80]],
  [300, [0x81, 0x2c]],
  [0x3fff, [0xbf, 0xff]],
  [0x4000, [0xc0, 0x40, 0x00]],
  [0x1fffff, [0xdf, 0xff, 0xff]],
  [0x200000, [0xe0, 0x20, 0x00, 0x00]],
  [0xfffffff, [0xef, 0xff, 0xff, 0xff]],
  [0x10000000, [0xf0, 0x10, 0x00, 0x00, 0x00]],
  [0xffffffff, [0xf0, 0xff, 0xff, 0xff, 0xff]],
  [2 ** 32, [0xf4, 0, 0, 0, 1, 0, 0, 0, 0]],
];

describe("writeVarint", () => {
  it.each(cases)("encodes %d", (value, bytes) => {
    expect([...writeVarint(value)]).toEqual(bytes);
  });

  it("rejects negative and non-integer values", () => {
    expect(() => writeVarint(-1)).toThrow();
    expect(() => writeVarint(1.5)).toThrow();
  });
});

describe("readVarint", () => {
  it.each(cases)("decodes %d", (value, bytes) => {
    expect(readVarint(new Uint8Array(bytes), 0)).toEqual({ value, offset: bytes.length });
  });

  it("reads at an offset", () => {
    expect(readVarint(new Uint8Array([9, 9, 0x81, 0x2c, 7]), 2)).toEqual({
      value: 300,
      offset: 4,
    });
  });

  it("decodes the negative forms", () => {
    expect(readVarint(new Uint8Array([0xfc]), 0).value).toBe(-1);
    expect(readVarint(new Uint8Array([0xfd]), 0).value).toBe(-2);
    expect(readVarint(new Uint8Array([0xf8, 0x05]), 0).value).toBe(-6);
  });

  it("throws on truncated input", () => {
    expect(() => readVarint(new Uint8Array([0x81]), 0)).toThrow(/truncated/);
    expect(() => readVarint(new Uint8Array([]), 0)).toThrow(/truncated/);
  });
});
