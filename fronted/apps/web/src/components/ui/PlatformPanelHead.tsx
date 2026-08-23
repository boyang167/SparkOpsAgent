interface PlatformPanelHeadProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
}

/** 平台运营页内嵌标题（替代 PageHeader，避免重复层级） */
export function PlatformPanelHead({ title, description, action }: PlatformPanelHeadProps) {
  return (
    <div className="platform-orgs-head">
      <div className="platform-orgs-head-copy">
        <h1 className="platform-orgs-title">{title}</h1>
        {description ? <p className="platform-orgs-desc">{description}</p> : null}
      </div>
      {action}
    </div>
  );
}
