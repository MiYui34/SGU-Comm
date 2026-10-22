// 假的 Minecraft 服务器，只为本地试接口：照服务器列表 ping 的协议回一份状态。
// 用法：node dev/fake-mc.mjs（环境变量 MC_PORT / MC_NAMES / MC_MAX / MC_DELAY_MS）。
import net from "node:net";

const port = Number(process.env.MC_PORT ?? 25566);
const names = (process.env.MC_NAMES ?? "Kernel,Ender,Buildar").split(",").filter(Boolean);
const max = Number(process.env.MC_MAX ?? 50);
// 故意慢一点，好看出「预计游戏延迟」里服务器到游戏服的那一段。
const delay = Number(process.env.MC_DELAY_MS ?? 0);

const version = "1.21.4";
const status = JSON.stringify({
  version: { name: version, protocol: 769 },
  players: {
    max,
    online: names.length,
    sample: names.map((name, index) => ({ name, id: `00000000-0000-0000-0000-0000000000${index}` })),
  },
  description: { text: "SGU 生存服" },
});

function writeVarint(value) {
  const bytes = [];
  let rest = value >>> 0;
  while (rest >= 0x80) {
    bytes.push((rest & 0x7f) | 0x80);
    rest >>>= 7;
  }
  bytes.push(rest);
  return Buffer.from(bytes);
}

function readVarint(buffer, at) {
  let value = 0;
  for (let i = 0; i < 5; i += 1) {
    if (at + i >= buffer.length) return { value: null, size: 0 };
    const byte = buffer[at + i];
    value |= (byte & 0x7f) << (7 * i);
    if ((byte & 0x80) === 0) return { value, size: i + 1 };
  }
  return { value: null, size: 0 };
}

function packet(body) {
  return Buffer.concat([writeVarint(body.length), body]);
}

const server = net.createServer((socket) => {
  let buffer = Buffer.alloc(0);
  let packets = 0;

  socket.on("data", (chunk) => {
    buffer = Buffer.concat([buffer, chunk]);
    for (;;) {
      const { value: length, size } = readVarint(buffer, 0);
      if (length === null || buffer.length < size + length) return;
      buffer = buffer.subarray(size + length);
      packets += 1;
      if (packets < 2) continue; // 先是握手，再是状态请求
      const body = Buffer.concat([Buffer.from([0x00]), writeVarint(Buffer.byteLength(status)), Buffer.from(status)]);
      if (delay > 0) {
        setTimeout(() => socket.end(packet(body)), delay);
        return;
      }
      socket.end(packet(body));
      return;
    }
  });

  socket.on("error", () => {});
});

server.listen(port, "127.0.0.1", () => {
  console.log(`fake minecraft on 127.0.0.1:${port}: ${names.length}/${max} online (${version})`);
});
