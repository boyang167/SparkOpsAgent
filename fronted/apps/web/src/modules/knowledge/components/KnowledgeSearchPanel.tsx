"use client";

import { useState } from "react";
import { knowledge } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { DocumentVM } from "@sparkops/contracts/knowledge";

interface KnowledgeSearchPanelProps {
  workspaceId: string;
  kbId: string;
}

export function KnowledgeSearchPanel({ workspaceId, kbId }: KnowledgeSearchPanelProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<DocumentVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);

  async function onSearch(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSearched(true);
    try {
      const data = await knowledge.searchDocuments(kbId, workspaceId, { query, limit: 10 });
      setResults(data.items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "检索失败");
      setResults([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="section stack-md">
      <div>
        <h2 className="section-title">检索测试</h2>
        <p className="text-muted mt-1">关键词匹配文档标题与内容（MVP 全文检索）</p>
      </div>
      <form className="flex flex-wrap gap-2" onSubmit={onSearch}>
        <input
          className="input flex-1 min-w-[200px]"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="输入检索关键词"
          required
        />
        <button className="btn" type="submit" disabled={loading}>
          {loading ? "检索中..." : "检索"}
        </button>
      </form>
      {error ? <p className="error">{error}</p> : null}
      {searched && results.length === 0 && !loading && !error ? (
        <div className="empty-state">未找到匹配文档</div>
      ) : null}
      {results.length > 0 ? (
        <div className="stack-md">
          {results.map((doc) => (
            <div key={doc.id} className="card">
              <p className="font-semibold">{doc.title}</p>
              <p className="text-muted mt-2 text-sm whitespace-pre-wrap line-clamp-6">{doc.content}</p>
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}
