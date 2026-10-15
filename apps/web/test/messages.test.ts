import { describe, expect, it } from "vitest";
import type { ChatMessage, User } from "@kallisti/protocol";
import { ownsMessage } from "../src/lib/messages";

const line = (actorName: string, verified: boolean, at = 1000) => ({ actorName, verified, at }) as ChatMessage & { verified: boolean };
const user = (name: string, userId: number | null) => ({ name, userId }) as User;

describe("ownsMessage", () => {
  it("registered users own their verified messages", () => {
    expect(ownsMessage(line("Lain", true), user("lain", 4), 5000)).toBe(true);
    expect(ownsMessage(line("lain", false), user("lain", 4), 0)).toBe(false);
  });

  it("guests own unverified messages since they joined", () => {
    expect(ownsMessage(line("guest", false, 6000), user("guest", null), 5000)).toBe(true);
    expect(ownsMessage(line("guest", false, 4000), user("guest", null), 5000)).toBe(false);
    expect(ownsMessage(line("guest", true, 6000), user("guest", null), 5000)).toBe(false);
  });

  it("nobody owns others' messages", () => {
    expect(ownsMessage(line("bob", true), user("lain", 4), 0)).toBe(false);
    expect(ownsMessage(line("bob", true), undefined, 0)).toBe(false);
  });
});
