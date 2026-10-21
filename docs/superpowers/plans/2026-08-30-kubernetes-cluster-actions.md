# Kubernetes Cluster Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 集群管理页提供授权、关联、变更、删除。

**Architecture:** kubeconfig 由插件 GET 从 ClusterClientFactory 重建；用户/角色走 apiserver Proxy；关联走 Velaux 代理；删除走工作区撤销授权。

**Tech Stack:** Gin 插件路由、Next.js、现有 `k8sFetch`。

## Global Constraints

- 写操作仅 governance:admin。
- 删除只撤销工作区授权。
- 不嵌入 Vue。
