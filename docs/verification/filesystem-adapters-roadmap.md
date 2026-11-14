# Unified File Infrastructure — 云厂商与 OpenSearch 扩展路线

> v1.0 首发范围：**LOCAL / NFS / S3 Compatible（含 MinIO）**。本文描述后续 Adapter 与搜索扩展，**不构成 v1.0 发布 PASS 项**。

## 对象存储 Adapter

| 厂商 | 目标 Adapter | 状态 |
|---|---|---|
| 腾讯云 COS | `ProviderTypeS3` + 签名/地域元数据 | 规划中 |
| 阿里云 OSS | 同上（S3 兼容或原生 SDK） | 规划中 |
| Azure Blob | 独立 `ProviderTypeAzure` 或 S3 网关 | 规划中 |

落地顺序建议：先统一 **Provider 注册/密钥引用（Vault/K8s Secret）** 与 **契约测试（`contracttest.Run`）**，再按客户需求逐个 Adapter 开关上线。

## 搜索（OpenSearch / Elasticsearch）

- 当前：关系库侧 `RepositorySearchIndex` + 元数据检索。
- 扩展：异步索引 Worker（Revision/Metadata 事件）→ OpenSearch 集群；需定义 **PITR 与索引重建 Runbook**（与 `docs/verification/unified-file-infrastructure-disaster-recovery.md` 对齐）。

## 发布门禁

上述项在具备 **目标环境 E2E 证据** 前，不得在 `unified-file-infrastructure-v1.0.md` 中标记为 PASS。
