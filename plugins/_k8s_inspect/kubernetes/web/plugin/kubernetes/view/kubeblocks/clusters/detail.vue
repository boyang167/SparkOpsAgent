<template>
  <div>
    <el-tabs v-model="activeName" class="demo-tabs">
      <el-tab-pane label="基本信息" name="basic">
        <el-card class="el-card">
          <detail-basic :form="detailData" />
        </el-card>
      </el-tab-pane>
      <el-tab-pane label="实例列表" name="podlist">
        <detail-pods
          v-if="Selectors !== ''"
          v-model="PodsVisible"
          :data="detailData"
          :selectors="Selectors"
        />
      </el-tab-pane>
      <el-tab-pane label="备份列表" name="backup">
        <detail-backup :cluster_id="cluster_id" :name="name" :namespace="namespace" :selector="Selectors"/>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
export default {
  name: 'Detail',
}
</script>
<script setup>
import { ref } from 'vue'
import DetailBasic from '@/plugin/kubernetes/components/kubeblocks/clusters/basic.vue'
import DetailBackup from '@/plugin/kubernetes/components/kubeblocks/backup/index.vue'
import DetailPods from '@/plugin/kubernetes/components/detail/detail-pods.vue'
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
const namespace = ref('')
const name = ref('')
const Selectors = ref('')
const PodsVisible = ref(true)
const cluster_id = ref(0)
const activeName = ref('basic')
const detailInitData = () => {
  detailData.value = props.data
  if (detailData.value.metadata) {
    namespace.value = detailData.value.metadata.namespace
    name.value = detailData.value.metadata.name
    cluster_id.value = detailData.value.cluster_id
    Selectors.value = 'app.kubernetes.io/instance=' + detailData.value.metadata.name + ',app.kubernetes.io/managed-by=kubeblocks'
  }
}

detailInitData()

</script>
<style scoped>

</style>
