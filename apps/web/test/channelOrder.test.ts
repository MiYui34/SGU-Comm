import { describe, expect, it } from "vitest";
import { moveInList, moveVoiceChannel } from "../src/lib/channelOrder";

describe("moveInList", () => {
  it("moves an id one step, and not past the ends", () => {
    expect(moveInList([1, 2, 3], 3, -1)).toEqual([1, 3, 2]);
    expect(moveInList([1, 2, 3], 1, 1)).toEqual([2, 1, 3]);
    expect(moveInList([1, 2, 3], 1, -1)).toEqual([1, 2, 3]);
    expect(moveInList([1, 2, 3], 9, 1)).toEqual([1, 2, 3]);
  });
});

describe("moveVoiceChannel", () => {
  // Siblings as the sidebar shows them; Mumble sorts by position, then name.
  const siblings = [
    { id: 5, position: 0 },
    { id: 6, position: 0 },
    { id: 7, position: 0 },
  ];

  it("numbers the siblings in the new order, changing only what differs", () => {
    expect(moveVoiceChannel(siblings, 7, -1)).toEqual([
      { id: 7, position: 1 },
      { id: 6, position: 2 },
    ]);
  });

  it("does nothing at the ends", () => {
    expect(moveVoiceChannel(siblings, 5, -1)).toEqual([]);
  });

  it("keeps positions that are already right", () => {
    const numbered = [
      { id: 5, position: 0 },
      { id: 6, position: 1 },
      { id: 7, position: 2 },
    ];
    expect(moveVoiceChannel(numbered, 6, 1)).toEqual([
      { id: 7, position: 1 },
      { id: 6, position: 2 },
    ]);
  });
});
