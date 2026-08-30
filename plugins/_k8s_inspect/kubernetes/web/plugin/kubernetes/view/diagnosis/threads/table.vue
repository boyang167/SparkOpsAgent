<template>
  <div>
    <el-table ref="multipleTable" :key="tableData" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column align="left" label="ID" prop="id" />
      <el-table-column align="left" label="名称" prop="name" />
      <el-table-column align="left" label="Group" prop="group" />
      <el-table-column align="left" label="Priority" prop="priority" />
      <el-table-column align="left" label="State" prop="state" />
      <el-table-column align="left" label="CPU(%)" prop="cpu" />
      <el-table-column align="left" label="Time(秒)" prop="time" />
      <el-table-column align="left" label="Interrupted" prop="interrupted" />
      <el-table-column align="left" label="Daemon" prop="daemon" />
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
