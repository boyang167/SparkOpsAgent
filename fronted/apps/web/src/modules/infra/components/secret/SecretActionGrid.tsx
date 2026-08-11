import { DeleteIcon, EditIcon, TestIcon, ViewIcon } from "./SecretIcons";

interface SecretActionGridProps {
  testing?: boolean;
  onView: () => void;
  onEdit: () => void;
  onTest: () => void;
  onDelete: () => void;
}

export function SecretActionGrid({ testing, onView, onEdit, onTest, onDelete }: SecretActionGridProps) {
  return (
    <div className="secret-action-grid">
      <button type="button" className="secret-action-btn is-view" onClick={onView}>
        <ViewIcon />
        查看
      </button>
      <button type="button" className="secret-action-btn is-edit" onClick={onEdit}>
        <EditIcon />
        编辑
      </button>
      <button type="button" className="secret-action-btn is-test" disabled={testing} onClick={onTest}>
        <TestIcon />
        {testing ? "测试中" : "测试"}
      </button>
      <button type="button" className="secret-action-btn is-delete" onClick={onDelete}>
        <DeleteIcon />
        删除
      </button>
    </div>
  );
}
