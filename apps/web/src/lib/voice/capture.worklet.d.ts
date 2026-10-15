// Types of the worklet's helpers (the worklet itself is plain JavaScript).
export type Denoiser = { process(frame: Float32Array): number; destroy(): void };
export function createDenoiser(module: WebAssembly.Module): Denoiser;
export class Fifo {
  constructor(size: number);
  push(samples: ArrayLike<number>): void;
  pull(out: Float32Array): void;
}
