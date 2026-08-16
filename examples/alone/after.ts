// examples/alone/after.ts — the old path is gone; the one function that must
// stay temporarily says exactly when it can go.

export function createUser(name: string): { name: string; id: string } {
  return { name, id: crypto.randomUUID() };
}

// procoder: remove after v3.0 — kept only until the mobile client stops
// requesting the legacy numeric id shape.
export function createUserLegacyIdShape(
  name: string
): { name: string; id: string; legacyId: number } {
  return { name, id: crypto.randomUUID(), legacyId: Math.floor(Math.random() * 1e9) };
}
