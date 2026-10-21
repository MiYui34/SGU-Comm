import { describe, expect, it } from "vitest";
import { headerParts } from "../src/lib/chatHeader";

const all = { topic: true, search: true, threads: true, pins: true, details: true };

describe("chat header parts", () => {
  it("show everything on wide screens", () => {
    expect(headerParts(false, all)).toEqual({ topic: true, searchBox: true, searchIcon: false, threads: true, pins: true });
  });

  it("move the topic, threads and pins to the channel sheet on phones", () => {
    expect(headerParts(true, all)).toEqual({ topic: false, searchBox: false, searchIcon: true, threads: false, pins: false });
  });

  it("keep them in the header on phones when there is no channel sheet", () => {
    expect(headerParts(true, { ...all, details: false })).toEqual({
      topic: true,
      searchBox: false,
      searchIcon: true,
      threads: true,
      pins: true,
    });
  });

  it("show only what the chat has", () => {
    const none = { topic: false, search: false, threads: false, pins: false, details: false };
    expect(headerParts(false, none)).toEqual({ topic: false, searchBox: false, searchIcon: false, threads: false, pins: false });
    expect(headerParts(true, none)).toEqual({ topic: false, searchBox: false, searchIcon: false, threads: false, pins: false });
  });
});
