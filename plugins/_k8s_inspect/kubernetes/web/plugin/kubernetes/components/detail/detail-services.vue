<template>
  <div>
    <h3>服务</h3>
    <el-alert
      v-if="ServicesData.length === 0"
      type="info"
    >
      <div slot="title">
        <i class="el-icon-info" />
        <span>此处显示与 {0} 同名的 Service: {1}，但是您并未定义此 Service。</span>
      </div>
    </el-alert>
    <el-table
      v-if="ServicesData.length !== 0"
      :data="ServicesData"
    >
      <el-table-column
        label="名称"
        prop="metadata.name"
      />
      <el-table-column
        label="协议"
        prop="protocol"
      >
        <template #default="scope">
          <div v-for="(item, index1)  in scope.row.spec.ports" :key="index1">
            <span>{{ item.protocol }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="端口"
        prop="port"
      >
        <template #default="scope">
          <div v-for="(item, index2)  in scope.row.spec.ports" :key="index2">
            <span>{{ item.port }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="NodePort"
        prop="nodePort"
      >
        <template #default="scope">
          <div v-for="(item, index3)  in scope.row.spec.ports" :key="index3">
            <span>{{ item.nodePort }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="容器端口"
        prop="targetPort"
      >
        <template #default="scope">
          <div v-for="(item, index4)  in scope.row.spec.ports" :key="index4">
            <span>{{ item.targetPort }}</span>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
export default {
  name: 'DetailServices',
}
</script>
<script setup>
import { ref } from 'vue'
import {
  ServicesList,
} from '@/plugin/kubernetes/api/services'
const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
  selectors: {
    default: function() {
      return ''
    },
    type: String
  }
})

// 数据初始化
const Detaildata = ref({})
const DetailSelectors = ref('')
const DetailDataInit = async() => {
  Detaildata.value = props.data
  DetailSelectors.value = props.selectors
}
DetailDataInit()

// 判断值在不在
const containsAllKeyValues = (selector, criteria) => {
  return Object.entries(criteria).every(([key, value]) =>
    Object.prototype.hasOwnProperty.call(selector, key) && selector[key] === value
  )
}

// Service列表
const ServicesData = ref([])
const ServiceDataGet = async() => {
  const [key, value] = DetailSelectors.value.split('=')
  const dictionary = { [key]: value }
  const res = await ServicesList(Detaildata.value.cluster_id, '', '', Detaildata.value.metadata.namespace, Detaildata.value.metadata.name, '')
  if (res.code === 0 && res.data.items) {
    for (const svc of res.data.items) {
      if (containsAllKeyValues(svc.spec.selector, dictionary)) {
        ServicesData.value.push(svc)
      }
    }
  }
}

ServiceDataGet()
</script>
<style scoped>

</style>
