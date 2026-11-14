# 创建存储池时配置 Provider 成员

## 目标

补齐存储池创建闭环：管理员在“新增存储池”弹窗中直接配置一个或多个 Provider 成员，创建请求把成员作为 `members` 一次性提交。该功能不扩展创建后的成员编辑能力，也不修改后端 API。

## 交互设计

- 名称为必填项。
- 成员区域使用可增删的行，每行包含 Provider、角色和优先级。
- 打开弹窗时，若存在 ACTIVE Provider，默认加入第一个 Provider，角色为 `PRIMARY`，优先级为 `100`。
- 可继续添加成员；同一个 Provider 在一个池中只能出现一次。
- 角色沿用后端契约：`PRIMARY`、`REPLICA`、`ARCHIVE`、`DRAINING`。
- 创建池必须至少包含一个 `PRIMARY`；多个 `PRIMARY` 可以存在，但优先级不能相同。
- 无可用 Provider 时显示引导文案，并禁用创建，提示先注册 Provider。
- 校验错误显示在弹窗内；网络或服务端错误沿用页面错误提示。

## 数据流

前端维护 `name + members` 草稿。提交前由纯函数完成校验和空白标准化，随后调用现有 `createStoragePool`，形成：

```json
{
  "name": "primary-pool",
  "members": [
    {
      "provider_id": "<uuid>",
      "mode": "PRIMARY",
      "priority": 100
    }
  ]
}
```

创建成功后关闭弹窗、重置表单、重新加载 Provider 和存储池列表。列表继续按现有方式展示成员名称和角色。

## 边界与错误处理

- 仅允许选择状态为 `ACTIVE` 的 Provider。
- 拒绝空名称、空成员、重复 Provider、缺少 PRIMARY、PRIMARY 优先级冲突。
- 后端仍是最终校验者；前端错误提示只用于避免静默失败和无效请求。
- 不实现更新、删除或重排已有存储池成员。

## 验证

- 纯函数单元测试覆盖默认成员、命令构造及全部前端校验。
- 运行文件系统组件测试和前端类型检查。
- 手工验证弹窗可创建带成员的池，刷新后列表显示 Provider 名称与角色。
