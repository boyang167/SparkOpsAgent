<template>
  <div>
    <el-table ref="multipleTable" :key="tableData" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column align="left" label="ts" prop="ts" />
      <el-table-column align="left" label="accessPoint" prop="accessPoint" />
      <el-table-column align="left" label="className" prop="className" />
      <el-table-column align="left" label="methodName" prop="methodName" />
      <el-table-column align="cost" label="cost" prop="cost" />
      <el-table-column align="left" label="value" prop="value" />
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
