<template>
  <div class="container">
    <warning-bar title="注：请在Kubernetes集群内安装Kubevela控制器及安装Velaux插件以便于使用该功能，建议Kubevela版本 >= v1.9 。" />
    <ClusterNamespace />
    <el-tabs style="width:100%;">
      <el-tab-pane label="Application">
        <!--子组件-->
        <Applications />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
<script>
import WarningBar from '@/components/warningBar/warningBar.vue'
import ClusterNamespace from '@/plugin/kubernetes/components/select/index.vue'
import Applications from './applications/index.vue'
export default {
  name: 'Kubevela',
  components: {
    ClusterNamespace,
    WarningBar,
    Applications,
  },
}
</script>
