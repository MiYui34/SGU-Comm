// The tab title: the number of mentions and direct messages not read
// first, as in Discord ("(3) Kallisti").
export const tabTitle = (name: string, count: number) => (count > 0 ? `(${count > 99 ? "99+" : count}) ${name}` : name);
