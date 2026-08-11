"use client";

import { useState } from "react";
import type { SecretVM } from "@sparkops/contracts/secret";
import { serviceTypeLabel } from "@sparkops/contracts/secret";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Descriptions, DescriptionsItem } from "./Descriptions";
import { ConnectionIcon, TestIcon } from "./SecretIcons";
import {
  creatorDisplayName,
  formatDateTime,
  formatSensitive,
  labelOf,
  SECRET_STATUS_OPTIONS,
  serviceTypeBadgeClass,
  statusBadgeClass,
} from "./secret-utils";

interface SecretDetailProps {
  secret: SecretVM;
  testing: boolean;
  onTest: () => Promise<{ success: boolean; message: string } | void>;
}

export function SecretDetail({ secret, testing, onTest }: SecretDetailProps) {
  const [testMessage, setTestMessage] = useState<{ tone: "info" | "error"; text: string } | null>(null);

  async function handleTest() {
    setTestMessage(null);
    try {
      const result = await onTest();
      if (result) {
        setTestMessage({
          tone: result.success ? "info" : "error",
          text: result.message || (result.success ? "连接测试成功" : "连接测试失败"),
        });
      }
    } catch {
      setTestMessage({ tone: "error", text: "连接测试失败" });
    }
  }

  const isGitLab = secret.service_type === "GitLab";

  return (
    <div className="secret-detail">
      <Descriptions title="基本信息">
        <DescriptionsItem label="配置名称">
          <div className="secret-name-inline">
            <ConnectionIcon />
            <span>{secret.name}</span>
          </div>
        </DescriptionsItem>
        <DescriptionsItem label="显示名称">{secret.display_name || "-"}</DescriptionsItem>
        <DescriptionsItem label="服务类型">
          <span className={serviceTypeBadgeClass(secret.service_type)}>
            {serviceTypeLabel(secret.service_type)}
          </span>
        </DescriptionsItem>
        <DescriptionsItem label="服务地址">
          <a href={secret.service_url} target="_blank" rel="noreferrer" className="secret-link">
            {secret.service_url}
          </a>
        </DescriptionsItem>
        <DescriptionsItem label="项目">-</DescriptionsItem>
        <DescriptionsItem label="创建者">{creatorDisplayName(secret)}</DescriptionsItem>
        <DescriptionsItem label="状态">
          <span className={statusBadgeClass(secret.status)}>
            {labelOf(SECRET_STATUS_OPTIONS, secret.status)}
          </span>
        </DescriptionsItem>
        <DescriptionsItem label="默认配置">
          {secret.is_default ? <span className="secret-tag secret-tag-orange">默认</span> : "-"}
        </DescriptionsItem>
        <DescriptionsItem label="创建时间" span={2}>
          {formatDateTime(secret.created_at)}
        </DescriptionsItem>
        <DescriptionsItem label="更新时间" span={2}>
          {formatDateTime(secret.updated_at)}
        </DescriptionsItem>
        <DescriptionsItem label="描述" span={2}>
          {secret.description || "-"}
        </DescriptionsItem>
      </Descriptions>

      <Descriptions title="认证信息">
        {isGitLab ? (
          <DescriptionsItem label="访问令牌" span={2}>
            {secret.has_token ? formatSensitive(secret.credentials?.token) : "-"}
          </DescriptionsItem>
        ) : (
          <>
            <DescriptionsItem label="用户名">{secret.username || "-"}</DescriptionsItem>
            <DescriptionsItem label="密码">
              {secret.has_password ? formatSensitive(secret.credentials?.password) : "-"}
            </DescriptionsItem>
            <DescriptionsItem label="访问令牌">
              {secret.has_token ? formatSensitive(secret.credentials?.token) : "-"}
            </DescriptionsItem>
            <DescriptionsItem label="API密钥">
              {secret.has_api_key ? formatSensitive(secret.credentials?.api_key) : "-"}
            </DescriptionsItem>
          </>
        )}
      </Descriptions>

      {!isGitLab && secret.has_private_key ? (
        <Descriptions title="证书和密钥信息" column={1}>
          <DescriptionsItem label="私钥" span={2}>
            <textarea className="secret-readonly-textarea" readOnly value="******" rows={4} />
          </DescriptionsItem>
        </Descriptions>
      ) : null}

      {secret.has_extra_config ? (
        <Descriptions title="额外配置" column={1}>
          <DescriptionsItem label="配置内容" span={2}>
            <textarea className="secret-readonly-textarea" readOnly value="******" rows={4} />
          </DescriptionsItem>
        </Descriptions>
      ) : null}

      <div className="secret-test-panel">
        <button type="button" className="secret-test-btn" disabled={testing} onClick={() => void handleTest()}>
          <TestIcon />
          {testing ? "测试中…" : "测试连接"}
        </button>
        {testMessage ? (
          <InlineAlert tone={testMessage.tone === "info" ? "info" : "error"}>{testMessage.text}</InlineAlert>
        ) : null}
      </div>
    </div>
  );
}
