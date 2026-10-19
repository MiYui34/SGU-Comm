import { describe, expect, it } from "vitest";
import {
  badPercent,
  biggerBuffer,
  connectionLabel,
  latePercent,
  speechLatePercent,
  meanVariance,
  qualityLevel,
  recent,
  transportOf,
} from "../src/lib/connectionQuality";

describe("recent", () => {
  it("keeps what is at most 30 s old", () => {
    const items = [{ at: 0 }, { at: 10_000 }, { at: 40_000 }];
    expect(recent(items, 40_000)).toEqual([{ at: 10_000 }, { at: 40_000 }]);
  });
});

describe("meanVariance", () => {
  it("gives the mean and the variance", () => {
    expect(meanVariance([40, 50, 60])).toEqual({ mean: 50, variance: 200 / 3 });
  });

  it("has nothing for no values", () => {
    expect(meanVariance([])).toBeNull();
  });
});

describe("latePercent", () => {
  it("counts late audio only", () => {
    expect(latePercent({ good: 100, late: 0, lost: 0 }, { good: 190, late: 6, lost: 4 })).toBe(6);
    expect(latePercent({ good: 5, late: 1, lost: 0 }, { good: 5, late: 1, lost: 0 })).toBeNull();
  });
});

describe("biggerBuffer", () => {
  it("suggests the next level from 2 % late audio", () => {
    expect(biggerBuffer(2, "low")).toBe("medium");
    expect(biggerBuffer(5, "medium")).toBe("high");
  });

  it("suggests nothing below that, without audio, or at the top", () => {
    expect(biggerBuffer(1.9, "low")).toBeNull();
    expect(biggerBuffer(null, "low")).toBeNull();
    expect(biggerBuffer(10, "high")).toBeNull();
  });
});

describe("badPercent", () => {
  it("counts late and lost audio between two snapshots", () => {
    expect(badPercent({ good: 100, late: 0, lost: 0 }, { good: 190, late: 6, lost: 4 })).toBe(10);
  });

  it("has nothing when no audio came", () => {
    expect(badPercent({ good: 5, late: 1, lost: 0 }, { good: 5, late: 1, lost: 0 })).toBeNull();
  });
});

describe("qualityLevel", () => {
  it("is good below 100 ms and 2 %", () => {
    expect(qualityLevel(99, 1.9)).toBe("good");
    expect(qualityLevel(40, null)).toBe("good");
  });

  it("is fair below 250 ms and 8 %", () => {
    expect(qualityLevel(100, 0)).toBe("fair");
    expect(qualityLevel(20, 2)).toBe("fair");
    expect(qualityLevel(249, 7.9)).toBe("fair");
  });

  it("is poor beyond", () => {
    expect(qualityLevel(250, 0)).toBe("poor");
    expect(qualityLevel(20, 8)).toBe("poor");
  });

  it("is unknown before the first ping came back", () => {
    expect(qualityLevel(null, 0)).toBeNull();
  });
});

describe("connectionLabel", () => {
  it("names the level and the ping", () => {
    expect(connectionLabel("good", 42.4)).toBe("Connection: good, 42 ms");
  });

  it("says when it does not know yet", () => {
    expect(connectionLabel(null, null)).toBe("Connection: measuring");
  });
});

describe("transportOf", () => {
  it("is the desktop bridge for its local address", () => {
    expect(transportOf("ws://127.0.0.1:4321/?secret=s&host=h&port=64738")).toBe("UDP or TCP (desktop app)");
  });

  it("is a WebSocket otherwise", () => {
    expect(transportOf("wss://kallisti.example/ws")).toBe("WebSocket");
  });
});

describe("speechLatePercent", () => {
  it("leaves out late audio of music speakers (the music buffer covers them)", () => {
    expect(speechLatePercent({ good: 100, late: 0, lost: 0, musicLate: 0 }, { good: 190, late: 6, lost: 4, musicLate: 5 })).toBe(1);
    expect(speechLatePercent({ good: 0, late: 0, lost: 0 }, { good: 98, late: 2, lost: 0 })).toBe(2);
  });
});
