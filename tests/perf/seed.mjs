// Seeds the "general" text channel (id 1) of a built-in server's database:
// 400 messages from the load users, every 4th with one of 20 JPEGs
// (1280x720, about 70 KB, made with ffmpeg). Run with the server stopped,
// after it created the schema.
//
//   node seed.mjs <db> <upload dir>
//
// FFMPEG, SQLITE3: the binaries (default: from PATH).
import { execFileSync } from "node:child_process";
import { mkdirSync, statSync } from "node:fs";
import { join } from "node:path";

const [db, uploads] = process.argv.slice(2);
if (!db || !uploads) throw new Error("usage: node seed.mjs <db> <upload dir>");
mkdirSync(uploads, { recursive: true });

const IMAGES = 20;
const MESSAGES = 400;
const words = "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua".split(" ");
const start = Date.now() - MESSAGES * 60_000;
let sql = "BEGIN;\n";
for (let i = 0; i < IMAGES; i++) {
  const key = `seedimg${i}`;
  const file = join(uploads, key);
  execFileSync(process.env.FFMPEG || "ffmpeg", ["-loglevel", "error", "-y", "-f", "lavfi", "-i", `testsrc2=s=1280x720:d=1,noise=alls=${i * 3}:allf=t`, "-frames:v", "1", "-q:v", "4", "-f", "mjpeg", file]);
  sql += `INSERT OR REPLACE INTO uploads (id,key,name,type,size,author,created_at,used) VALUES (${1000 + i},'${key}','pic${i}.jpg','image/jpeg',${statSync(file).size},'load01',${start},1);\n`;
}
for (let i = 1; i <= MESSAGES; i++) {
  const text = Array.from({ length: 5 + ((i * 7) % 60) }, (_, j) => words[(i + j * 3) % words.length]).join(" ");
  const upload = i % 4 === 0 ? 1000 + ((i / 4) % IMAGES) : "NULL";
  sql += `INSERT OR REPLACE INTO messages (id,channel_id,author,text,created_at,upload_id,verified) VALUES (${i},1,'load${String(i % 25).padStart(2, "0")}','${text}',${start + i * 60_000},${upload},1);\n`;
}
sql += `INSERT OR REPLACE INTO store_meta (key,value) VALUES ('last_message_id',${MESSAGES});\nCOMMIT;\n`;
execFileSync(process.env.SQLITE3 || "sqlite3", [db], { input: sql });
console.log(`seeded ${MESSAGES} messages, ${IMAGES} images`);
