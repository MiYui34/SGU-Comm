// The message box of a chat (see ChatPane).

// One upload at a time: no new file waits while one is sent.
export const canAttach = (uploads: boolean, uploading: boolean) => uploads && !uploading;

type Sent = { target: string; caption: string; file: File };
type Now = { target: string; draft: string; file: File | undefined };

// What goes after an upload finished: only a success takes the file and
// the caption (a failure leaves them for a retry), and only while they
// are still what was sent (the same chat, the caption not changed).
export const afterUpload = (ok: boolean, sent: Sent, now: Now) => ({
  file: ok && now.file === sent.file,
  caption: ok && now.target === sent.target && now.draft.trim() === sent.caption,
});
