<template>
  <div class="container">
    <warning-bar title="注：请在Kubernetes集群内安装KubeBlocks控制器以便于使用该功能，建议KubeBlocks版本 >= 0.7 。" />
    <ClusterNamespace />
    <el-tabs style="width:100%;">
      <el-tab-pane label="集群管理">
        <!--子组件-->
        <Clusters />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>

import Clusters from './clusters/index.vue'
import WarningBar from '@/components/warningBar/warningBar.vue'
import ClusterNamespace from '@/plugin/kubernetes/components/select/index.vue'
export default {
  name: 'KubeBlocks',
  components: {
    Clusters,
    WarningBar,
    ClusterNamespace
  },
}
</script>
