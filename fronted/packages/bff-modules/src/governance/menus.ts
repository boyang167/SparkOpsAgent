import type { BffContext } from "@sparkops/bff-core";
import type { MenuTreeVM, MenuVM, SetMenuRolesCommand, UpsertMenuCommand } from "@sparkops/contracts/governance";

export async function listMenuTree(ctx: BffContext, workspaceId: string): Promise<MenuTreeVM[]> {
  const data = await ctx.upstream.governance.listMenuTree(workspaceId);
  return data.items;
}

export async function listNavMenus(ctx: BffContext, workspaceId: string): Promise<MenuTreeVM[]> {
  const data = await ctx.upstream.governance.listNavMenus(workspaceId);
  return data.items;
}

export async function listMyMenuButtons(ctx: BffContext, workspaceId: string, path: string) {
  return ctx.upstream.governance.listMyMenuButtons(workspaceId, path);
}

export async function getMenu(ctx: BffContext, workspaceId: string, menuId: string): Promise<MenuVM> {
  return ctx.upstream.governance.getMenu(workspaceId, menuId);
}

export async function createMenu(ctx: BffContext, workspaceId: string, input: UpsertMenuCommand) {
  return ctx.upstream.governance.createMenu(workspaceId, input);
}

export async function updateMenu(
  ctx: BffContext,
  workspaceId: string,
  menuId: string,
  input: UpsertMenuCommand,
) {
  return ctx.upstream.governance.updateMenu(workspaceId, menuId, input);
}

export async function deleteMenu(ctx: BffContext, workspaceId: string, menuId: string) {
  return ctx.upstream.governance.deleteMenu(workspaceId, menuId);
}

export async function getMenuRoles(ctx: BffContext, workspaceId: string, menuId: string) {
  const data = await ctx.upstream.governance.getMenuRoles(workspaceId, menuId);
  return data.role_ids;
}

export async function setMenuRoles(
  ctx: BffContext,
  workspaceId: string,
  menuId: string,
  input: SetMenuRolesCommand,
) {
  return ctx.upstream.governance.setMenuRoles(workspaceId, menuId, input);
}

export async function getRoleMenus(ctx: BffContext, workspaceId: string, roleId: string) {
  const data = await ctx.upstream.governance.getRoleMenus(workspaceId, roleId);
  return data.menu_ids;
}

export async function setRoleMenus(
  ctx: BffContext,
  workspaceId: string,
  roleId: string,
  input: import("@sparkops/contracts/governance").SetRoleMenusCommand,
) {
  return ctx.upstream.governance.setRoleMenus(workspaceId, roleId, input);
}
