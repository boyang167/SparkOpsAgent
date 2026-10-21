<template>
  <div>
    <el-card>
      <el-descriptions
        class="margin-top"
        title="基本信息"
        :column="2"
        border
      >
        <el-descriptions-item>
          <template #label>
            <div class="cell-item">
              名称
            </div>
          </template>
          {{ basicData.metadata.name }}
        </el-descriptions-item>
        <el-descriptions-item>
          <template #label>
            <div class="cell-item">
              命名空间
            </div>
          </template>
          {{ basicData.metadata.namespace }}
        </el-descriptions-item>
        <el-descriptions-item>
          <template #label>
            <div class="cell-item">
              存活时间
            </div>
          </template>
          {{ AgeFormat(basicData.metadata.creationTimestamp) }}
        </el-descriptions-item>
        <el-descriptions-item>
          <template #label>
            <div class="cell-item">
              标签
            </div>
          </template>
          <div>
            <detail-key-value
              v-if="basicData.metadata.labels"
              :form="basicData.metadata.labels"
            />
          </div>
        </el-descriptions-item>
        <el-descriptions-item>
          <template #label>
            <div class="cell-item">
              finalizers
            </div>
          </template>
          <div
            v-for="value in basicData.metadata.finalizers"
            :key="value"
            class="myTag"
          >
            <el-tag
              type="info"
              size="small"
            >
              {{ value.length > 100 ? value.substring(0, 100) + "..." : value }}
            </el-tag>
          </div>
        </el-descriptions-item>

        <el-descriptions-item>
          <template #label>
            <div class="cell-item">
              注解
            </div>
          </template>
          <div>
            <detail-key-value
              v-if="basicData.metadata.annotations"
              :form="basicData.metadata.annotations"
            />
          </div>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
    <el-card>
      <el-row v-if="statistics">
        <el-col :span="6">
          <el-statistic
            title="环境"
            :value="statistics.envCount"
          />
        </el-col>
        <el-col :span="6">
          <el-statistic
            title="交付目标"
            :value="statistics.targetCount"
          />
        </el-col>
        <el-col :span="6">
          <el-statistic
            title="版本"
            :value="statistics.revisionCount"
          />
        </el-col>
        <el-col :span="6">
          <el-statistic
            title="工作流"
            :value="statistics.workflowCount"
          />
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<script>
export default {
  name: 'DetailBasic',
}
</script>
<script setup>
import { ref } from 'vue'
import { AgeFormat } from '@/plugin/kubernetes/utils/age'
import DetailKeyValue from './detail-key-value.vue'
import { ApplicationStatisticsGet } from '@/plugin/kubernetes/api/kubevela/applications'
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
})

const statistics = ref()

const GetStatisticsList = async() => {
  const res = await ApplicationStatisticsGet(basicData.value.cluster_id, basicData.value.metadata.name)
  if (res.code === 0) {
    statistics.value = res.data
  } else {
    statistics.value = {
      'envCount': 0,
      'revisionCount': 0,
      'targetCount': 0,
      'workflowCount': 0
    }
  }
}

const containers = ref([])
const basicData = ref({})

const basicInitData = () => {
  basicData.value = props.form
  if (basicData.value.spec?.template?.spec?.containers) {
    containers.value = basicData.value.spec.template.spec.containers
    return true
  } else if (basicData.value.spec?.jobTemplate?.spec?.template?.spec?.containers) {
    containers.value = basicData.value.spec.jobTemplate.spec.template.spec.containers
    return true
  } else {
    return false
  }
}
basicInitData()
GetStatisticsList()
</script>
<style scoped>

</style>
