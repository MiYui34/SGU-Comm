// The message box of a chat (see ChatPane).

// One upload at a time: no new file waits while one is sent.
export const canAttach = (uploads: boolean, uploading: boolean) => uploads && !uploading;

type Sent = { target: string; caption: string; file: File };
type Now = { target: string; draft: string; file: File | undefined; drafts: ReadonlyMap<string, string> };

// What goes after an upload finished: only a success takes the file and
// the caption (a failure leaves them for a retry), and only while they
// are still what was sent (the same chat, the caption not changed).
// kept: the caption waits as the draft of the chat we left; it goes too.
export const afterUpload = (ok: boolean, sent: Sent, now: Now) => {
  const here = now.target === sent.target;
  return {
    file: ok && now.file === sent.file,
    caption: ok && here && now.draft.trim() === sent.caption,
    kept: ok && !here && now.drafts.get(sent.target)?.trim() === sent.caption,
  };
};

// Another chat: the draft of the one we leave is kept (drafts: of the
// chats not open, by target), the other one's comes back.
export const switchDraft = (drafts: ReadonlyMap<string, string>, from: string, to: string, draft: string) => {
  const next = new Map(drafts);
  if (draft.trim()) next.set(from, draft);
  else next.delete(from);
  const back = next.get(to) ?? "";
  next.delete(to);
  return { drafts: next, draft: back };
};
