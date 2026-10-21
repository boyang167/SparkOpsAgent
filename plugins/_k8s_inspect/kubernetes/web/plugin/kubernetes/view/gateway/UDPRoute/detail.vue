<template>
  <div>
    <el-row :gutter="20" class="row-box">
      <el-col :span="24">
        <el-card class="el-card">
          <detail-basic :form="detailData" />
        </el-card>
      </el-col>
      <el-col :span="24">
        <el-tabs style="margin-top: 20px" tab-position="top" type="border-card">
          <el-tab-pane label="Gateway 引用">
            <el-table v-if="detailData.spec?.parentRefs?.length > 0" :data="detailData.spec.parentRefs" border>
              <el-table-column label="Gateway 名称" prop="name" />
              <el-table-column label="Namespace" prop="namespace">
                <template #default="scope">{{ scope.row.namespace || '-' }}</template>
              </el-table-column>
              <el-table-column label="Section Name" prop="sectionName">
                <template #default="scope">{{ scope.row.sectionName || '-' }}</template>
              </el-table-column>
            </el-table>
            <el-empty v-else description="暂无引用" />
            <div v-if="detailData.spec?.parentRefs?.length > 0" style="margin-top: 12px;">
              <el-divider content-position="left">Gateway 详情</el-divider>
              <RelatedGatewayDetail :route-data="detailData" />
            </div>
          </el-tab-pane>
          <el-tab-pane label="状态">
            <div v-if="detailData.status?.parents?.length > 0">
              <div v-for="(parent, idx) in detailData.status.parents" :key="idx" style="margin-bottom: 16px;">
                <el-divider>Parent {{ idx + 1 }}</el-divider>
                <el-descriptions :column="2" border>
                  <el-descriptions-item label="Parent Ref">{{ parent.parentRef?.name }}</el-descriptions-item>
                  <el-descriptions-item label="Controller">{{ parent.controllerName }}</el-descriptions-item>
                </el-descriptions>
                <el-table v-if="parent.conditions?.length > 0" :data="parent.conditions" border style="margin-top: 8px;">
                  <el-table-column label="类型" prop="type" />
                  <el-table-column label="状态" prop="status">
                    <template #default="scope">
                      <el-tag :type="scope.row.status === 'True' ? 'success' : 'danger'" size="small">{{ scope.row.status }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="原因" prop="reason" />
                  <el-table-column label="消息" prop="message" />
                </el-table>
              </div>
            </div>
            <el-empty v-else description="暂无状态" />
          </el-tab-pane>
        </el-tabs>
      </el-col>
    </el-row>
  </div>
</template>

<script>
export default {
  name: 'UDPRouteDetail',
}
</script>
<script setup>
import DetailBasic from '@/plugin/kubernetes/components/detail/detail-basic.vue'
import RelatedGatewayDetail from '@/plugin/kubernetes/view/gateway/components/related-gateway-detail.vue'
import { ref } from 'vue'

const props = defineProps({
  data: {
    default: () => ({})
  }
})

const detailData = ref({})
const detailInitData = () => {
  detailData.value = props.data || {}
}
detailInitData()
</script>
<style scoped>
</style>
