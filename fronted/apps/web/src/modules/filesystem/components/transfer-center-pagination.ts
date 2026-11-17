// Slices local and persisted transfer rows for table pagination.

import type { TransferSessionVM } from "@sparkops/contracts/filesystem";
import type { TransferItem } from "./transfer-queue";

export const TRANSFER_TABLE_PAGE_SIZE = 10;

export const TRANSFER_TABLE_PAGE_SIZE_OPTIONS = [10, 20, 50] as const;

export type TransferTablePage = {
  pageIndex: number;
  totalPages: number;
  totalCount: number;
  localItems: TransferItem[];
  persistedItems: TransferSessionVM[];
};

export function sliceTransferTablePage(
  localItems: readonly TransferItem[],
  persistedItems: readonly TransferSessionVM[],
  pageIndex: number,
  pageSize = TRANSFER_TABLE_PAGE_SIZE,
): TransferTablePage {
  const totalCount = localItems.length + persistedItems.length;
  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));
  const safeIndex = Math.min(Math.max(0, pageIndex), totalPages - 1);
  const start = safeIndex * pageSize;
  const end = start + pageSize;
  const localEnd = Math.min(end, localItems.length);
  const pageLocal =
    start < localItems.length ? localItems.slice(start, localEnd) : [];
  const persistedStart = Math.max(0, start - localItems.length);
  const persistedEnd = Math.max(0, end - localItems.length);
  const pagePersisted = persistedItems.slice(persistedStart, persistedEnd);
  return {
    pageIndex: safeIndex,
    totalPages,
    totalCount,
    localItems: pageLocal,
    persistedItems: pagePersisted,
  };
}