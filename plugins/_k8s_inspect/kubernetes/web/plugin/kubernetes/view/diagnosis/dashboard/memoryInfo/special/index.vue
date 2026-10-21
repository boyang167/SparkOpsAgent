<template>
  <div v-show="show" ref="memoryInfochart" className="chart-container"/>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from "vue";
import { getCurrentDateTime } from "@/plugin/kubernetes/utils/date";
import { MemorybytesToUnits } from "@/plugin/kubernetes/utils/convertMemory";
import * as echarts from "echarts";

const props = defineProps({
  data: {
    default: () => { },
    type: Object,
  },
  show: {
    default: () => false,
    type: Boolean,
  },
  title: {
    default: () => '',
    type: String,
  }
});

const memoryInfochart = ref(null);
let chartInstance = null;

const memoryInfoUsed = ref([]);
const memoryInfoTotal = ref([]);
const memoryInfoPercentage = ref([]);  // 用于存储百分比数据
const timeData = ref([]);
const memoryInfoData = ref({});

const initMemoryInfomemoryInfoData = () => {
  memoryInfoData.value = props.data;
  if (memoryInfoData.value) {
    timeData.value.push(getCurrentDateTime());
    const used = MemorybytesToUnits(memoryInfoData.value.used);
    const total = MemorybytesToUnits(memoryInfoData.value.total);
    memoryInfoUsed.value.push(used);
    memoryInfoTotal.value.push(total);

    // 计算使用率的百分比
    let percentage;

    if ((used / total) * 100 > 0) {
      percentage = ((used / total) * 100).toFixed(2);
    } else {
      percentage = "-";
    }

    memoryInfoPercentage.value.push(percentage);
  }

  // 如果 chartInstance 已经存在，只更新数据，不重新初始化
  if (chartInstance) {
    updateChart();
  } else {
    initmemoryInfoChart();
  }
};

// 初始化图表
const initmemoryInfoChart = () => {
  if (memoryInfochart.value) {
    chartInstance = echarts.init(memoryInfochart.value);

    const option = {
      title: {
        text: props.title,
      },
      tooltip: {
        trigger: 'axis',
        formatter: function (params) {
          let tooltipText = `${params[0].name}<br/>`; // 显示时间
          params.forEach(item => {
            tooltipText += `${item.marker}${item.seriesName}: ${item.value}<br/>`;
          });
          return tooltipText;
        }
      },
      legend: {
        orient: 'vertical',
        top: '10%',
        right: '1%',
        data: ['Used', 'Total', 'Usage(%)'],
      },
      grid: {
        left: '3%',
        right: '10%',
        bottom: '3%',
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: timeData.value
      },
      yAxis: [
        {
          type: 'value',
          name: 'MB',
          axisLabel: {
            formatter: function (value) {
              return value + ' MB';
            }
          }
        },
        {
          type: 'value',
          name: 'Usage(%)',
          position: 'right',
          axisLabel: {
            formatter: function (value) {
              return value + ' %';  // 显示百分比并保留两位小数
            }
          }
        }
      ],
      series: [
        {
          name: 'Used',
          type: 'line',
          areaStyle: {},
          data: memoryInfoUsed.value,
        },
        {
          name: 'Total',
          type: 'line',
          areaStyle: {},
          data: memoryInfoTotal.value,
        },
        {
          name: 'Usage(%)',
          type: 'line',
          yAxisIndex: 1,
          data: memoryInfoPercentage.value,
        },
      ],
    };

    chartInstance.setOption(option);
  }
};

// 更新图表数据
const updateChart = () => {
  if (chartInstance) {
    chartInstance.setOption({
      xAxis: {
        data: timeData.value
      },
      series: [
        { data: memoryInfoUsed.value },
        { data: memoryInfoTotal.value },
        { data: memoryInfoPercentage.value }
      ]
    });
  }
};

// 调整图表大小
const resizeChart = () => {
  if (chartInstance) {
    chartInstance.resize();
  }
};

onMounted(() => {
  initmemoryInfoChart();
  window.addEventListener('resize', resizeChart);
  const resizeObserver = new ResizeObserver(() => {
    resizeChart();
  });

  if (memoryInfochart.value) {
    resizeObserver.observe(memoryInfochart.value);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeChart);
  if (chartInstance) {
    chartInstance.dispose();
  }
  clearInterval(intervalId);
});

const intervalId = setInterval(initMemoryInfomemoryInfoData, 3000);

</script>

<style scoped>
/* Optional styling */
</style>
