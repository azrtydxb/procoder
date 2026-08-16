// examples/safe/after.ts — role changes are validated and authorized
// server-side, and the query is parameterized.

interface Request { body: unknown; auth: { userId: string } }
interface Response { sendStatus(code: number): void }

declare const db: { query(sql: string, params: unknown[]): Promise<unknown> };
declare function currentUserRole(userId: string): Promise<'admin' | 'user'>;

function parseRoleChange(body: unknown): { targetId: string; role: 'admin' | 'user' } {
  const record = body as Record<string, unknown>;
  const targetId = String(record.userId ?? '');
  const role = record.role;
  if (!targetId || (role !== 'admin' && role !== 'user')) {
    throw new Error('invalid role-change payload');
  }
  return { targetId, role };
}

export async function updateUserRole(req: Request, res: Response): Promise<void> {
  const { targetId, role } = parseRoleChange(req.body);
  const actingRole = await currentUserRole(req.auth.userId);

  if (actingRole !== 'admin') {
    res.sendStatus(403);
    return;
  }

  await db.query('UPDATE users SET role = $1 WHERE id = $2', [role, targetId]);
  res.sendStatus(204);
}
