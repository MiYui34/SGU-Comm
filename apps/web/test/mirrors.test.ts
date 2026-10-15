import { describe, expect, it } from "vitest";
import { channelTree, initialState, type Channel, type ServerState } from "@kallisti/protocol";
import { mirrorIds, withoutMirrors } from "../src/lib/mirrors";
import { initialTextState, type TextState } from "../src/lib/textChannels";

const ch = (id: number, parent: number | null, name: string, position = 0): Channel => ({
  id, parent, name, description: "", descriptionHash: null, position, temporary: false,
});
const server: ServerState = {
  ...initialState,
  channels: Object.fromEntries(
    [ch(0, null, "Root"), ch(1, 0, "General"), ch(5, 0, "Text channels", 1000), ch(6, 5, "#general"), ch(7, 5, "#random")].map(
      (c) => [c.id, c],
    ),
  ),
};
const text: TextState = { ...initialTextState, channels: [{ id: 10, name: "general", mumble: 6 }, { id: 11, name: "random", mumble: 7 }] };

describe("text channel mirrors", () => {
  it("are the mirrors and their parent", () => {
    expect([...mirrorIds(server, text.channels)].sort()).toEqual([5, 6, 7]);
    expect(mirrorIds(server, []).size).toBe(0);
  });

  it("do not show as voice channels", () => {
    const tree = withoutMirrors(channelTree(server), mirrorIds(server, text.channels));
    expect(tree?.children.map((n) => n.channel.name)).toEqual(["General"]);
  });
});
