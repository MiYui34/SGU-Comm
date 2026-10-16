import { decodeFrame, encodeFrame, fitSize, pickCodec } from "./protocol";
import Ticker from "./ticker.worker?worker";

// Screen sharing over the stream connection (/api/stream, see the server's
// stream package): the screen is encoded with WebCodecs and sent as frames;
// watched streams are decoded into canvases. One per voice connection.

const FPS = 15;
const BITRATE = 2_500_000;
const KEYFRAME_EVERY = FPS * 8; // frames; viewers also ask for keyframes
const MAX_BUFFERED = 1_500_000; // bytes waiting in the socket: skip frames
const MAX_DECODE_QUEUE = 10; // frames waiting in a decoder: skip to a keyframe
// Thumbnails for people not watching: small JPEGs, now and then.
const PREVIEW_EVERY_MS = 15_000;
const PREVIEW_FIRST_MS = 2_000;
const PREVIEW_WIDTH = 320;
const MAX_PREVIEW = 60_000; // characters of the data URL (the server takes 64 KiB)

type Viewer = { decoder: VideoDecoder | null; needKey: boolean; canvas: HTMLCanvasElement | null };
type Text = { type: string; session?: number; sessions?: number[]; codec?: string; message?: string };

export const streamingSupported = () =>
  typeof VideoEncoder !== "undefined" &&
  typeof VideoDecoder !== "undefined" &&
  typeof VideoFrame !== "undefined" &&
  !!navigator.mediaDevices?.getDisplayMedia;

