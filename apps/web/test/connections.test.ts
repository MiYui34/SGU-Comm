import { describe, expect, it } from "vitest";
import {
  connectionLevel,
  formatOnline,
  formatPercent,
  formatPing,
  parseConnections,
  worstFirst,
  type Connection,
} from "../src/lib/connections";
import { initialTextState, manageRequest, parseServerEvent, reduceText } from "../src/lib/textChannels";

const conn = (c: Partial<Connection>): Connection => ({
  session: 1,
  name: "a",
  transport: "udp",
  ping: 20,
  fromClient: 0,
  fromServer: 0,
  dropped: 0,
  overflows: 0,
  online: 60,
  client: "1.5.634",
  os: "Linux",
  ...c,
});

describe("connectionLevel", () => {
  it("takes the worse of ping and late or lost voice in either direction", () => {
    expect(connectionLevel(conn({}))).toBe("good");
    expect(connectionLevel(conn({ ping: 120 }))).toBe("fair");
    expect(connectionLevel(conn({ fromServer: 9 }))).toBe("poor");
    expect(connectionLevel(conn({ fromClient: 3, fromServer: null }))).toBe("fair");
  });
  it("judges by what is known", () => {
    expect(connectionLevel(conn({ ping: null, fromServer: 9 }))).toBe("poor");
    expect(connectionLevel(conn({ ping: null, fromClient: null, fromServer: null }))).toBeNull();
  });
});

describe("worstFirst", () => {
  it("sorts by level, then by ping, unknown last", () => {
    const list = [
      conn({ session: 1, ping: 20 }),
      conn({ session: 2, ping: null, fromClient: null, fromServer: null }),
      conn({ session: 3, ping: 300 }),
      conn({ session: 4, ping: 40 }),
      conn({ session: 5, ping: 10, fromServer: 20 }),
    ];
    expect(worstFirst(list).map((c) => c.session)).toEqual([3, 5, 4, 1, 2]);
    expect(list[0]!.session).toBe(1); // a copy
  });
});

describe("formatting", () => {
  it("shows percentages and times compactly", () => {
    expect(formatPing(null)).toBe("–");
    expect(formatPing(42.6)).toBe("43 ms");
    expect(formatPercent(null)).toBe("–");
    expect(formatPercent(0)).toBe("0 %");
    expect(formatPercent(1.26)).toBe("1.3 %");
    expect(formatPercent(12.4)).toBe("12 %");
    expect(formatOnline(59)).toBe("59 s");
    expect(formatOnline(125)).toBe("2 min");
    expect(formatOnline(3 * 3600 + 120)).toBe("3 h 2 min");
    expect(formatOnline(2 * 86400 + 3600)).toBe("2 d 1 h");
  });
});

describe("parseConnections", () => {
  it("keeps valid entries only", () => {
    expect(parseConnections([conn({}), { session: "x" }])).toEqual([conn({})]);
    expect(parseConnections("nope")).toBeNull();
  });
  it("is a text event and a request", () => {
    const s = reduceText(initialTextState, parseServerEvent(JSON.stringify({ type: "connections", connections: [conn({})] }))!);
    expect(s.connections).toEqual([conn({})]);
    expect(parseServerEvent('{"type":"connections"}')).toBeNull();
    expect(JSON.parse(manageRequest({ connections: true }))).toEqual({ type: "connections" });
  });
});
