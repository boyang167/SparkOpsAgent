# Kubernetes Member Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 工作区成员可被授权访问指定 Kubernetes 集群，并下载 24h ServiceAccount Token kubeconfig。

**Architecture:** SparkOps 持久化 grant；用集群 admin 客户端创建托管 SA 与 RBAC；Token 按需签发不入库。

**Tech Stack:** Gin、client-go、GORM、Next.js。

## Global Constraints

- 用户范围为当前工作区成员。
- 支持 ClusterRole 与命名空间 RoleBinding。
- Token 有效期 24 小时，删除授权时吊销访问。
- 写操作仅 governance:admin。
