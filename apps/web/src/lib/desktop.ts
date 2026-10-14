// The desktop app (Tauri) connects through a local bridge in the app, to any
// Mumble server, without the Kallisti server.

export type BridgeInfo = { port: number; secret: string };
export type ServerAddress = { host: string; port: number };

const DEFAULT_PORT = 64738;

// "host", "host:port", "[v6]:port" or a bare IPv6 address.
export const parseAddress = (input: string): ServerAddress | null => {
  const text = input.trim();
  if (!text) return null;
  const bracketed = /^\[([^\]]+)\](?::(\d+))?$/.exec(text);
  const [host, portText] = bracketed
    ? [bracketed[1]!, bracketed[2]]
    : text.split(":").length === 2
      ? (text.split(":") as [string, string])
      : [text, undefined];
  if (portText !== undefined && !/^\d+$/.test(portText)) return null;
  const port = portText === undefined ? DEFAULT_PORT : Number(portText);
  if (!host || port < 1 || port > 65535) return null;
  return { host, port };
};

export const bridgeUrl = (bridge: BridgeInfo, server: ServerAddress) => {
  const params = new URLSearchParams({ secret: bridge.secret, host: server.host, port: String(server.port) });
  return `ws://127.0.0.1:${bridge.port}/?${params}`;
};

// Running inside the Tauri app?
export const isDesktop = () => typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;

export const getBridge = async (): Promise<BridgeInfo> => {
  const { invoke } = await import("@tauri-apps/api/core");
  return invoke<BridgeInfo>("bridge_info");
};
