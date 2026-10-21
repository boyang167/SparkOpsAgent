<template>
  <div>
    <div>
      <h3>运行时</h3>
    </div>
    <el-table :data="tableData" style="width: 100%" tooltip-effect="dark">
      <el-table-column align="left" label="Name">
        <template v-slot="scope">
          {{ scope.row.name }}
        </template>
      </el-table-column>

      <el-table-column align="left" label="Value">
        <template v-slot="scope">
          <span v-if="scope.row.name === 'uptime'">{{ scope.row.value  }} 秒</span>
          <span v-else-if="scope.row.name === 'timestamp'">{{ formatTimeToStr(scope.row.value) }}</span>
          <span v-else-if="scope.row.name === 'systemLoadAverage'">{{ scope.row.value.toFixed(2) }}</span>
          <span v-else>{{ scope.row.value }}</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
export default {
  name: 'TableBlock',
}
</script>
<script setup>
import { ref } from 'vue'
import { formatTimeToStr } from '@/plugin/kubernetes/utils/date'

const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})


const tableData = ref([])
const InitTableData = async() => {
  if (props.data) {
    tableData.value = Object.entries(props.data).map(([name, value]) => ({ name, value }))
  }
}

InitTableData()

setInterval(() => InitTableData(), 1000);
</script>
<style scoped>
</style>
