// examples/true/after.ts — the empty case is guarded, and the write failure
// is logged with a correlation id before being rethrown.

import assert from 'node:assert';

interface Order { amount: number }

declare const db: { write(v: unknown): Promise<void> };
declare const logger: { error(message: string, context: unknown): void };
declare function currentCorrelationId(): string;

function averageAmount(orders: Order[]): number {
  if (orders.length === 0) return 0;
  return orders.reduce((sum, o) => sum + o.amount, 0) / orders.length;
}

export async function saveOrderTotal(orders: Order[]): Promise<number> {
  const total = averageAmount(orders);

  try {
    await db.write({ total });
  } catch (e) {
    logger.error('order write failed', { correlationId: currentCorrelationId(), cause: e });
    throw e;
  }

  return total;
}

// procoder self-check: averageAmount handles the empty-array edge without NaN.
assert.strictEqual(averageAmount([]), 0);
assert.strictEqual(averageAmount([{ amount: 10 }, { amount: 20 }]), 15);
