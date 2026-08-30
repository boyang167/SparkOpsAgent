<template>
  <div>
    <el-table ref="multipleTable" :key="tableData" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column align="left" label="类型" prop="type" />
      <el-table-column align="left" label="名称" prop="name" />
      <el-table-column align="left" label="Used" prop="used">
        <template #default="scope">
         <span v-if="scope.row.type !== 'buffer_pool'"> {{ MemorybytesToUnits(scope.row.used) }} M </span>
         <span v-else> OK </span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="Total" prop="total">
        <template #default="scope">
          <span v-if="scope.row.type !== 'buffer_pool'"> {{ MemorybytesToUnits(scope.row.total) }} M </span>
          <span v-else> OK </span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="Max" prop="max">
        <template #default="scope">
          <span v-if="scope.row.type !== 'buffer_pool'"> {{ MemorybytesToUnits(scope.row.max) }} M </span>
          <span v-else> - </span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="Usage" prop="Usage">
        <template #default="scope">
          <span>{{ ((scope.row.used / scope.row.total) * 100).toFixed(2) }} %</span>
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
import { MemorybytesToUnits } from '@/plugin/kubernetes/utils/convertMemory'
defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  },
  cluster_id: {
    default: function() {
      return 0
    },
    type: Number
  }
})
</script>
<style scoped>
</style>
