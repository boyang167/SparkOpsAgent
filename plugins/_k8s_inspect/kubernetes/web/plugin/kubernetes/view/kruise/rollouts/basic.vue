<template>
  <div>
    <h3>基本信息</h3>
    <table style="width: 100%" class="myTable">
      <tbody>
        <tr>
          <td>名称</td>
          <td colspan="4">{{ basicData.metadata.name }}</td>
        </tr>
        <tr v-if="basicData.metadata.namespace">
          <td>命名空间</td>
          <td colspan="4">{{ basicData.metadata.namespace }}</td>
        </tr>
        <tr>
          <td>存活时间</td>
          <td colspan="4">{{ AgeFormat(basicData.metadata.creationTimestamp) }}</td>
        </tr>
        <tr>
          <td>标签</td>
          <td colspan="4">
            <detail-key-value v-if="basicData.metadata.labels" :form="basicData.metadata.labels" />
          </td>
        </tr>
        <tr>
          <td>注解</td>
          <td colspan="4">
            <detail-key-value v-if="basicData.metadata.annotations" :form="basicData.metadata.annotations" />
          </td>
        </tr>
      </tbody>
    </table>
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
import DetailKeyValue from '@/plugin/kubernetes/components/detail/detail-key-value.vue'
const props = defineProps({
  form: {
    default: function() {
      return {}
    },
    type: Object
  },
})

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
</script>
<style scoped>

</style>
