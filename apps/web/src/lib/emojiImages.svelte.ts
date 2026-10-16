import { EMOJI_BASE } from "./emoji";

// Whether the Twemoji images are there (a build without them shows emoji
// as text): one probe at start.
export const emojiImages = $state({ ok: true });
if (typeof Image !== "undefined") {
  const probe = new Image();
  probe.onerror = () => (emojiImages.ok = false);
  probe.src = `${EMOJI_BASE}1f600.png`;
}
