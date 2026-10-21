// The chat header's items (see ChatPane), by what the chat has. Phones
// keep the menu, the name, a search icon and the member list, as in
// Discord's phone app; the topic, threads and pins go to the channel
// sheet (details), when there is one.
export type HeaderHas = { topic: boolean; search: boolean; threads: boolean; pins: boolean; details: boolean };

export const headerParts = (phone: boolean, has: HeaderHas) => {
  const inSheet = phone && has.details;
  return {
    topic: has.topic && !inSheet,
    searchBox: has.search && !phone,
    searchIcon: has.search && phone,
    threads: has.threads && !inSheet,
    pins: has.pins && !inSheet,
  };
};
