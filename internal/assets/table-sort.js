const sortableTableSelector =
  'table:not([class]):not([data-m2h-sortable="true"])';
const interactiveHeaderSelector =
  "a, button, input, select, textarea, [contenteditable=true]";

export function hasSortableTables(root) {
  return root.querySelector(sortableTableSelector) !== null;
}

export function enhanceSortableTables(root, Tablesort) {
  for (const table of root.querySelectorAll(sortableTableSelector)) {
    const head = table.tHead;
    if (!head?.rows.length || (table.tBodies[0]?.rows.length ?? 0) < 2) {
      continue;
    }
    // Match the header row that Tablesort uses for column indexes.
    const row =
      Array.from(head.rows).find((row) => row.dataset.sortMethod === "thead") ??
      head.rows[head.rows.length - 1];
    const headers = Array.from(row.cells).filter(
      (cell) =>
        cell.dataset.sortMethod !== "none" &&
        !cell.querySelector(interactiveHeaderSelector),
    );
    const methods = Array.from(row.cells, (cell) => cell.dataset.sortMethod);

    // Disable Tablesort's cell click handlers during setup. Restore custom
    // comparison methods before sorting with the button.
    for (const cell of row.cells) cell.dataset.sortMethod = "none";
    let sorter;
    try {
      sorter = new Tablesort(table);
    } catch {
      continue;
    } finally {
      Array.from(row.cells).forEach((cell, index) => {
        const method = methods[index];
        if (method === undefined) delete cell.dataset.sortMethod;
        else cell.dataset.sortMethod = method;
      });
    }
    table.dataset.m2hSortable = "true";

    let current;
    let defaultHeader;
    const controls = headers.map((cell) => {
      const label = cell.textContent.trim();
      const button = table.ownerDocument.createElement("button");
      button.type = "button";
      button.className = "m2h-sort-button";
      button.dataset.m2hDecor = "table-sort";
      if (cell.hasAttribute("data-sort-default")) defaultHeader = cell;
      cell.setAttribute("aria-sort", "none");
      cell.append(button);
      button.addEventListener("click", () => sortBy(cell));
      return { cell, label, button };
    });

    function syncButtons() {
      for (const { cell, label, button } of controls) {
        const order = cell.getAttribute("aria-sort");
        const title =
          order === "ascending"
            ? "当前升序，切换为降序"
            : order === "descending"
              ? "当前降序，切换为升序"
              : "升序排序";
        button.title = title;
        button.setAttribute("aria-label", label ? `${label}：${title}` : title);
      }
    }

    function sortBy(cell) {
      if (current && current !== cell)
        current.setAttribute("aria-sort", "none");
      current = cell;
      sorter.sortTable(cell);
      syncButtons();
    }

    if (defaultHeader) sortBy(defaultHeader);
    else syncButtons();
  }
}
