export function hasSortableTables(root: HTMLElement): boolean;

export function enhanceSortableTables(
  root: HTMLElement,
  Tablesort: new (
    table: HTMLTableElement,
  ) => {
    sortTable(cell: HTMLTableCellElement): void;
  },
): void;
