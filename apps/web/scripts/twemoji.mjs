// Puts the Twemoji PNGs (72x72) into public/emoji, from the pinned release
// (checked by its SHA-256). Runs before builds; does nothing when this
// version is there. Without network the build goes on and emoji show as
// text, except in CI (CI set): there it fails.
// Graphics: CC-BY 4.0, see public/third-party-notices.txt.
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const VERSION = "17.0.2";
const SHA256 = "3678d74fc4e793b76cbc810c77195d08877b97afdcc9e8f1a812579d854dccc8";
const URL = `https://github.com/jdecked/twemoji/archive/refs/tags/v${VERSION}.tar.gz`;
const publicDir = join(import.meta.dirname, "..", "public");
const out = join(publicDir, "emoji");
const marker = join(out, ".version");

if (existsSync(marker) && readFileSync(marker, "utf8") === VERSION) process.exit(0);
// Next to the target, so the finished folder is moved in at once.
const tmp = mkdtempSync(join(publicDir, ".twemoji-"));
try {
  const res = await fetch(URL, { signal: AbortSignal.timeout(60_000) });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const data = Buffer.from(await res.arrayBuffer());
  const sum = createHash("sha256").update(data).digest("hex");
  if (sum !== SHA256) throw new Error(`checksum ${sum}, want ${SHA256}`);
  writeFileSync(join(tmp, "t.tar.gz"), data);
  const images = join(tmp, "emoji");
  mkdirSync(images);
  execFileSync("tar", ["-xzf", join(tmp, "t.tar.gz"), "-C", images, "--strip-components=3", `twemoji-${VERSION}/assets/72x72`]);
  writeFileSync(join(images, ".version"), VERSION);
  rmSync(out, { recursive: true, force: true });
  renameSync(images, out);
  console.log(`twemoji ${VERSION}: images in public/emoji`);
} catch (err) {
  if (process.env.CI) throw err;
  console.warn(`twemoji: no images (${err.message}); emoji show as text`);
} finally {
  rmSync(tmp, { recursive: true, force: true });
}
