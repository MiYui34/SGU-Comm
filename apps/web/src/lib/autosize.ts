// A Svelte action: a textarea as tall as its text (CSS max-height caps it,
// then it scrolls). Pass the value, so it fits again when it changes; after
// the update, when the new value is in the element.
export const autosize = (node: HTMLTextAreaElement, _value?: string) => {
  const fit = () => {
    node.style.height = "auto";
    node.style.height = `${node.scrollHeight}px`;
  };
  fit();
  return { update: () => queueMicrotask(fit) };
};
