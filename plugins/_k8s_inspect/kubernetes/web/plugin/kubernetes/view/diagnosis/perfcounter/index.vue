<template>
  <div>
    <div class="gva-table-box">
      <div class="gva-btn-list" style="display: flex; justify-content: flex-end;">
        <el-form
            ref="searchForm"
            :inline="true"
            :model="searchInfo"
        >
          <el-form-item>
            <el-button size="small" type="primary" icon="refresh" @click="handleflush">
              更新
            </el-button>
          </el-form-item>
        </el-form>
      </div>
      <TableBlock :table-data="TableData" :cluster_id="searchInfo.cluster_id" />
    </div>
  </div>
</template>

<script>

export default {
  name: 'jvm',
}
</script>

<script setup>
import TableBlock from './table.vue'
import { shallowRef } from 'vue'
import {extractNameValuePairs} from '@/plugin/kubernetes/utils/mergeArrays'

import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";


const searchInfo = shallowRef({ 'keywords': '', 'namespace': '' })
const TableData = shallowRef([])


// 初始化数据
const InitTableData = async() => {
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action":"exec",
      "command":"perfcounter  --add-opens java.base/jdk.internal.perf=ALL-UNNAMED --add-exports java.base/jdk.internal.perf=ALL-UNNAMED --add-opens java.management/sun.management.counter.perf=ALL-UNNAMED --add-opens java.management/sun.management.counter=ALL-UNNAMED"
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          responseData.data.arthas.body.results.forEach((item, index) => {
            if (item.perfCounters) {
              TableData.value = extractNameValuePairs(item)
            }
          })
        }
      }
    }
  }
}


InitTableData()


const handleflush = async() => {
  await InitTableData()
}


setInterval(() => {
  InitTableData();
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
