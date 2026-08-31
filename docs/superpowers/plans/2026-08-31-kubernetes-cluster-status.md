# Kubernetes Cluster Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 集群管理表显示当前页每个集群的 API 连通状态。

**Architecture:** 纯前端探测现有 Kubernetes 代理 `/version`；纯函数映射 checking/healthy/unhealthy。

**Tech Stack:** Next.js、现有 `k8sFetch`、vitest。

## Global Constraints

- 写操作权限不变；状态探测使用已有 GET 代理。
- 不新增后端路由。
- 删除仍只撤销工作区授权。
