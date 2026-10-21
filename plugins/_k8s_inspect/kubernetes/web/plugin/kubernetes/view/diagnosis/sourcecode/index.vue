<template>
  <div>
    <div class="gva-table-box">
      <div style="overflow: hidden;width: 100%;">
        <el-form
            ref="searchForm"
            :inline="true"
            :rules="rules"
            :model="sourcecodeData"
        >
          <el-form-item
              label="类名"
              prop="className"
          >
            <el-input v-model="sourcecodeData.className" @keyup.enter="handleSearch"  autocomplete="off" />
          </el-form-item>

          <el-form-item
              label="方法名"
              prop="methodName"
          >
            <el-input v-model="sourcecodeData.methodName" @keyup.enter="handleSearch" autocomplete="off" />
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
          <div v-if="sourcecodeHtmlData" v-html="sourcecodeHtmlData" style="background-color: black; color: white; padding: 10px;"></div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script>

export default {
  name: 'sourcecode',
}
</script>

<script setup>
import { ref } from 'vue'
import {diagnosisProxy} from "@/plugin/kubernetes/api/diagnosis";


const searchInfo = ref({ 'keywords': '', 'namespace': '', 'containerName': '', 'podName': ''})
const sourcecodeData =  ref({"className": '', "methodName": ''})

const rules = ref({
  className: [{ required: true, message: '请输入类名', trigger: 'blur' }],
  methodName: [{ required: true, message: '请输入方法名', trigger: 'blur' }],
})

// 初始化数据
const sourcecodeHtmlData  = ref('')
const InitSearchData = async() => {
  sourcecodeHtmlData.value = ''
  if (searchInfo.value.containerName && searchInfo.value.podName) {
    searchInfo.value['body'] = {
      "action":"exec",
      "command": "jad " + sourcecodeData.value.className + " "  + sourcecodeData.value.methodName,
    }

    const responseData = await diagnosisProxy(searchInfo.value)
    if (responseData.code === 0) {
      if (responseData.data.arthas) {
        if (responseData.data.arthas.state === "SUCCEEDED") {
          // 找到匹配的结果对象
          const result = responseData.data.arthas.body.results.find(r => r.type === "jad");

          // 提取相关数据
          const classLoader = result.classInfo.classloader;
          const location = result.location;
          const source = result.source;

            // 格式化输出
          sourcecodeHtmlData.value =  `
              ClassLoader: </br>
              ${classLoader.map(loader => `+-${loader}`).join('<br>')}  </br></br>

              Location:   </br>
              ${location} </br></br>
              ${source
                            .replace(/\u003c/g, '<')
                            .replace(/\u003e/g, '>')
                            .replace(/\n/g, '<br>')
                            .replace(/ /g, '&nbsp;')
                        } </br></br>`;

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
