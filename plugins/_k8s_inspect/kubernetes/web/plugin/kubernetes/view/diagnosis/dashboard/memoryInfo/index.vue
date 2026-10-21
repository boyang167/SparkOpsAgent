<template>
  <div>
    <div class="gva-table-box">
      <div>
        <h3>JVM 内存</h3>
      </div>
      <div v-if="tableData.length > 0">
        <TableBlock :table-data="tableData" :cluster_id="searchInfo.cluster_id" />
        <div class="button-container">
          <el-button v-if="!jvmshowAll" type="primary" link @click="handleJvmShowAll">
            查看更多
          </el-button>
          <el-button v-if="jvmshowAll" type="primary" link @click="handleJvmShowAll">
            收起
          </el-button>
        </div>
      </div>
      <div>
        <GeneralChartBlock :show="jvmshowAll" :data="memoryInfoHeap" :title="heapTitle" />
        <GeneralChartBlock :show="jvmshowAll" :data="memoryInfoPsEdenSpace" :title="psEdenSpaceTitle" />
        <GeneralChartBlock :show="jvmshowAll" :data="memoryInfoPsSurvivorSpace" :title="psSurvivorSpaceTitle" />
        <GeneralChartBlock :show="jvmshowAll" :data="memoryInfoPsOldGen" :title="psOldGenTitle" />
        <SpecialChartBlock :show="jvmshowAll" :data="memoryInfoNonHeap" :title="nonHeapTitle" />
        <GeneralChartBlock :show="jvmshowAll" :data="memoryInfoCodeCache" :title="codeCacheTitle"/>
        <SpecialChartBlock :show="jvmshowAll" :data="memoryInfoMetaspace" :title="metaSpaceTitle" />
        <GeneralChartBlock :show="jvmshowAll" :data="memoryInfoCompressedClassSpace" :title="compressedClassSpaceTitle"/>
        <SpecialChartBlock :show="jvmshowAll" :data="memoryInfoDirect" :title="directTitle"/>
        <SpecialChartBlock :show="jvmshowAll" :data="memoryInfoMapped" :title="mappedTitle"/>
      </div>
    </div>
  </div>
</template>

<script setup>
import TableBlock from './table.vue';
import GeneralChartBlock from './general/index.vue'
import SpecialChartBlock from './special/index.vue'
import { ref } from 'vue';


const searchInfo = ref({ keywords: '', namespace: '' });
const jvmshowAll = ref(false);

const props = defineProps({
  tableData: {
    default: () => [],
    type: Array,
  },
});


const memoryInfo = ref([])
const memoryInfoHeap = ref({})
const memoryInfoPsEdenSpace = ref({})
const memoryInfoPsSurvivorSpace = ref({})
const memoryInfoPsOldGen = ref({})
const memoryInfoNonHeap = ref({})
const memoryInfoCodeCache = ref({})
const memoryInfoMetaspace = ref({})
const memoryInfoCompressedClassSpace = ref({})
const memoryInfoDirect = ref({})
const memoryInfoMapped = ref({})
const heapTitle = ref('heap')
const psEdenSpaceTitle = ref('ps_eden_space')
const psSurvivorSpaceTitle = ref('ps_survivor_space')
const psOldGenTitle = ref('ps_old_gen')
const compressedClassSpaceTitle = ref('compressed_class_space')
const codeCacheTitle = ref('code_cache')
const nonHeapTitle = ref('nonheap')
const metaSpaceTitle = ref('metaspace')
const directTitle = ref('direct')
const mappedTitle = ref('mapped')
const initMemoryInfoData = () => {
  memoryInfo.value = props.tableData
  memoryInfo.value.forEach(item => {
    if (item) {

      if (item.name  === 'heap') {
        memoryInfoHeap.value = item
      }

      if (item.name  === 'ps_eden_space') {
        memoryInfoPsEdenSpace.value = item
      }

      if (item.name  === 'ps_survivor_space') {
        memoryInfoPsSurvivorSpace.value = item
      }

      if (item.name  === 'ps_old_gen') {
        memoryInfoPsOldGen.value = item
      }

      if (item.name  === 'nonheap') {
        memoryInfoNonHeap.value = item
      }

      if (item.name  === 'code_cache') {
        memoryInfoCodeCache.value = item
      }

      if (item.name  === 'metaspace') {
        memoryInfoMetaspace.value = item
      }

      if (item.name  === 'compressed_class_space') {
        memoryInfoCompressedClassSpace.value = item
      }

      if (item.name  === 'direct') {
        memoryInfoMapped.value = item
      }

      if (item.name  === 'mapped') {
        memoryInfoDirect.value = item
      }
    }
  });

};

const handleJvmShowAll = () => {
  if (!jvmshowAll.value) {
    jvmshowAll.value = true
  } else {
    jvmshowAll.value = false
  }
}

setInterval(initMemoryInfoData, 3000);

</script>

<style scoped>
.chart-container {
  width: 100%; /* 图表宽度自适应 */
  height: 400px; /* 图表高度 */
}
</style>