export const createStreamClient = (url: string, token: () => Promise<string>) => {
  let ws: WebSocket | null = null;
  let opening: Promise<WebSocket> | null = null;
  let closed = false;
  // Sharing.
  let sharing = $state(false);
  let starting = false;
  let preview = $state<MediaStream | null>(null);
  let error = $state<string | null>(null);
  let screen: MediaStream | null = null;
  let encoder: VideoEncoder | null = null;
  let stopFrames: (() => void) | null = null;
  let forceKey = true;
  let started: ((err: string | null) => void) | null = null;
  // Who watches our stream (sessions), and whether others see thumbnails.
  let viewers = $state<number[]>([]);
  let previews = true;
  // Watching: by streamer session.
  const watched = new Map<number, Viewer>();
  let watching = $state<number[]>([]);

  // The connection: open once the server took the token ("ready").
  const open = (): Promise<WebSocket> => {
    if (ws?.readyState === WebSocket.OPEN) return Promise.resolve(ws);
    opening ??= (async () => {
      const t = await token();
      const sock = new WebSocket(url);
      sock.binaryType = "arraybuffer";
      await new Promise<void>((resolve, reject) => {
        sock.onopen = () => sock.send(JSON.stringify({ type: "auth", token: t }));
        sock.onmessage = (e) => typeof e.data === "string" && JSON.parse(e.data).type === "ready" && resolve();
        sock.onclose = () => reject(new Error("cannot connect for screen sharing"));
      });
      if (closed) {
        sock.close();
        throw new Error("closed");
      }
      sock.onmessage = (e) => (typeof e.data === "string" ? onText(JSON.parse(e.data)) : onFrame(e.data as ArrayBuffer));
      sock.onclose = () => {
        if (ws === sock) ws = null;
        started?.("the screen sharing connection closed");
        stopSharing(false);
        for (const s of [...watched.keys()]) unwatch(s, false);
      };
      ws = sock;
      return sock;
    })().finally(() => (opening = null));
    return opening;
  };

  const send = (v: object) => ws?.readyState === WebSocket.OPEN && ws.send(JSON.stringify(v));

  const onText = (m: Text) => {
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
      case "viewers":
        viewers = m.sessions ?? [];
        break;
      case "error":
        // About a stream we wanted to watch, or about our own.
        if (m.session !== undefined) unwatch(m.session, false);
        else if (started) started(m.message ?? "error");
        if (!started || m.session !== undefined) error = m.message ?? "error";
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
    if (sharing || starting) return; // a second click while the picker is open
    starting = true;
    error = null;
    try {
      const sock = await open();
      screen = await navigator.mediaDevices.getDisplayMedia({ video: { frameRate: FPS }, audio: false });
      const track = screen.getVideoTracks()[0]!;
      track.onended = () => stopSharing(true); // the browser's own "Stop sharing"
      const codec = await pickCodec(async (c) => (await VideoEncoder.isConfigSupported(encoderConfig(c, 1280, 720))).supported === true);
      if (!codec) throw new Error("this browser cannot encode video for screen sharing");
      const settings = track.getSettings();
      let size = fitSize(settings.width || 1280, settings.height || 720);
      const configure = () => {
        if (encoder?.state !== "closed") encoder?.close();
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
      let previewAt = performance.now() + PREVIEW_FIRST_MS;
      stopFrames = frames(track, (frame) => {
        if (previews && performance.now() >= previewAt) {
          previewAt = performance.now() + PREVIEW_EVERY_MS;
          const image = thumbnail(frame);
          if (image.length <= MAX_PREVIEW) send({ type: "preview", image });
        }
        if (!encoder || encoder.state !== "configured" || encoder.encodeQueueSize > 2 || sock.bufferedAmount > MAX_BUFFERED) {
          if (sock.bufferedAmount > MAX_BUFFERED) forceKey = true; // viewers miss frames: start whole again
          frame.close();
          return;
        }
        const next = fitSize(frame.displayWidth, frame.displayHeight);
        if (next.width !== size.width || next.height !== size.height) {
          size = next; // the shared window was resized
          configure();
          send({ type: "start", codec, ...size });
        }
        encoder.encode(frame, { keyFrame: forceKey || n % KEYFRAME_EVERY === 0 });
        frame.close();
        forceKey = false;
        n++;
      });
    } catch (e) {
      started = null;
      stopSharing(true);
      // Closing the browser's picker is not an error.
      if (!(e instanceof DOMException && e.name === "NotAllowedError")) error = e instanceof Error ? e.message : String(e);
    } finally {
      starting = false;
    }
  };

  const stopSharing = (tell: boolean) => {
    if (tell && sharing) send({ type: "stop" });
    stopFrames?.();
    stopFrames = null;
    if (encoder && encoder.state !== "closed") encoder.close();
    encoder = null;
    screen?.getTracks().forEach((t) => t.stop());
    screen = null;
    sharing = false;
    preview = null;
    viewers = [];
  };

  // Watching -----------------------------------------------------------

  const watch = async (session: number) => {
    error = null;
    if (watched.has(session)) return;
    watched.set(session, { decoder: null, needKey: true, canvas: null });
    watching = [...watched.keys()];
    try {
      await open();
      send({ type: "watch", session });
    } catch (e) {
      unwatch(session, false);
      error = e instanceof Error ? e.message : String(e);
    }
  };

  const configureIfSupported = async (session: number, codec: string) => {
    if (!(await VideoDecoder.isConfigSupported({ codec })).supported) {
      unwatch(session);
      error = "This browser cannot play this screen share.";
      return;
    }
    configure(session, codec);
  };

  // A decoder for a stream, then a keyframe to start from.
  const configure = (session: number, codec: string) => {
    const v = watched.get(session);
    if (!v) return;
    if (v.decoder && v.decoder.state !== "closed") v.decoder.close();
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
      error: () => watched.get(session) === v && configure(session, codec), // start over
    });
    v.decoder.configure({ codec, optimizeForLatency: true });
    send({ type: "keyframe", session });
  };

  const onFrame = (buf: ArrayBuffer) => {
    const f = decodeFrame(buf);
    const v = f && watched.get(f.session);
    if (!f || !v?.decoder || v.decoder.state !== "configured") return;
    if (!f.key && (v.needKey || v.decoder.decodeQueueSize > MAX_DECODE_QUEUE)) {
      if (!v.needKey) send({ type: "keyframe", session: f.session }); // too slow: skip ahead
      v.needKey = true;
      return;
    }
    v.needKey = false;
    v.decoder.decode(new EncodedVideoChunk({ type: f.key ? "key" : "delta", timestamp: f.timestamp, data: f.data }));
  };

  const unwatch = (session: number, tell = true) => {
    const v = watched.get(session);
    if (!v) return;
    if (tell) send({ type: "unwatch", session });
    if (v.decoder && v.decoder.state !== "closed") v.decoder.close();
    watched.delete(session);
    watching = [...watched.keys()];
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
    get viewers() {
      return viewers;
    },
    // Whether others see thumbnails of our stream (a setting).
    setPreviews: (on: boolean) => (previews = on),
    startSharing,
    stopSharing: () => stopSharing(true),
    clearError: () => (error = null),
    watch,
    unwatch: (session: number) => unwatch(session),
    // Where a watched stream is drawn (null: nowhere for now).
    attach: (session: number, canvas: HTMLCanvasElement | null) => {
      const v = watched.get(session);
      if (v) v.canvas = canvas;
    },
    close: () => {
      closed = true;
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

// The track's frames, at most FPS a second, until the returned stop. Where
// the browser has MediaStreamTrackProcessor (Chrome, Edge) frames come
// straight from the track, also in a hidden tab (the usual case: sharing
// another window). Elsewhere a hidden video element is drawn on a worker's
// timer (page timers are slowed in hidden tabs; worker timers less so).
type ProcessorCtor = new (init: { track: MediaStreamTrack }) => { readable: ReadableStream<VideoFrame> };

// A frame as a small JPEG data URL.
const thumbnail = (frame: VideoFrame): string => {
  const canvas = document.createElement("canvas");
  canvas.width = PREVIEW_WIDTH;
  canvas.height = Math.round((PREVIEW_WIDTH * frame.displayHeight) / Math.max(1, frame.displayWidth));
  canvas.getContext("2d")?.drawImage(frame, 0, 0, canvas.width, canvas.height);
  return canvas.toDataURL("image/jpeg", 0.6);
};

const frames = (track: MediaStreamTrack, onFrame: (frame: VideoFrame) => void): (() => void) => {
  const Processor = (globalThis as { MediaStreamTrackProcessor?: ProcessorCtor }).MediaStreamTrackProcessor;
  if (Processor) {
    const reader = new Processor({ track }).readable.getReader();
    let stopped = false;
    let last = -Infinity;
    void (async () => {
      while (!stopped) {
        const { value: frame, done } = await reader.read().catch(() => ({ value: undefined, done: true }));
        if (done || !frame) return;
        if (stopped || frame.timestamp - last < 1e6 / FPS - 2000) frame.close();
        else {
          last = frame.timestamp;
          onFrame(frame);
        }
      }
    })();
    return () => {
      stopped = true;
      void reader.cancel().catch(() => {});
    };
  }
  const video = document.createElement("video");
  video.muted = true;
  video.srcObject = new MediaStream([track]);
  void video.play().catch(() => {});
  const worker = new Ticker();
  worker.postMessage(1000 / FPS);
  worker.onmessage = () => {
    if (video.readyState >= 2) onFrame(new VideoFrame(video, { timestamp: performance.now() * 1000 }));
  };
  return () => {
    worker.terminate();
    video.srcObject = null;
  };
};
