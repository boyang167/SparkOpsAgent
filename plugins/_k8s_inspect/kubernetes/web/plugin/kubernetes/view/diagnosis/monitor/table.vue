<template>
  <div>
    <el-table ref="multipleTable" :key="tableData" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column align="left" label="timestamp" prop="timestamp" />
      <el-table-column align="left" label="className" prop="className" />
      <el-table-column align="left" label="methodName" prop="methodName" />
      <el-table-column align="left" label="total" prop="total" />
      <el-table-column align="left" label="success" prop="success" />
      <el-table-column align="left" label="failed" prop="failed" />
      <el-table-column align="left" label="avg-rt(ms)" prop="cost">
        <template v-slot="scope">
          <span v-if="scope.row.cost"> {{ scope.row.cost.toFixed(2) }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="fail-rate" prop="fail-rate">
        <template v-slot="scope">
            <span v-if="scope.row.total"> {{ ((scope.row.failed / scope.row.total) * 100).toFixed(2) }}%</span>
           <span v-else>-</span>
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
defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  }
})

</script>
<style scoped>
</style>
