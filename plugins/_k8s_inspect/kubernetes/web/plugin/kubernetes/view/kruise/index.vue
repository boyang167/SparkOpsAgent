<template>
  <div class="container">
    <warning-bar title="注：请在Kubernetes集群内安装Kruise Rollouts控制器以便于使用该功能，建议Kruise Rollouts版本 >= 0.3.0 。" />
    <ClusterNamespace />
    <el-tabs style="width:100%;">
      <el-tab-pane label="Kruise Rollouts 渐进式交付">
        <!--子组件-->
        <Rollouts />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>

import Rollouts from './rollouts/index.vue'
import WarningBar from '@/components/warningBar/warningBar.vue'
import ClusterNamespace from '@/plugin/kubernetes/components/select/index.vue'
export default {
  name: 'OpenKruise',
  components: {
    Rollouts,
    WarningBar,
    ClusterNamespace
  },
}
</script>
