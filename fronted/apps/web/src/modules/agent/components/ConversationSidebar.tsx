"use client";



import Link from "next/link";

import type { ConversationSummaryVM } from "@sparkops/contracts/runtime";



interface ConversationSidebarProps {

  projectId: string;

  agentId: string;

  conversations: ConversationSummaryVM[];

  currentConversationId?: string;

}



function formatTime(value: string) {

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {

    return value;

  }

  return date.toLocaleString("zh-CN", {

    month: "short",

    day: "numeric",

    hour: "2-digit",

    minute: "2-digit",

  });

}



export function ConversationSidebar({

  projectId,

  agentId,

  conversations,

  currentConversationId,

}: ConversationSidebarProps) {

  const baseHref = `/projects/${projectId}/agents/${agentId}`;



  return (

    <aside className="sidebar-panel w-full lg:w-72 shrink-0">

      <div className="px-4 py-3 border-b border-[var(--border)] flex items-center justify-between gap-2 bg-[var(--surface-muted)]">

        <h2 className="text-sm font-semibold">对话历史</h2>

        <Link href={baseHref} className="link-primary text-xs">

          新对话

        </Link>

      </div>

      <div className="max-h-[min(72vh,680px)] overflow-y-auto">

        {conversations.length === 0 ? (

          <p className="empty-state !border-0 !rounded-none !bg-transparent m-3">暂无历史对话</p>

        ) : (

          <ul>

            {conversations.map((item) => {

              const active = item.id === currentConversationId;

              const href = `${baseHref}?conversationId=${item.id}`;

              return (

                <li key={item.id}>

                  <Link

                    href={href}

                    className={`conv-item ${active ? "conv-item-active" : ""}`}

                  >

                    <p className="text-sm line-clamp-2">{item.preview || "新对话"}</p>

                    <p className="text-xs text-muted mt-1">{formatTime(item.updated_at)}</p>

                  </Link>

                </li>

              );

            })}

          </ul>

        )}

      </div>

    </aside>

  );

}

