// Phones: the "Hold to talk" button. The release goes to the callback that
// took the press, also when the button went away (or got a new callback)
// while held: the microphone must not stay on.
export const createHoldTalk = () => {
  let held: ((down: boolean) => void) | null = null;
  const release = () => {
    const talk = held;
    held = null;
    talk?.(false);
  };
  return {
    press: (talk: (down: boolean) => void) => {
      release();
      held = talk;
      talk(true);
    },
    release,
  };
};
