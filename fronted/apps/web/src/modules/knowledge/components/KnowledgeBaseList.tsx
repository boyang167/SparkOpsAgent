import Link from "next/link";
import type { KnowledgeBaseVM } from "@sparkops/contracts/knowledge";

interface KnowledgeBaseListProps {
  knowledgeBases: KnowledgeBaseVM[];
}

export function KnowledgeBaseList({ knowledgeBases }: KnowledgeBaseListProps) {
  if (knowledgeBases.length === 0) {
    return <div className="empty-state">暂无知识库，点击「新建知识库」创建。</div>;
  }

  return (
    <div className="grid-cards">
      {knowledgeBases.map((kb) => (
        <Link key={kb.id} href={`/knowledge/${kb.id}`} className="card card-hover card-link">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="font-semibold truncate">{kb.name}</p>
              <p className="text-muted mt-1.5 line-clamp-2">{kb.description || "暂无描述"}</p>
            </div>
            <span className="badge shrink-0 badge-neutral">{kb.status}</span>
          </div>
        </Link>
      ))}
    </div>
  );
}
