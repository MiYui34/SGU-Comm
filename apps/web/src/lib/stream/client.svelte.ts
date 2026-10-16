import { decodeFrame, encodeFrame, fitSize, pickCodec } from "./protocol";

// Screen sharing over the stream connection (/api/stream, see the server's
// stream package): the screen is encoded with WebCodecs and sent as frames;
// watched streams are decoded into canvases. One per voice connection.

const FPS = 15;
const BITRATE = 2_500_000;
const KEYFRAME_EVERY = FPS * 8; // frames; viewers also ask for keyframes
const MAX_BUFFERED = 1_500_000; // bytes waiting in the socket: skip frames

type Viewer = { decoder: VideoDecoder | null; needKey: boolean; canvas: HTMLCanvasElement | null };

export const streamingSupported = () =>
  typeof VideoEncoder !== "undefined" &&
  typeof VideoDecoder !== "undefined" &&
  typeof VideoFrame !== "undefined" &&
  !!navigator.mediaDevices?.getDisplayMedia;

export const createStreamClient = (url: string, token: () => Promise<string>) => {
  let ws: WebSocket | null = null;
  let opening: Promise<WebSocket> | null = null;
  // Sharing.
  let sharing = $state(false);
  let preview = $state<MediaStream | null>(null);
  let error = $state<string | null>(null);
  let screen: MediaStream | null = null;
  let encoder: VideoEncoder | null = null;
  let timer: ReturnType<typeof setInterval> | undefined;
  let forceKey = true;
  let started: ((err: string | null) => void) | null = null;
  // Watching: by streamer session.
  const viewers = new Map<number, Viewer>();
  let watching = $state<number[]>([]);

  const open = (): Promise<WebSocket> => {
    if (ws && ws.readyState === WebSocket.OPEN) return Promise.resolve(ws);
    opening ??= (async () => {
      const t = await token();
      const sock = new WebSocket(url);
      sock.binaryType = "arraybuffer";
      await new Promise<void>((resolve, reject) => {
        sock.onopen = () => resolve();
        sock.onerror = () => reject(new Error("cannot connect for screen sharing"));
      });
      sock.send(JSON.stringify({ type: "auth", token: t }));
      sock.onmessage = (e) => (typeof e.data === "string" ? onText(JSON.parse(e.data)) : onFrame(e.data as ArrayBuffer));
      sock.onclose = () => {
        if (ws === sock) ws = null;
        stopSharing(false);
        for (const s of [...viewers.keys()]) unwatch(s, false);
      };
      ws = sock;
      return sock;
    })().finally(() => (opening = null));
    return opening;
  };

  const send = (v: object) => ws?.readyState === WebSocket.OPEN && ws.send(JSON.stringify(v));

  const onText = (m: { type: string; session?: number; codec?: string; message?: string }) => {
    switch (m.type) {
      case "started":
        started?.(null);
        break;
      case "keyframe":
        forceKey = true;
        break;
      case "stopped":
        stopSharing(false);
        break;
      case "error":
        if (started) started(m.message ?? "error");
        else error = m.message ?? "error";
        if (m.session !== undefined) unwatch(m.session, false);
        break;
      case "stream":
        if (m.session !== undefined && m.codec) void configureIfSupported(m.session, m.codec);
        break;
      case "ended":
        if (m.session !== undefined) unwatch(m.session, false);
        break;
    }
  };

  // Sharing ------------------------------------------------------------

  const startSharing = async () => {
    error = null;
    try {
      const sock = await open();
      screen = await navigator.mediaDevices.getDisplayMedia({ video: { frameRate: FPS }, audio: false });
      const track = screen.getVideoTracks()[0]!;
      track.onended = () => stopSharing(true); // the browser's own "Stop sharing"
      const video = document.createElement("video");
      video.muted = true;
      video.srcObject = screen;
      await video.play();
      const codec = await pickCodec(async (c) => (await VideoEncoder.isConfigSupported(encoderConfig(c, 1280, 720))).supported === true);
      if (!codec) throw new Error("this browser cannot encode video for screen sharing");
      let size = fitSize(video.videoWidth || 1280, video.videoHeight || 720);
      const configure = () => {
        encoder?.close();
        encoder = new VideoEncoder({
          output: (chunk) => {
            const data = new Uint8Array(chunk.byteLength);
            chunk.copyTo(data);
            if (sock.readyState === WebSocket.OPEN) sock.send(encodeFrame(chunk.type === "key", chunk.timestamp, data));
          },
          error: (e) => (error = `Video encoder failed: ${e.message}`),
        });
        encoder.configure(encoderConfig(codec, size.width, size.height));
        forceKey = true;
      };
      configure();
      const err = await new Promise<string | null>((resolve) => {
        started = resolve;
        send({ type: "start", codec, ...size });
      });
      started = null;
      if (err) throw new Error(err);
      sharing = true;
      preview = screen;
      let n = 0;
      timer = setInterval(() => {
        if (!encoder || encoder.state !== "configured" || encoder.encodeQueueSize > 2) return;
        if (sock.bufferedAmount > MAX_BUFFERED) {
          forceKey = true; // the viewers miss frames: start whole again
          return;
        }
        const next = fitSize(video.videoWidth || size.width, video.videoHeight || size.height);
        if (next.width !== size.width || next.height !== size.height) {
          size = next; // the shared window was resized
          configure();
          send({ type: "start", codec, ...size });
        }
        const frame = new VideoFrame(video, { timestamp: performance.now() * 1000 });
        encoder.encode(frame, { keyFrame: forceKey || n % KEYFRAME_EVERY === 0 });
        frame.close();
        forceKey = false;
        n++;
      }, 1000 / FPS);
    } catch (e) {
      stopSharing(true);
      // Closing the browser's picker is not an error.
      if (!(e instanceof DOMException && e.name === "NotAllowedError")) error = e instanceof Error ? e.message : String(e);
    }
  };

  const stopSharing = (tell: boolean) => {
    if (tell && sharing) send({ type: "stop" });
    clearInterval(timer);
    timer = undefined;
    if (encoder?.state !== "closed") encoder?.close();
    encoder = null;
    screen?.getTracks().forEach((t) => t.stop());
    screen = null;
    sharing = false;
    preview = null;
  };

  // Watching -----------------------------------------------------------

  const watch = async (session: number) => {
    error = null;
    if (viewers.has(session)) return;
    viewers.set(session, { decoder: null, needKey: true, canvas: null });
    watching = [...viewers.keys()];
    try {
      await open();
      send({ type: "watch", session });
    } catch (e) {
      unwatch(session, false);
      error = e instanceof Error ? e.message : String(e);
    }
  };

  const configureIfSupported = async (session: number, codec: string) => {
    if ((await VideoDecoder.isConfigSupported({ codec })).supported) return configure(session, codec);
    unwatch(session);
    error = "This browser cannot play this screen share.";
  };

  const configure = (session: number, codec: string) => {
    const v = viewers.get(session);
    if (!v) return;
    v.decoder?.close();
    v.needKey = true;
    v.decoder = new VideoDecoder({
      output: (frame) => {
        const c = v.canvas;
        if (c) {
          if (c.width !== frame.displayWidth || c.height !== frame.displayHeight) {
            c.width = frame.displayWidth;
            c.height = frame.displayHeight;
          }
          c.getContext("2d")?.drawImage(frame, 0, 0);
        }
        frame.close();
      },
      error: () => configure(session, codec), // start over at the next keyframe
    });
    v.decoder.configure({ codec, optimizeForLatency: true });
  };

  const onFrame = (buf: ArrayBuffer) => {
    const f = decodeFrame(buf);
    const v = f && viewers.get(f.session);
    if (!f || !v?.decoder || v.decoder.state !== "configured") return;
    if (v.needKey && !f.key) return;
    v.needKey = false;
    v.decoder.decode(new EncodedVideoChunk({ type: f.key ? "key" : "delta", timestamp: f.timestamp, data: f.data }));
  };

  const unwatch = (session: number, tell = true) => {
    const v = viewers.get(session);
    if (!v) return;
    if (tell) send({ type: "unwatch", session });
    if (v.decoder?.state !== "closed") v.decoder?.close();
    viewers.delete(session);
    watching = [...viewers.keys()];
  };

  return {
    get sharing() {
      return sharing;
    },
    get preview() {
      return preview;
    },
    get error() {
      return error;
    },
    get watching() {
      return watching;
    },
    startSharing,
    stopSharing: () => stopSharing(true),
    watch,
    unwatch: (session: number) => unwatch(session),
    // Where a watched stream is drawn (null: nowhere for now).
    attach: (session: number, canvas: HTMLCanvasElement | null) => {
      const v = viewers.get(session);
      if (v) v.canvas = canvas;
    },
    close: () => {
      stopSharing(true);
      ws?.close();
    },
  };
};

export type StreamClient = ReturnType<typeof createStreamClient>;

const encoderConfig = (codec: string, width: number, height: number): VideoEncoderConfig => ({
  codec,
  width,
  height,
  bitrate: BITRATE,
  framerate: FPS,
  latencyMode: "realtime",
  ...(codec.startsWith("avc1") ? { avc: { format: "annexb" as const } } : {}),
});
