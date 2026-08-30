<template>
  <div>
    <el-table ref="multipleTable" :data="tableData" row-key="ID" style="width: 100%" tooltip-effect="dark">
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
      <el-table-column align="left" label="状态" prop="status.phase">
        <template #default="scope">
          <div v-if="scope.row.status.phase === 'Completed'">
            <el-tooltip placement="top">
              <template #content>
                <div slot="content">
                  {{ scope.row.status.phase }}
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
      <el-table-column align="left" label="持续时间" prop="status.duration">
        <template #default="scope">
          <span>{{ scope.row.status.duration }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="存活时间" prop="metadata.creationTimestamp">
        <template #default="scope">
          <span>{{ AgeFormat(scope.row.metadata.creationTimestamp) }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="创建时间" prop="status.timeRange.start">
        <template #default="scope">
          <span v-if="scope.row.status.timeRange">{{ formatTimeToStr(scope.row.status.timeRange.start) }}</span>
        </template>
      </el-table-column>

      <el-table-column align="left" label="结束时间" prop="status.timeRange.end">
        <template #default="scope">
          <span v-if="scope.row.status.timeRange">{{ formatTimeToStr(scope.row.status.timeRange.end) }}</span>
        </template>
      </el-table-column>
      <el-table-column align="left" label="操作">
        <template #default="scope">
<!--          <el-button icon="edit" link size="small" type="primary" @click="handleRestore(scope.row)">恢复</el-button>-->
          <el-button icon="edit" link size="small" type="primary" @click="handleUpdate(scope.row)">编辑</el-button>
          <el-button icon="delete" link size="small" type="primary" @click="handleDelete(scope.row)">删除</el-button>
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
import { formatTimeToStr } from '@/plugin/kubernetes/utils/date'
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

// 恢复
const handleRestore = (value) => {
  console.log(value)
}
</script>
<style scoped>
   .operate-span-primary {
     color: var(--el-color-primary);
     cursor: pointer;
     font-weight:bold;
   }
</style>
