<template>
  <div>
    <div>
      <el-row>
        <div style="margin-left: auto;">
          <el-button
            size="small"
            icon="refresh"
            @click="onReset"
          >刷新</el-button>
        </div>
      </el-row>
      <el-table
        ref="multipleTable"
        :data="PodsData"
        class="el-icon-extend"
        style="width: 100%"
        tooltip-effect="dark"
        @expand-change="handleExpand"
      >
        <el-table-column
          type="expand"
        >
          <template #default="scope">
            <div v-show="podView.containers">
              <el-table :data="podView.containers">
                <el-table-column
                  label="容器名称"
                  prop="name"
                />
                <el-table-column
                  label="镜像"
                  prop="image"
                />
                <el-table-column
                  label="重启次数"
                  prop="status.restartCount"
                />
                <el-table-column
                  align="left"
                  label="操作"
                >
                  <template #default="scope1">
                    <el-button
                      size="small"
                      type="primary"
                      link
                      @click="handleTerminal(scope.row, scope1.row)"
                    >终端</el-button>
                    <el-button
                      size="small"
                      type="primary"
                      link
                      @click="handleLogs(scope.row, scope1.row)"
                    >日志</el-button>
                  </template>
                </el-table-column>

              </el-table>
            </div>
            <el-card v-show="podView.events">
              <el-table :data="podView.events">
                <el-table-column
                  label="容器名称"
                  prop="name"
                >
                  <template #default="scope">
                    {{ scope.row.type }} / {{ scope.row.reason }}
                  </template>
                </el-table-column>
                <el-table-column
                  label="时间"
                  prop="firstTimestamp"
                >
                  <template #default="scope">
                    {{ AgeFormat(scope.row.firstTimestamp) }}
                  </template>
                </el-table-column>
                <el-table-column
                  label="消息"
                  prop="firstTimestamp"
                >
                  <template #default="scope">
                    {{ scope.row.message }}
                  </template>
                </el-table-column>
              </el-table>
            </el-card>
          </template>
        </el-table-column>
        <el-table-column
          label="集群"
          prop="cluster"
        >
          <template #default="scope">
            <span class="span-link">{{ scope.row.cluster }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="命名空间"
          prop="namespace"
        >
          <template #default="scope">
            <span class="span-link">{{ scope.row.metadata.namespace }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="名称"
          prop="name"
        >
          <template #default="scope">
            <span class="span-link">{{ scope.row.metadata.name }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          prop="status"
          type="scope"
          width="90"
        >
          <template #default="scope">
            <el-tag
              v-if="scope.row.status.phase ==='Running'"
              type="success"
            >Running</el-tag>
            <el-tag
              v-else
              type="danger"
            >{{ scope.row.status.phase }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="存活时间"
          prop="creationTimestamp"
        >
          <template #default="scope">
            {{ AgeFormat(scope.row.metadata.creationTime) }}
          </template>
        </el-table-column>
        <el-table-column
          label="节点"
          prop="nodeName"
        >
          <template #default="scope">
            <span class="span-link">{{ scope.row.status.nodeName }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <div v-if="dialogTerminalVisible">
      <el-dialog
        v-model="dialogTerminalVisible"
        width="60%"
        :title="title"
        :show-close="false"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
        :fullscreen="dialogFull"
        :center="true"
      >
        <template #default>
          <div style="text-align:right;">
            <el-button
              type="primary"
              link
              icon="FullScreen"
              @click="handledialogFull"
            />
            <el-button
              type="primary"
              link
              icon="Close"
              @click="closeDialog"
            />
          </div>
          <div class="dialog-footer">
            <PodsTerminalBlock
              :data="TerminalData"
              :name="ContainerName"
              @close="closeDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>
    <div v-if="dialogTerminalLogsVisible">
      <el-dialog
        v-model="dialogTerminalLogsVisible"
        width="60%"
        :title="title"
        :show-close="false"
        :close-on-press-escape="false"
        :close-on-click-modal="false"
        :fullscreen="dialogFull"
        :center="true"
      >
        <template #default>
          <div class="dialog-footer">
            <div style="text-align:right;">
              <el-button
                type="primary"
                link
                icon="FullScreen"
                @click="handledialogFull"
              />
              <el-button
                type="primary"
                link
                icon="Close"
                @click="closeDialog"
              />
            </div>
            <PodsTerminalLogBlock
              :data="TerminalData"
              :name="ContainerName"
              @close="closeDialog"
            />
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script>
export default {
  name: 'DetailPods',
}
</script>
<script setup>
import { ref } from 'vue'
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
import { ApplicationVelaqlList } from '@/plugin/kubernetes/api/kubevela/applications'
import PodsTerminalBlock from '@/plugin/kubernetes/components/pods/terminal.vue'
import PodsTerminalLogBlock from '@/plugin/kubernetes/components/pods/terminal-log.vue'

const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const velaql = ref('')
const PodsData = ref([])
const GetPods = async() => {
  velaql.value = 'component-pod-view{appNs=' + detailData.value.metadata.namespace + ', appName=' + detailData.value.metadata.name + '}.status'
  const res = await ApplicationVelaqlList(detailData.value.cluster_id, velaql.value)
  if (res.code === 0) {
    if (res.data.podList) {
      PodsData.value = res.data.podList
    }
  }
}

// 刷新Pod列表
const onReset = () => {
  GetPods()
}

const podView = ref({})
const GetPodView = async(name, namespace, cluster) => {
  velaql.value = 'pod-view{namespace=' + namespace + ', name=' + name + ', cluster=' + cluster + '}.status'
  const res = await ApplicationVelaqlList(detailData.value.cluster_id, velaql.value)
  if (res.code === 0) {
    podView.value = res.data
  }
}

// 数据初始化
const detailData = ref({})
const detailInitData = async() => {
  detailData.value = props.data
  await GetPods()
}

detailInitData()

// Pod详情
const handleExpand = async(row) => {
  await GetPodView(row.metadata.name, row.metadata.namespace, row.cluster)
}

// 终端
const dialogTerminalVisible = ref(false)
const handleTerminal = (row, row1) => {
  TerminalData.value = row
  title.value = '容器组名：' + row.metadata.name + ' 容器名：' + row1.name + ' 终端日志'
  TerminalData.value['cluster_id'] = detailData.value.cluster_id
  ContainerName.value = row1.name
  dialogTerminalVisible.value = true
}

// 终端日志
const title = ref('')
const dialogTerminalLogsVisible = ref(false)
const TerminalData = ref({})
const ContainerName = ref('')
const handleLogs = (row, row1) => {
  console.log(row, row1)
  TerminalData.value = row
  title.value = '容器组名：' + row.metadata.name + ' 容器名：' + row1.name + ' 终端日志'
  TerminalData.value['cluster_id'] = detailData.value.cluster_id
  ContainerName.value = row1.name
  dialogTerminalLogsVisible.value = true
}

// 全屏
const dialogFull = ref(false)
const handledialogFull = () => {
  if (dialogFull.value) {
    dialogFull.value = false
  } else {
    dialogFull.value = true
  }
}

// 关闭
const closeDialog = () => {
  dialogTerminalLogsVisible.value = false
  dialogTerminalVisible.value = false
  dialogFull.value = false
}
</script>
<style scoped>
</style>
