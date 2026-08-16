// examples/alone/before.ts — a deprecated function with no removal trigger,
// and a commented-out block left behind by whoever wrote the replacement.

/**
 * @deprecated
 */
export function createUserV1(name: string): { name: string; id: number } {
  return { name, id: Math.floor(Math.random() * 1e9) };
}

export function createUser(name: string): { name: string; id: string } {
  // const legacyId = Math.floor(Math.random() * 1e9);
  // const record = { name, id: legacyId };
  // return record;
  return { name, id: crypto.randomUUID() };
}
