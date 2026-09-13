interface K8sPaginationProps {
  page: number;
  pageSize: number;
  total: number;
  pageSizes?: number[];
  loading?: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}

export function K8sPagination({
  page,
  pageSize,
  total,
  pageSizes = [10, 30, 50, 100],
  loading = false,
  onPageChange,
  onPageSizeChange,
}: K8sPaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="k8s-pagination-bar">
      <span className="k8s-pagination-total">共 {total} 条</span>
      <div className="k8s-pagination-controls">
        <select
          className="input w-auto"
          value={pageSize}
          onChange={(e) => onPageSizeChange(Number(e.target.value))}
          aria-label="每页条数"
        >
          {pageSizes.map((size) => (
            <option key={size} value={size}>
              {size} 条/页
            </option>
          ))}
        </select>
        <button
          type="button"
          className="btn-secondary text-sm"
          disabled={page <= 1 || loading}
          onClick={() => onPageChange(Math.max(1, page - 1))}
        >
          上一页
        </button>
        <span className="k8s-page-indicator">
          <span className="k8s-page-current">{page}</span>
          <span className="k8s-page-sep">/</span>
          <span>{totalPages}</span>
        </span>
        <button
          type="button"
          className="btn-secondary text-sm"
          disabled={page >= totalPages || loading}
          onClick={() => onPageChange(Math.min(totalPages, page + 1))}
        >
          下一页
        </button>
      </div>
    </div>
  );
}
