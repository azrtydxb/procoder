// examples/true/before.ts — an unhandled empty-array edge and a swallowed
// write error.

interface Order { amount: number }

declare const db: { write(v: unknown): Promise<void> };

export async function saveOrderTotal(orders: Order[]): Promise<number> {
  const total = orders.reduce((sum, o) => sum + o.amount, 0) / orders.length;

  try {
    await db.write({ total });
  } catch (e) {}

  return total;
}
