<template>
  <div class="container">
    <el-tabs style="width:100%;">
      <el-tab-pane label="概况">
        <DetailBasicBlock :form="detailData" />
      </el-tab-pane>
      <el-tab-pane label="工作流">
        <DetailWorkflowBlock :data="detailData" />
      </el-tab-pane>
      <el-tab-pane label="实例">
        <DetailPodsBlock :data="detailData" />
      </el-tab-pane>
      <el-tab-pane label="拓扑图">
        <TreeViewBlock :data="detailData" />
      </el-tab-pane>
      <el-tab-pane label="版本">
        <DetailRevisionBlock :data="detailData" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>

export default {
  name: 'BaseTree',
}

</script>
<script setup>
import { ref } from 'vue'
import TreeViewBlock from '@/plugin/kubernetes/components/kubevela/treeview.vue'
import DetailBasicBlock from '@/plugin/kubernetes/components/kubevela/detail-basic.vue'
import DetailPodsBlock from '@/plugin/kubernetes/components/kubevela/detail-pods.vue'
import DetailRevisionBlock from '@/plugin/kubernetes/components/kubevela/detail-revisions.vue'
import DetailWorkflowBlock from '@/plugin/kubernetes/components/kubevela/detail-workflows.vue'

const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})
// 数据初始化
const detailData = ref({})
const detailInitData = async() => {
  detailData.value = props.data
}

detailInitData()

</script>
<style scoped>
</style>

