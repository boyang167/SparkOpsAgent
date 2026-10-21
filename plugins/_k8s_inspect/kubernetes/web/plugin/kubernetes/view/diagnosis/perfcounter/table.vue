<template>
  <div>
    <el-table :data="tableData" style="width: 100%" tooltip-effect="dark">
      <el-table-column align="left" label="Name">
        <template v-slot="scope">
          {{ scope.row.name }}
        </template>
      </el-table-column>

      <el-table-column align="left" label="Value">
        <template v-slot="scope">
          <span>{{ scope.row.value }}</span>
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
