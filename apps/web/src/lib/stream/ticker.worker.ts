// Ticks at the given interval (ms), from a worker: page timers are slowed
// in hidden tabs, worker timers less so (see client.svelte.ts).
let timer: ReturnType<typeof setInterval> | undefined;
self.onmessage = (e: MessageEvent<number>) => {
  clearInterval(timer);
  timer = setInterval(() => self.postMessage(0), e.data);
};
