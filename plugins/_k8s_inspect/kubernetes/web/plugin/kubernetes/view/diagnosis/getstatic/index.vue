<template>
  <div>
    <div class="gva-table-box">
      <div style="overflow: hidden;width: 100%;">
        <el-form
            ref="searchForm"
            :inline="true"
            :rules="rules"
            :model="getstaticData"
        >
          <el-form-item
              label="类名"
              prop="className"
          >
            <el-input v-model="getstaticData.className" @keyup.enter="handleSearch"  autocomplete="off" />
          </el-form-item>

          <el-form-item
              label="方法名"
              prop="methodName"
          >
            <el-input v-model="getstaticData.methodName" @keyup.enter="handleSearch" autocomplete="off" />
          </el-form-item>
          <el-form-item label-width="100px">
            <el-button size="small" type="primary" icon="refresh"   @click="handleSearch">
              查询
            </el-button>
          </el-form-item>
        </el-form>
      </div>
      <div style="overflow: hidden;width: 100%;">
        <el-card>
          <div v-if="getstaticHtmlData" v-html="getstaticHtmlData" style="background-color: black; color: white; padding: 10px;"></div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script>

export default {
  name: 'getstatic',
}
</script>

<script setup>
import { ref } from 'vue'
import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";
import { formatString } from "@/plugin/kubernetes/utils/formatString";


const searchInfo = ref({ 'keywords': '', 'namespace': '', 'containerName': '', 'podName': ''})
const getstaticData =  ref({"className": '', "methodName": ''})

const rules = ref({
  className: [{ required: true, message: '请输入类名', trigger: 'blur' }],
  methodName: [{ required: true, message: '请输入方法名', trigger: 'blur' }],
})

// 初始化数据
const getstaticHtmlData  = ref('')
const InitSearchData = async() => {
  getstaticHtmlData.value = ''
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action":"exec",
      "command": "getstatic " + getstaticData.value.className + " "  + getstaticData.value.methodName,
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          responseData.data.arthas.body.results.forEach((item, index) => {
            if (item.type === "getstatic") {
              getstaticHtmlData.value = "field: " + item.fieldName + "</br>"
              getstaticHtmlData.value += formatString(item.field)
            }
          })
        }
      }
    }
  }
}

// 查询
const handleSearch = async() => {
  await InitSearchData()
}

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
})
</script>
<style>
.button-container {
  display: flex;              /* 使用 Flexbox */
  justify-content: center;    /* 水平居中 */
  align-items: center;        /* 垂直居中（如果有需要的话） */
}
</style>
