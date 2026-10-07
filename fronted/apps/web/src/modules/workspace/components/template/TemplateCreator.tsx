"use client";

import type { TemplateCreatorVM } from "@sparkops/contracts/product";

interface CreatorLike {
  creator?: { id: string; email?: string; display_name?: string };
  created_by?: string;
}

export function emailUsername(email?: string) {
  if (!email) return "";
  return email.includes("@") ? email.split("@")[0] : email;
}

export function resolveCreator(item: CreatorLike): TemplateCreatorVM | undefined {
  if (item.creator) return item.creator;
  if (!item.created_by) return undefined;
  return { id: item.created_by };
}

/** Member ID the backend uses when seeding built-in default templates. */
const SYSTEM_CREATOR_ID = "00000000-0000-4000-8000-000000000099";

export function isSystemCreated(item: CreatorLike) {
  return (item.creator?.id ?? item.created_by) === SYSTEM_CREATOR_ID;
}

export function creatorDisplayName(item: CreatorLike) {
  if (isSystemCreated(item)) return "系统内置";
  const creator = resolveCreator(item);
  if (!creator) return "未知用户";
  const nick = creator.display_name?.trim();
  if (nick) return nick;
  const username = emailUsername(creator.email);
  return username || "未知用户";
}

export function creatorInitial(item: CreatorLike) {
  const name = creatorDisplayName(item);
  return name.charAt(0).toUpperCase();
}

export function creatorHint(item: CreatorLike) {
  const creator = resolveCreator(item);
  if (!creator?.email) return creatorDisplayName(item);
  const nick = creator.display_name?.trim();
  if (nick) return `${nick} · ${creator.email}`;
  return creator.email;
}

interface TemplateCreatorProps {
  template: CreatorLike;
  compact?: boolean;
}

export function TemplateCreator({ template, compact = false }: TemplateCreatorProps) {
  const creator = resolveCreator(template);
  const name = creatorDisplayName(template);
  const hint = creatorHint(template);

  return (
    <div className={`template-creator ${compact ? "is-compact" : ""}`} title={hint}>
      <span className="template-avatar">{creatorInitial(template)}</span>
      <div className="template-creator-copy">
        <span className="template-creator-name">{name}</span>
        {!compact && creator?.email ? (
          <span className="template-creator-email">{emailUsername(creator.email)}</span>
        ) : null}
      </div>
    </div>
  );
}
