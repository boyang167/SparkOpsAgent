<template>
  <div>
    <el-row :gutter="20" class="row-box">
      <el-col :span="24">
        <el-card class="el-card">
          <DetailBasic :form="detailData" />
        </el-card>
      </el-col>
      <el-col :span="24">
        <el-tabs style="margin-top: 20px" tab-position="top" type="border-card">
          <el-tab-pane label="基本信息">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="名称">
                {{ detailData.metadata?.name || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="API 版本">
                {{ detailData.apiVersion || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="Kind">
                {{ detailData.kind || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="控制器">
                {{ detailData.spec?.controllerName || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="描述">
                {{ detailDescription }}
              </el-descriptions-item>
            </el-descriptions>
          </el-tab-pane>
          <el-tab-pane label="状态">
            <el-table v-if="detailConditions.length > 0" :data="detailConditions" border>
              <el-table-column label="类型" prop="type" width="180" />
              <el-table-column label="状态" prop="status" width="100">
                <template #default="scope">
                  <el-tag :type="scope.row.status === 'True' ? 'success' : 'danger'" size="small">{{ scope.row.status }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="原因" prop="reason" width="180" />
              <el-table-column label="消息" prop="message" />
              <el-table-column label="最近更新时间" min-width="180">
                <template #default="scope">
                  {{ scope.row.lastTransitionTime || '-' }}
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="暂无状态" />

            <div v-if="supportedFeatures.length > 0" style="margin-top: 12px;">
              <el-divider content-position="left">Supported Features</el-divider>
              <el-tag v-for="item in supportedFeatures" :key="item" size="small" style="margin-right: 6px; margin-bottom: 6px;">{{ item }}</el-tag>
            </div>
          </el-tab-pane>
        </el-tabs>
      </el-col>
    </el-row>
  </div>
</template>

<script>
export default {
  name: 'GatewayClassDetail',
}
</script>

<script setup>
import { computed, ref, watch } from 'vue'
import DetailBasic from '@/plugin/kubernetes/components/detail/detail-basic.vue'

const props = defineProps({
  data: {
    type: Object,
    default: () => ({})
  },
})

const detailData = ref({})

const detailDescription = computed(() =>
  detailData.value?.spec?.description ||
  detailData.value?.metadata?.annotations?.description ||
  '-'
)

const detailConditions = computed(() =>
  Array.isArray(detailData.value?.status?.conditions) ? detailData.value.status.conditions : []
)

const supportedFeatures = computed(() =>
  Array.isArray(detailData.value?.status?.supportedFeatures) ? detailData.value.status.supportedFeatures : []
)

const initDetailData = () => {
  detailData.value = props.data || {}
}

watch(
  () => props.data,
  () => {
    initDetailData()
  },
  { deep: true, immediate: true }
)
</script>

<style scoped>
</style>
