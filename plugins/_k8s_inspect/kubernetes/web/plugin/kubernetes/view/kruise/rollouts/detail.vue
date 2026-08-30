<template>
  <div>
    <warning-bar title="注：发布工作流，发布完毕, 点击( 重置 ) 按钮, 把工作流恢复初始状态，否则下次发布没有工作流审批限制! " />
    <el-row
      :gutter="20"
      class="row-box"
    >
      <el-col :span="24">
        <el-card class="el-card">
          <detail-basic :form="detailData" />
          <el-steps :active="active">
            <el-step
              v-for="(step, index) in steps"
              :key="index"
              :title="step.title"
            >
              <template #description>
                {{ step.description }}
              </template>
            </el-step></el-steps>
          <el-button
            v-show="active !== stepsData.length"
            style="margin-top: 20px"
            size="small"
            icon="MagicStick"
            type="primary"
            @click="handleNext"
          >审批</el-button>
        </el-card>
      </el-col>
      <el-col :span="24">
        <el-card class="el-card">
          <el-row :gutter="24">
            <el-col :span="12">
              <el-button
                size="small"
                icon="refresh"
                @click="onReset"
              >刷新</el-button>
            </el-col>
          </el-row>
          <el-row :gutter="24">
            <el-col :span="12">
              <div>
                <detail-pods
                  v-if="Selectors !== ''"
                  v-model="PodsVisible"
                  :data="workloadData"
                  :selectors="Selectors"
                />
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script>
export default {
  name: 'Detail',
}
</script>
<script setup>
import { ref } from 'vue'
import DetailBasic from './basic.vue'
import DetailPods from '@/plugin/kubernetes/components/detail/detail-pods.vue'
import WarningBar from '@/components/warningBar/warningBar.vue'
import { DeploymentsGet } from '@/plugin/kubernetes/api/deployments'
import { DaemonsetsGet } from '@/plugin/kubernetes/api/daemonsets'
import { StatefulSetsGet } from '@/plugin/kubernetes/api/statefulsets'
import { CloneSetsGet } from '@/plugin/kubernetes/api/openkruise/clonesets'
import { RolloutsPatch } from '@/plugin/kubernetes/api/kruiseRollouts/rollouts'
const props = defineProps({
  data: {
    default: function() {
      return {}
    },
    type: Object
  },
})

// 生成steps 数据
const steps = ref([])
const stepsData = ref([])
const stepsDataInit = () => {
  const stepsNum = ref(1)
  for (let i = 0; i <= stepsData.value.length; i++) {
    if (stepsData.value[i] !== undefined) {
      steps.value.push({ title: '第 ' + stepsNum.value + ' 批次', description: '容器数量: ' + stepsData.value[i].replicas })
      stepsNum.value++
    }
  }
}

// 获取标签选择器
const SelectormatchLabel = async(data) => {
  if (data.spec.selector) {
    if (data.spec.selector.matchLabels) {
      for (const key in data.spec.selector.matchLabels) {
        if (Object.prototype.hasOwnProperty.call(data.spec.selector.matchLabels, key)) {
          Selectors.value += key + '=' + data.spec.selector.matchLabels[key] + ','
        }
      }
      Selectors.value = Selectors.value.length !== 0 ? Selectors.value.substring(0, Selectors.value.length - 1) : ''
    }
  }

  if (data.type === 'broadcastjob') {
    Selectors.value = 'broadcastjob-name' + '=' + data.metadata.name
  }
}

// 获取关联工作负载Pod列表
const workloadData = ref({})
const workloadDetail = async() => {
  let res
  switch (workloadRef.value.kind) {
    case 'Deployment':
      res = await DeploymentsGet(cluster_id.value, namespace.value, workloadRef.value.name)
      if (res.data.items) {
        if (res.data.items.code) {
          ElMessage({
            type: 'error',
            message: res.data.items.message,
          })
          return
        }
      }

      if (res.code === 0) {
        workloadData.value = res.data.items
        workloadData.value.cluster_id = cluster_id.value
        await SelectormatchLabel(res.data.items)
      }
      break
    case 'DaemonSet':
      res = await DaemonsetsGet(cluster_id.value, namespace.value, workloadRef.value.name)
      if (res.data.items) {
        if (res.data.items.code) {
          ElMessage({
            type: 'error',
            message: res.data.items.message,
          })
          return
        }
      }

      if (res.code === 0) {
        workloadData.value = res.data.items
        workloadData.value.cluster_id = cluster_id.value
        await SelectormatchLabel(res.data.items)
      }
      break
    case 'StatefulSet':
      res = await StatefulSetsGet(cluster_id.value, namespace.value, workloadRef.value.name)
      if (res.data.items) {
        if (res.data.items.code) {
          ElMessage({
            type: 'error',
            message: res.data.items.message,
          })
          return
        }
      }

      if (res.code === 0) {
        workloadData.value = res.data.items
        workloadData.value.cluster_id = cluster_id.value
        await SelectormatchLabel(res.data.items)
      }
      break
    case 'CloneSet':
      res = await CloneSetsGet(cluster_id.value, namespace.value, workloadRef.value.name)
      if (res.data.items) {
        if (res.data.items.code) {
          ElMessage({
            type: 'error',
            message: res.data.items.message,
          })
          return
        }
      }

      if (res.code === 0) {
        workloadData.value = res.data.items
        workloadData.value.cluster_id = cluster_id.value
        await SelectormatchLabel(res.data.items)
      }
      break
  }
}

// 刷新Pod列表
const onReset = () => {
  Selectors.value = ''
  workloadDetail()
}

// 数据初始化
const detailData = ref({})
const active = ref(0)
const namespace = ref('')
const workloadRef = ref({})
const Selectors = ref('')
const PodsVisible = ref(true)
const cluster_id = ref(0)
const detailInitData = () => {
  detailData.value = props.data
  if (detailData.value.spec) {
    if (detailData.value.spec.strategy) {
      if (detailData.value.spec.strategy.canary) {
        if (detailData.value.spec.strategy.canary.steps) {
          stepsData.value = detailData.value.spec.strategy.canary.steps
        }
      }
    }
  }

  if (detailData.value) {
    if (detailData.value.status) {
      if (detailData.value.status.canaryStatus) {
        active.value = detailData.value.status.canaryStatus.currentStepIndex
      }
    }
  }

  if (detailData.value.metadata) {
    namespace.value = detailData.value.metadata.namespace
  }

  if (detailData.value.spec) {
    if (detailData.value.spec.workloadRef) {
      workloadRef.value = detailData.value.spec.workloadRef
    }
  }

  cluster_id.value = detailData.value.cluster_id
  stepsDataInit()
  onReset()
}

detailInitData()

// 审批状态
const handleNext = async() => {
  for (const key in detailData.value.spec.strategy.canary.steps) {
    if (active.value === parseInt(key)) {
      detailData.value.spec.strategy.canary.steps[key - 1].pause = { 'duration': 0 }
    }
  }

  const res = await RolloutsPatch(cluster_id.value, detailData.value.metadata.namespace, detailData.value.metadata.name, { spec: { strategy: { canary: { steps: detailData.value.spec.strategy.canary.steps }}}})
  if (res.data.items) {
    if (res.data.items.code) {
      ElMessage({
        type: 'error',
        message: res.data.items.message,
      })
      return
    }
  }

  if (res.code === 0) {
    detailData.value = res.data.items
    detailData.value.cluster_id = cluster_id.value
    active.value++
  }

  onReset()
}
</script>
<style scoped>

</style>
