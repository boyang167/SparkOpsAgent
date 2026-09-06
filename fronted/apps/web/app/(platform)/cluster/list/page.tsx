import { redirect } from "next/navigation";

/** 集群管理已迁移至项目内「运行环境」，旧路径重定向到项目列表。 */
export default function ClusterListPage() {
  redirect("/projects");
}
