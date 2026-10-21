<template>
  <div>
    <el-table ref="multipleTable" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID" @sort-change="sortChange" @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="55" />
      <el-table-column align="left" label="ID" prop="id" sortable="custom" />
      <el-table-column align="left" label="名称" prop="name" sortable="custom" />
      <el-table-column align="left" label="Prometheus" prop="prometheus_url" />
      <el-table-column align="left" label="操作">
        <template #default="scope">
          <el-button size="small" type="primary" link icon="User" @click="handleRbacManage(scope.row)">授权</el-button>
          <el-button size="small" type="primary" link icon="Grid" @click="handleAssociat(scope.row)">关联</el-button>
          <el-button size="small" type="primary" link icon="edit" @click="handleUpdate(scope.row)">变更</el-button>
          <el-button size="small" type="primary" link icon="delete" @click="handleDelete(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
export default {
  name: 'TableBlock',
  emits: {
    update: () => {
      return true
    },
    search: () => {
      return true
    },
    delete: (value) => {
      if (value.length === 1) {
        return true
      } else {
        return false
      }
    },
  }
}
</script>
<script setup>
import { ref } from 'vue'
import { toSQLLine } from '@/utils/stringFun'
const emit = defineEmits(['update', 'delete', 'search', 'rbac', 'select', 'associat'])
defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  },
})

// 搜索排序
const searchInfo = ref({})
const sortChange = ({ prop, order }) => {
  if (prop) {
    if (prop === 'ID') {
      prop = 'id'
    }
    searchInfo.value.orderKey = toSQLLine(prop)
    searchInfo.value.desc = order === 'descending'
  }
  emit('search', searchInfo.value)
}

// 更新
const handleUpdate = (value) => {
  emit('update', value)
}

// 删除
const handleDelete = (value) => {
  emit('delete', value)
}

// 批量操作
const handleSelectionChange = (value) => {
  emit('select', value)
}

// 授权
const handleRbacManage = (value) => {
  emit('rbac', value)
}

// 关联集群
const handleAssociat = (value) => {
  emit('associat', value)
}
</script>
<style scoped>

</style>
