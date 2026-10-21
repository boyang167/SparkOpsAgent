<template>
  <div ref="chart" className="chart-container"></div>
</template>

<script setup>
import {onBeforeUnmount, onMounted, ref} from "vue";
import * as echarts from "echarts";

const chart = ref(null);
let chartInstance = null;

const props = defineProps({
  tableData: {
    type: Array,
    default: () => [],
  },
});

const collectionName = ref([]);
const collectionCount = ref([]);
const collectionTime = ref([]);

// 初始化数据函数
const initgcInfosData = () => {
  collectionName.value = [];
  collectionCount.value = [];
  collectionTime.value = [];

  props.tableData.forEach(item => {
    if (item.name) {
      collectionName.value.push(item.name);
    }
    collectionCount.value.push(item.collectionCount || 0);
    collectionTime.value.push(item.collectionTime || 0);
  });

  // 如果 chartInstance 已经存在，只更新数据
  if (chartInstance) {
    updateChart();
  }
};

// 初始化图表
const initChart = () => {
  if (chart.value) {
    chartInstance = echarts.init(chart.value);
    const option = {
      title: {
        text: "GC",
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'shadow',
        },
      },
      legend: {
        data: ['collectionCount', 'collectionTime'],
        top: 30,
      },
      xAxis: {
        type: 'category',
        data: collectionName.value,
      },
      yAxis: [
        {
          type: 'value',
          name: 'collectionCount',
          position: 'left',
          axisLabel: {
            formatter: '{value}',
          },
        },
        {
          type: 'value',
          name: 'collectionTime',
          position: 'right',
          axisLabel: {
            formatter: '{value} ms',
          },
        },
      ],
      series: [
        {
          name: 'collectionCount',
          type: 'bar',
          data: collectionCount.value,
          itemStyle: {
            color: '#4A69BB',
          },
        },
        {
          name: 'collectionTime',
          type: 'bar',
          data: collectionTime.value,
          yAxisIndex: 1,
          itemStyle: {
            color: '#78C850',
          },
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
        data: collectionName.value, // 更新 X 轴数据
      },
      series: [
        {
          data: collectionCount.value, // 更新 collectionCount 数据
        },
        {
          data: collectionTime.value, // 更新 collectionTime 数据
        },
      ],
    });
  }
};

// 调整图表大小
const resizeChart = () => {
  if (chartInstance) {
    chartInstance.resize();
  }
};

// 生命周期钩子
onMounted(() => {
  initgcInfosData(); // 初始化数据
  initChart();       // 初始化图表

  window.addEventListener('resize', resizeChart);

  // 使用 ResizeObserver 来监听 chart 容器的大小变化
  const resizeObserver = new ResizeObserver(() => {
    resizeChart();
  });

  if (chart.value) {
    resizeObserver.observe(chart.value);
  }

  // 每隔10秒更新数据并更新图表
  setInterval(() => {
    initgcInfosData(); // 只更新数据
  }, 10000);
});

onBeforeUnmount(() => {
  // 移除窗口大小变化监听器
  window.removeEventListener('resize', resizeChart);
  if (chartInstance) {
    chartInstance.dispose(); // 清理图表实例
  }
});
</script>

<style scoped>
.chart-container {
  width: 100%; /* 图表宽度自适应 */
  height: 400px;
}
</style>
