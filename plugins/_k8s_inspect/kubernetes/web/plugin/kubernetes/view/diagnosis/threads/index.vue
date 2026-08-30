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
            <span>总量:</span> <el-tag style="margin: 0 10px;" effect="dark" type="primary">{{ threadStateCount.SUM }}</el-tag>
            <span>状态分布</span>
            <el-badge :value="threadStateCount.NEW" class="item">
              <el-tag style="margin: 0 10px;" effect="dark" type="primary">NEW</el-tag>
            </el-badge>
            <el-badge :value="threadStateCount.RUNNABLE" class="item">
              <el-tag style="margin: 0 10px;" effect="dark" type="success">RUNNABLE</el-tag>
            </el-badge>
            <el-badge :value="threadStateCount.BLOCKED" class="item">
              <el-tag style="margin: 0 10px;" effect="dark" type="danger">BLOCKED</el-tag>
            </el-badge>
            <el-badge :value="threadStateCount.WAITING" class="item">
              <el-tag style="margin: 0 10px;" effect="dark" type="warning">WAITING</el-tag>
            </el-badge>
            <el-badge :value="threadStateCount.TIMED_WAITING" class="item">
              <el-tag style="margin: 0 10px;" effect="dark" type="warning">TIMED_WAITING</el-tag>
            </el-badge>
            <el-badge :value="threadStateCount.TERMINATED" class="item">
              <el-tag style="margin: 0 10px;" effect="dark" type="info">TERMINATED</el-tag>
            </el-badge>
          </el-form-item>
          <el-form-item>
            <el-button size="small" type="primary" icon="refresh" @click="handleflush">
              更新
            </el-button>
          </el-form-item>
        </el-form>
      </div>
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
</template>

<script>

export default {
  name: 'Threads',
}
</script>

<script setup>
import TableBlock from './table.vue'
import { ref,shallowRef } from 'vue'
import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";
const searchInfo = shallowRef({ 'keywords': '', 'namespace': '' })

// 查询
const threadTable = shallowRef([])
const threadshowAll = ref(false)
const threadAlldataTable = ref([])
const threadTop10dataTable = ref([])


// 初始化数据
const threadStateCount = ref({})
const InitTableData = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action":"exec",
      "command":"thread"
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          responseData.data.arthas.body.results.forEach((item, index) => {
            if (item.threadStateCount) {
              threadStateCount.value = item.threadStateCount
              threadStateCount.value['SUM'] = Object.values(item.threadStateCount).reduce((sum, value) => sum + value, 0);
            }

            if (item.threadStats) {
              threadAlldataTable.value = item.threadStats
              threadTop10dataTable.value = item.threadStats.slice(0,10)
              if (threadshowAll.value) {
                threadTable.value = threadAlldataTable.value
              } else {
                threadTable.value = threadTop10dataTable.value
              }
            }
          })
        }
      }
    }
  }

}

const handleflush = async() => {
  await InitTableData()
}

const handlethreadAll= () => {
  if (!threadshowAll.value) {
    threadTable.value = threadAlldataTable.value
    threadshowAll.value = true
  } else {
    threadTable.value = threadTop10dataTable.value
    threadshowAll.value = false
  }
}

InitTableData()

setInterval(() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    InitTableData();
  }
}, 10000);


// 事件监听
window.addEventListener('setItemCluster', (e) => {
  searchInfo.value.cluster_id = parseInt(localStorage.getItem('cluster_id'))
})

window.addEventListener('setItemNamespace', (e) => {
  searchInfo.value.namespace = localStorage.getItem('namespace')
})

window.addEventListener('setItempodName', (e) => {
  searchInfo.value.podName = localStorage.getItem('podName')
})

window.addEventListener('setItemcontainerName', (e) => {
  searchInfo.value.containerName = localStorage.getItem('containerName')
  InitTableData()
})
</script>
<style>
.button-container {
  display: flex;              /* 使用 Flexbox */
  justify-content: center;    /* 水平居中 */
  align-items: center;        /* 垂直居中（如果有需要的话） */
}
</style>
