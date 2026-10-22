import type { TextState } from "../lib/textChannels";

// 聊天里能显示的一行。
export type ChatLine = {
  id: number;
  author: string;
  text: string;
  at: number;
  // 这条在回复谁。被回复的那条要是删了，服务器给的名字会是空的。
  reply?: { author: string; text: string };
  // 带了一个文件。url 是服务器上的地址，点开就能下。
  upload?: { url: string; name: string; size: string };
};

// 消息正文是 HTML（Mumble 允许粗体、链接、换行）：列表里按纯文本显示，
// 免得把标签原样打出来，也免得放过一段没洗过的 HTML。
export function plainText(html: string): string {
  return html
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/(p|div|li|blockquote)>/gi, "\n")
    .replace(/<[^>]*>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, "&")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

// 文件大小说成人话：1023 → "1023 B"，2048 → "2 KB"。
export function sizeOf(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${Math.round(kb)} KB`;
  return `${(kb / 1024).toFixed(1)} MB`;
}

// 最近的几条，老的在上，和聊天软件一样。
export function recentLines(state: TextState, channel: number, take = 60): ChatLine[] {
  const messages = state.messages[channel] ?? [];
  return messages.slice(-take).map((message) => ({
    id: message.id,
    author: message.author,
    text: plainText(message.text),
    at: message.at,
    ...(message.reply ? { reply: { author: message.reply.author, text: plainText(message.reply.text) } } : {}),
    ...(message.upload
      ? {
          upload: {
            url: message.upload.url,
            name: message.upload.name,
            size: sizeOf(message.upload.size),
          },
        }
      : {}),
  }));
}

// 只显示时分：聊今天的事够了，跨天的日期在消息里很少见。
export function timeOf(at: number): string {
  return new Date(at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
}
