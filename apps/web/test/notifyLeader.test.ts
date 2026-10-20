import { describe, expect, it } from "vitest";
import { createNotifyLeader, leads, tabsAfter } from "../src/lib/notifyLeader";

describe("tabsAfter and leads", () => {
  it("the tab used last leads; ties go to the smaller id", () => {
    let tabs = tabsAfter({}, { type: "active", id: "a", at: 1 });
    tabs = tabsAfter(tabs, { type: "hello", id: "b", at: 2 });
    expect(leads(tabs, "b")).toBe(true);
    expect(leads(tabs, "a")).toBe(false);
    tabs = tabsAfter(tabs, { type: "active", id: "a", at: 2 });
    expect(leads(tabs, "a")).toBe(true);
    tabs = tabsAfter(tabs, { type: "bye", id: "a" });
    expect(leads(tabs, "b")).toBe(true);
    expect(leads({}, "c")).toBe(true); // alone
  });
});

const settle = () => new Promise((r) => setTimeout(r, 20));

describe("createNotifyLeader", () => {
  it("lets one tab per account notify, the one used last", async () => {
    let t = 1;
    const now = () => t++;
    const a = createNotifyLeader("kallisti-test", now);
    await settle();
    const b = createNotifyLeader("kallisti-test", now);
    const other = createNotifyLeader("kallisti-other", now); // another account
    await settle();
    expect([a.leads(), b.leads(), other.leads()]).toEqual([false, true, true]);
    a.touch();
    await settle();
    expect([a.leads(), b.leads()]).toEqual([true, false]);
    a.close();
    await settle();
    expect(b.leads()).toBe(true);
    b.close();
    other.close();
  });
});
