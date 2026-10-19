import type { View } from "./place";

// The message box of a chat (see ChatPane).

// One upload at a time: no new file waits while one is sent.
export const canAttach = (uploads: boolean, uploading: boolean) => uploads && !uploading;

// Files in a paste: some engines give a pasted image only as an item, not
// in files.
type Pasted = { files: ArrayLike<File>; items: ArrayLike<{ kind: string; getAsFile(): File | null }> };
export const pastedFiles = (data: Pasted | null | undefined): File[] => {
  if (!data) return [];
  if (data.files.length) return Array.from(data.files);
  return Array.from(data.items).flatMap((i) => (i.kind === "file" ? (i.getAsFile() ?? []) : []));
};

// WebKitGTK (the Linux app) gives the page nothing on an image paste: no
// files, no items, no types. The clipboard API still reads the image then.
export const emptyPaste = (data: { types: ArrayLike<string> } | null | undefined) =>
  !Array.from(data?.types ?? []).some((t) => t === "Files" || t === "text/plain");

type Read = () => Promise<{ types: readonly string[]; getType(type: string): Promise<Blob> }[]>;
// The first image on the clipboard as a file, or null (also without
// clipboard access).
export const clipboardImage = async (read: Read): Promise<File | null> => {
  try {
    for (const item of await read()) {
      const type = item.types.find((t) => t.startsWith("image/"));
      if (type) return new File([await item.getType(type)], `pasted.${type.split(/[/+]/)[1]}`, { type });
    }
  } catch {
    // no clipboard API or no access
  }
  return null;
};

// A send the server refused comes back; it fills the box only if the user
// has not typed something new.
export const refusedDraft = (draft: string, text: string) => (draft.trim() ? draft : text);

// A file over the server's limit (none known: the server decides) is
// refused before it is sent.
const mb = (bytes: number) => `${+(bytes / 1024 / 1024).toFixed(1)} MB`;
export const tooLarge = (size: number, max: number | undefined) =>
  max && size > max ? `The file is too large (at most ${mb(max)}).` : null;
// The upload button's tooltip, with the limit if known.
export const uploadTitle = (max: number | undefined) => (max ? `Upload a file (up to ${mb(max)})` : "Upload a file");

// The open chat, for its draft: channels by id (voice channel names
// repeat in the tree, a rename keeps the draft), private chats by name.
export const chatKeyOf = (view: View, voiceChannel: number) =>
  view.kind === "text" ? `text ${view.id}` : view.kind === "dm" ? `dm ${view.name}` : `voice ${voiceChannel}`;

type Sent = { chat: string; caption: string; file: File };
type Now = { chat: string; draft: string; file: File | undefined; drafts: ReadonlyMap<string, string> };

// What goes after an upload finished: only a success takes the file and
// the caption (a failure leaves them for a retry), and only while they
// are still what was sent (the same chat, the caption not changed). A
// caption kept as the draft of the chat we left goes from `drafts`.
export const afterUpload = (ok: boolean, sent: Sent, now: Now) => {
  const here = now.chat === sent.chat;
  const kept = ok && !here && now.drafts.get(sent.chat)?.trim() === sent.caption;
  return {
    file: ok && now.file === sent.file,
    caption: ok && here && now.draft.trim() === sent.caption,
    drafts: kept ? new Map([...now.drafts].filter(([chat]) => chat !== sent.chat)) : now.drafts,
  };
};

// Another chat: the draft of the one we leave is kept (drafts: of the
// chats not open, by chat key), the other one's comes back.
export const switchDraft = (drafts: ReadonlyMap<string, string>, from: string, to: string, draft: string) => {
  const next = new Map(drafts);
  if (draft.trim()) next.set(from, draft);
  else next.delete(from);
  const back = next.get(to) ?? "";
  next.delete(to);
  return { drafts: next, draft: back };
};
