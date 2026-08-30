<template>
  <div class="container">
    <warning-bar title="注：请在Kubernetes集群内安装velero控制器以便于使用该功能。(https://github.com/vmware-tanzu/velero)，注意: 操作且只能在velero所在命名空间执行。" />
    <ClusterNamespace />
    <el-tabs style="width:100%;">
      <el-tab-pane label="备份">
        <!--子组件-->
        <backupsBlock />
      </el-tab-pane>
      <el-tab-pane label="恢复">
        <!--子组件-->
        <DrestoreBlock />
      </el-tab-pane>
      <el-tab-pane label="计划">
        <!--子组件-->
        <SchedulesBlock />
      </el-tab-pane>
      <el-tab-pane label="下载">
        <!--子组件-->
        <DownloadRequestsBlock />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>
import WarningBar from '@/components/warningBar/warningBar.vue'
import backupsBlock from './backups/index.vue'
import DrestoreBlock from './restore/index.vue'
import SchedulesBlock from './schedules/index.vue'
import DownloadRequestsBlock from './downloadrequests/index.vue'
import ClusterNamespace from '@/plugin/kubernetes/components/select/index.vue'
export default {
  name: 'BackupCenter',
  components: {
    WarningBar,
    backupsBlock,
    DrestoreBlock,
    SchedulesBlock,
    DownloadRequestsBlock,
    ClusterNamespace,
  },
}
</script>
