<template>
  <div>
    <el-table ref="multipleTable" :data="tableData" style="width: 100%" tooltip-effect="dark" row-key="ID">
      <el-table-column type="selection" width="55" />
      <el-table-column align="left" label="名称" prop="metadata.name">
        <template #default="scope">
          <span class="operate-span-primary" @click="handleDetail(scope.row)">{{ scope.row.metadata.name }}</span>
        </template>
      </el-table-column>
      <el-table-column
        label="命名空间"
        prop="namespace"
        type="scope"
      >
        <template #default="scope">
          <span>{{ scope.row.metadata.namespace }}</span>
        </template>
      </el-table-column>
      <el-table-column
        label="类型"
        prop="clusterDefinitionRef"
        type="scope"
      >
        <template #default="scope">
          <span>{{ scope.row.spec.clusterDefinitionRef }}</span>
        </template>
      </el-table-column>
      <el-table-column
        label="版本"
        prop="clusterVersionRef"
        type="scope"
      >
        <template #default="scope">
          <span>{{ scope.row.spec.clusterVersionRef }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="状态" prop="status.phase" width="180">
        <template #default="scope">
          <div v-if="scope.row.status.phase  === 'Running'">
            <el-tooltip placement="top">
              <template #content>
                <div slot="content">
                  {{ scope.row.status.phase }}（running, ready）
                </div>
              </template>
              <el-tag class="ml-2" effect="dark" type="success" />
            </el-tooltip>
          </div>
          <div v-else>
            <el-tooltip placement="top">
              <template #content>
                <div slot="content">
                  {{ scope.row.status.phase }}
                </div>
              </template>
              <el-tag class="ml-2" effect="dark" type="danger" />
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column align="left" label="存活时间" prop="metadata.creationTimestamp" width="180">
        <template #default="scope">
          <span>{{ AgeFormat(scope.row.metadata.creationTimestamp) }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="操作">
        <template #default="scope">
          <el-button size="small" type="primary" link icon="edit" @click="handleUpdate(scope.row)">编辑</el-button>
          <el-popover v-model="scope.row.visible" placement="top">
            <p>确定要删除吗？</p>
            <div style="text-align: right; margin-top: 8px;">
              <el-button size="small" type="primary" link @click="scope.row.visible = false">取消</el-button>
              <el-button type="primary" size="small" @click="handleDelete(scope.row)">确定</el-button>
            </div>
            <template #reference>
              <el-button type="primary" link icon="delete" size="small" @click="scope.row.visible = true">删除</el-button>
            </template>
          </el-popover>
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
    detail: () => {
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
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
const emit = defineEmits(['update', 'delete', 'search', 'detail'])
defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  },
})

// 更新
const handleUpdate = (value) => {
  emit('update', value)
}

// 删除
const handleDelete = (value) => {
  emit('delete', value)
}

// 详情
const handleDetail = (value) => {
  emit('detail', value)
}

</script>
<style scoped>
   .operate-span-primary {
     color: var(--el-color-primary);
     cursor: pointer;
     font-weight:bold;
   }
</style>
