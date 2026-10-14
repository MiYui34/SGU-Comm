// A limiter before the speakers: user volumes go up to 200 %, and a loud
// speaker at that gain would clip. The browser's compressor adds make-up
// gain (1.14 at these settings, measured in Chrome), which would make all
// quiet audio louder; a fixed gain after it cancels that. Measured: a
// full-scale tone at gain 2 peaks at 0.85; quiet audio passes unchanged.
const MAKEUP = 1.1402;

export const createLimiter = (ctx: BaseAudioContext) => {
  const input = new DynamicsCompressorNode(ctx, { threshold: -2, knee: 0, ratio: 20, attack: 0.001, release: 0.1 });
  const output = new GainNode(ctx, { gain: 1 / MAKEUP });
  input.connect(output);
  return { input, output };
};
