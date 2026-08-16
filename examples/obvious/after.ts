// examples/obvious/after.ts — three small functions instead of one, an
// options object instead of six positional parameters, and guard clauses.

interface User {
  name: string;
  role: string;
  active: boolean;
  verified: boolean;
  joined: string;
}

interface ReportOptions {
  includeInactive: boolean;
  sortKey: 'name' | 'joined';
  sortDir: 'asc' | 'desc';
  filterRole?: string;
  limit?: number;
}

function filterUsers(users: User[], options: ReportOptions): User[] {
  return users.filter((u) => {
    if (options.filterRole && u.role !== options.filterRole) return false;
    if (!options.includeInactive && !u.active) return false;
    return true;
  });
}

function statusOf(user: User): 'verified' | 'pending' | 'inactive' {
  if (!user.active) return 'inactive';
  // "active" alone is set on signup; verified needs its own explicit check
  return user.verified ? 'verified' : 'pending';
}

function sortUsers(users: User[], options: ReportOptions): User[] {
  const dir = options.sortDir === 'desc' ? -1 : 1;
  return [...users].sort((a, b) => dir * a[options.sortKey].localeCompare(b[options.sortKey]));
}

export function buildUserReport(users: User[], options: ReportOptions) {
  const activeUsers = filterUsers(users, options);
  const sorted = sortUsers(activeUsers, options);
  const withStatus = sorted.map((u) => ({ ...u, status: statusOf(u) }));
  return options.limit ? withStatus.slice(0, options.limit) : withStatus;
}
