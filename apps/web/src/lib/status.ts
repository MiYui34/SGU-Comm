import type { ServerState } from "@kallisti/protocol";

// Null while the connection is healthy, otherwise a message for the user.
export const connectionError = (state: ServerState, closed: boolean): string | null => {
  if (state.status === "rejected") return state.error || "The server rejected the connection.";
  if (state.status === "kicked") return state.error || "You were removed from the server.";
  return closed ? "Lost the connection to the server." : null;
};

// Pages served over https must use wss, or the browser blocks the socket.
export const proxyUrl = ({ protocol, hostname }: { protocol: string; hostname: string }) =>
  `${protocol === "https:" ? "wss" : "ws"}://${hostname}:8080`;
