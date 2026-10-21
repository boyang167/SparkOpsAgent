<template>
  <div>
    <div class="gva-table-box">
      <div class="gva-btn-list">
        <el-form
          ref="searchForm"
          :inline="true"
          :model="searchInfo"
        >
          <el-form-item>
            <h3>线程 TOP-10</h3>
          </el-form-item>
        </el-form>
      </div>
      <div>
        <TableBlock :table-data="threadTable" :cluster_id="searchInfo.cluster_id" />
        <div class="button-container">
          <el-button v-if="!threadshowAll || threadTable.length < 0" type="primary" link @click="handlethreadAll">
            查看更多
          </el-button>
          <el-button v-if="threadshowAll" type="primary" link @click="handlethreadAll">
            收起
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>

export default {
  name: 'Threads',
}
</script>

<script setup>
import TableBlock from './table.vue'
import { ref,shallowRef } from 'vue'
const threadTable = shallowRef([])
const searchInfo = shallowRef({ 'keywords': '', 'namespace': '' })

const props = defineProps({
  tableData: {
    default: function() {
      return []
    },
    type: Array
  },
})

// 查询
const threadshowAll = ref(false)
const threadAlldataTable = ref([])
const threadTop10dataTable = ref([])
const InitTableData = async() => {
  if (props.tableData.length > 0) {
    threadAlldataTable.value = props.tableData
    threadTop10dataTable.value = props.tableData.slice(0,10)
      if (threadshowAll.value) {
        threadTable.value = threadAlldataTable.value
      } else {
        threadTable.value = threadTop10dataTable.value
      }
  }
}

InitTableData()

const handlethreadAll= () => {
  if (!threadshowAll.value) {
    threadTable.value = threadAlldataTable.value
    threadshowAll.value = true
  } else {
    threadTable.value = threadTop10dataTable.value
    threadshowAll.value = false
  }
}

setInterval(() => InitTableData(), 5000);
</script>
<style>
.button-container {
  display: flex;              /* 使用 Flexbox */
  justify-content: center;    /* 水平居中 */
  align-items: center;        /* 垂直居中（如果有需要的话） */
}
</style>
