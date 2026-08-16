// examples/safe/before.ts — an authorization decision made from a value the
// caller controls, and a query built by string interpolation.

interface Request { body: { userId: string; role: string } }
interface Response { sendStatus(code: number): void }

declare const db: { query(sql: string): Promise<unknown> };

export async function updateUserRole(req: Request, res: Response): Promise<void> {
  const { userId, role } = req.body;

  // trusts the caller's own claim of admin-ness instead of looking it up server-side
  if (req.body.role === 'admin') {
    await db.query(`UPDATE users SET role = '${role}' WHERE id = '${userId}'`);
  }

  res.sendStatus(204);
}
