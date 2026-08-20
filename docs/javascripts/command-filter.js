/* An on-page filter for the command reference.
 *
 * The page is long by design — every skill plus every binary subcommand —
 * and the site's own search sends you to a different page rather than
 * narrowing this one. This narrows this one.
 *
 * It filters two shapes at once: rows of the skills table, and each
 * `#### procoder <cmd>` block in the reference below it. A section
 * heading whose children have all been filtered away hides too, so the
 * result never shows an empty "Everyday commands".
 *
 * Progressive: with JavaScript off the page is simply the full list.
 */
(() => {
  "use strict";

  const build = () => {
    const content = document.querySelector(".md-content__inner");
    if (!content) return;
    // only the command reference carries both of these
    const table = content.querySelector("table");
    const headings = content.querySelectorAll("h4");
    if (!table && !headings.length) return;
    if (content.querySelector(".procoder-filter")) return;

    // A group is a heading plus everything up to the next heading of the
    // same level or higher — the unit that gets shown or hidden as one.
    const groups = [];
    const nodes = Array.from(content.children);
    let current = null;
    for (const node of nodes) {
      const match = /^H([1-6])$/.exec(node.tagName);
      if (match) {
        if (Number(match[1]) >= 4) {
          current = { heading: node, nodes: [node], text: node.textContent };
          groups.push(current);
        } else {
          current = null; // h1–h3 are structure, never filtered away themselves
        }
        continue;
      }
      if (current) {
        current.nodes.push(node);
        current.text += ` ${node.textContent}`;
      }
    }
    for (const group of groups) group.text = group.text.toLowerCase();

    const rows = table ? Array.from(table.querySelectorAll("tbody tr")) : [];
    for (const row of rows)
      row.dataset.filterText = row.textContent.toLowerCase();

    // The h3 sections of the binary reference, so an emptied one hides.
    const sections = [];
    let section = null;
    for (const node of nodes) {
      if (node.tagName === "H3") {
        section = { heading: node, groups: [] };
        sections.push(section);
      } else if (section && node.tagName === "H4") {
        const group = groups.find((g) => g.heading === node);
        if (group) section.groups.push(group);
      }
    }

    const wrap = document.createElement("div");
    wrap.className = "procoder-filter";

    const label = document.createElement("label");
    label.setAttribute("for", "procoder-filter-input");
    label.textContent = "Filter commands";

    const input = document.createElement("input");
    input.type = "search";
    input.id = "procoder-filter-input";
    input.placeholder = "secrets, rename, sprint close…";
    input.autocomplete = "off";
    input.setAttribute("aria-describedby", "procoder-filter-count");

    const count = document.createElement("p");
    count.className = "procoder-filter__count";
    count.id = "procoder-filter-count";
    count.setAttribute("aria-live", "polite");

    wrap.append(label, input, count);

    const h1 = content.querySelector("h1");
    if (h1) {
      h1.after(wrap);
    } else {
      content.prepend(wrap);
    }

    const total = rows.length + groups.length;

    const apply = () => {
      const query = input.value.trim().toLowerCase();
      let shown = 0;

      for (const row of rows) {
        const hit = !query || row.dataset.filterText.includes(query);
        row.hidden = !hit;
        if (hit) shown++;
      }

      for (const group of groups) {
        const hit = !query || group.text.includes(query);
        for (const node of group.nodes) node.hidden = !hit;
        if (hit) shown++;
      }

      // a table filtered down to nothing is a stranded header row
      if (table) table.hidden = rows.length > 0 && rows.every((r) => r.hidden);

      for (const sec of sections) {
        const any = sec.groups.some((g) => !g.heading.hidden);
        sec.heading.hidden = sec.groups.length > 0 && !any;
      }

      if (!query) {
        count.textContent = "";
      } else if (shown === 0) {
        count.textContent = `No command matches “${input.value.trim()}”.`;
      } else {
        count.textContent = `${shown} of ${total} shown. Clear the box for all of them.`;
      }
    };

    input.addEventListener("input", apply);
    input.addEventListener("search", apply);
    input.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        input.value = "";
        apply();
      }
    });
  };

  // Material swaps page content without a reload, so rebuild per navigation.
  if (window.document$ && typeof window.document$.subscribe === "function") {
    window.document$.subscribe(build);
  } else {
    document.addEventListener("DOMContentLoaded", build);
  }
})();
