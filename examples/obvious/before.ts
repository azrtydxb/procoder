// examples/obvious/before.ts — one long function, six positional parameters,
// and a nested ternary, doing the work of several small pieces.

interface RawUser {
  name: string;
  role: string;
  active: boolean;
  verified: boolean;
  joined: string;
}

export function buildUserReport(
  users: RawUser[],
  includeInactive: boolean,
  sortKey: string,
  sortDir: string,
  filterRole: string,
  limit: number
) {
  const userArrayFiltered = [];
  for (const u of users) {
    if (filterRole && u.role !== filterRole) {
      continue;
    }
    if (!includeInactive && !u.active) {
      continue;
    }
    userArrayFiltered.push(u);
  }

  const withStatus = [];
  for (const u of userArrayFiltered) {
    const status = u.active ? (u.verified ? 'verified' : 'pending') : 'inactive';
    withStatus.push({ ...u, status });
  }

  let sorted = withStatus;
  if (sortKey === 'name') {
    sorted = withStatus.slice().sort((a, b) => {
      if (sortDir === 'desc') {
        return b.name.localeCompare(a.name);
      } else {
        return a.name.localeCompare(b.name);
      }
    });
  } else if (sortKey === 'joined') {
    sorted = withStatus.slice().sort((a, b) => {
      if (sortDir === 'desc') {
        return b.joined.localeCompare(a.joined);
      } else {
        return a.joined.localeCompare(b.joined);
      }
    });
  }

  let limited = sorted;
  if (limit && limit > 0) {
    limited = sorted.slice(0, limit);
  }

  const result = [];
  for (const u of limited) {
    result.push(u);
  }

  return result;
}
