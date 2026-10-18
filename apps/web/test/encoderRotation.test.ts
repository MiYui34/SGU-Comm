import { describe, expect, it } from "vitest";
import { createEncoderRotation, type Closable } from "../src/lib/voice/encoderRotation";

const fakeEncoder = (id: number) => ({ id, closed: false, close() { this.closed = true; } });
type Fake = ReturnType<typeof fakeEncoder> & Closable;

const setup = () => {
  let n = 0;
  const made: Fake[] = [];
  const rot = createEncoderRotation<Fake, number>(() => {
    const e = fakeEncoder(++n);
    made.push(e);
    return e;
  });
  return { rot, made };
};

describe("encoder rotation", () => {
  it("keeps the encoder while it was not used", () => {
    const { rot } = setup();
    const first = rot.current();
    rot.startSpurt();
    expect(rot.current()).toBe(first);
  });

  it("gives each talk spurt a fresh encoder", () => {
    const { rot } = setup();
    const first = rot.current();
    rot.encoded(first, 1);
    rot.startSpurt();
    expect(rot.current()).not.toBe(first);
  });

  it("closes an old encoder only after its last output", () => {
    const { rot, made } = setup();
    const first = rot.current();
    rot.encoded(first, 1);
    rot.encoded(first, 2);
    rot.startSpurt();
    rot.output(first);
    expect(made[0]!.closed).toBe(false);
    rot.output(first); // the terminator of the old spurt
    expect(made[0]!.closed).toBe(true);
  });

  it("closes a finished old encoder at once", () => {
    const { rot, made } = setup();
    const first = rot.current();
    rot.encoded(first, 1);
    rot.output(first);
    rot.startSpurt();
    expect(made[0]!.closed).toBe(true);
  });

  it("makes a new encoder on renew, also when the old one was not used", () => {
    const { rot, made } = setup();
    rot.renew();
    expect(rot.current()).toBe(made[1]);
    expect(made[0]!.closed).toBe(true);
  });

  it("closes everything on close", () => {
    const { rot, made } = setup();
    rot.encoded(rot.current(), 1);
    rot.startSpurt();
    rot.closeAll();
    expect(made.every((e) => e.closed)).toBe(true);
  });

  it("matches outputs to inputs in order; extra outputs get nothing", () => {
    const { rot } = setup();
    const e = rot.current();
    rot.encoded(e, 10);
    rot.encoded(e, 20);
    expect(rot.output(e)).toBe(10);
    expect(rot.output(e)).toBe(20);
    expect(rot.output(e)).toBeUndefined(); // e.g. Chrome's packet after flush()
  });
});
