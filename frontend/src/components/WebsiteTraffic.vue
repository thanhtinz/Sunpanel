<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NButtonGroup,
  NCard,
  NDataTable,
  NEmpty,
  NGrid,
  NGridItem,
  NSpace,
  NSpin,
  NStatistic,
  NTabPane,
  NTabs,
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  websiteApi,
  type TrafficCount,
  type TrafficFailure,
  type TrafficReport,
} from '@/api'
import { translateError } from '@/locales'
import { formatBytes } from '@/utils/format'
import LineChart from '@/components/LineChart.vue'

const props = defineProps<{ siteId: number | null }>()

const { t } = useI18n()
const message = useMessage()

const report = ref<TrafficReport | null>(null)
const loading = ref(false)
const window_ = ref('24h')

const windows = ['1h', '6h', '24h', '7d']

watch(
  () => [props.siteId, window_.value],
  () => void load(),
  { immediate: true },
)

async function load(): Promise<void> {
  if (props.siteId === null) return
  loading.value = true
  try {
    report.value = await websiteApi.traffic(props.siteId, window_.value)
  } catch (err) {
    message.error(
      err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'),
    )
    report.value = null
  } finally {
    loading.value = false
  }
}

/**
 * Nhãn trục hoành.
 *
 * Khung dài từ một ngày trở lên thì giờ không còn phân biệt được hai cột, nên
 * nhãn phải mang theo ngày.
 */
const labels = computed(() => {
  const daily = (report.value?.bucketSeconds ?? 0) >= 86400
  return (report.value?.buckets ?? []).map((bucket) =>
    daily
      ? new Date(bucket.start).toLocaleDateString(undefined, { day: '2-digit', month: '2-digit' })
      : new Date(bucket.start).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' }),
  )
})

const series = computed(() => [
  {
    name: t('traffic.requests'),
    data: (report.value?.buckets ?? []).map((bucket) => bucket.requests),
    color: '#15a34a',
  },
  {
    name: t('traffic.errors'),
    data: (report.value?.buckets ?? []).map((bucket) => bucket.errors),
    color: '#dc2626',
  },
])

/** Tỉ lệ của một dòng so với dòng đứng đầu, để vẽ thanh nền. */
function share(rows: TrafficCount[], row: TrafficCount): number {
  const top = rows[0]?.count ?? 0
  return top > 0 ? (row.count * 100) / top : 0
}

/**
 * Bảng xếp hạng.
 *
 * Thanh nền vẽ bằng gradient nền của chính ô chứ không phải một phần tử riêng:
 * các ô này được dựng bằng h() trong hàm vẽ cột nên chúng không mang thuộc tính
 * phạm vi mà khối style scoped dựa vào.
 */
function rankColumns(rows: () => TrafficCount[], title: string): DataTableColumns<TrafficCount> {
  return [
    {
      title,
      key: 'key',
      render: (row) =>
        h(
          'div',
          {
            title: row.key,
            style: [
              'padding: 2px 6px; border-radius: 4px',
              // Ô phải trải hết chiều ngang thì thanh nền mới đọc được thành tỉ lệ:
              // để nó co theo nội dung thì dòng có đường dẫn ngắn nhất trông như
              // dòng ít lượt nhất.
              'display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap',
              `background: linear-gradient(to right, color-mix(in srgb, var(--sp-action) 14%, transparent) ${share(rows(), row)}%, transparent ${share(rows(), row)}%)`,
            ].join('; '),
          },
          row.key,
        ),
    },
    {
      title: t('traffic.hits'),
      key: 'count',
      width: 90,
      render: (row) => row.count.toLocaleString(),
    },
  ]
}

const failureColumns = computed<DataTableColumns<TrafficFailure>>(() => [
  {
    title: t('traffic.status'),
    key: 'status',
    width: 90,
    render: (row) =>
      h(
        NTag,
        {
          size: 'small',
          bordered: false,
          type: row.status >= 500 ? 'error' : 'warning',
        },
        { default: () => row.status },
      ),
  },
  { title: t('traffic.path'), key: 'path', ellipsis: { tooltip: true } },
  { title: t('traffic.ip'), key: 'ip', width: 150 },
  {
    title: t('traffic.time'),
    key: 'time',
    width: 170,
    render: (row) =>
      h(
        NText,
        { depth: 3, style: 'font-size: 12px' },
        { default: () => new Date(row.time).toLocaleString() },
      ),
  },
])
</script>

<template>
  <NSpace vertical :size="14">
    <NSpace align="center" justify="space-between">
      <NButtonGroup size="small">
        <NButton
          v-for="value in windows"
          :key="value"
          :type="window_ === value ? 'primary' : 'default'"
          @click="window_ = value"
        >
          {{ t(`traffic.window.${value}`) }}
        </NButton>
      </NButtonGroup>
      <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
    </NSpace>

    <NSpin :show="loading">
      <NEmpty v-if="report && report.requests === 0" :description="t('traffic.empty')">
        <template #extra>
          <NText depth="3" style="font-size: 12px">{{ report.logPath }}</NText>
        </template>
      </NEmpty>

      <NSpace v-else-if="report" vertical :size="14">
        <NAlert v-if="report.truncated" type="info" :bordered="false">
          {{ t('traffic.truncated') }}
        </NAlert>

        <NGrid :x-gap="12" :y-gap="12" cols="2 s:4" responsive="screen">
          <NGridItem>
            <NCard size="small" style="height: 100%">
              <NStatistic
                :label="t('traffic.requests')"
                :value="report.requests.toLocaleString()"
              />
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" style="height: 100%">
              <NStatistic
                :label="t('traffic.visitors')"
                :value="report.visitors.toLocaleString()"
              />
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" style="height: 100%">
              <NStatistic :label="t('traffic.bandwidth')" :value="formatBytes(report.bytes)" />
            </NCard>
          </NGridItem>
          <NGridItem>
            <NCard size="small" style="height: 100%">
              <NStatistic :label="t('traffic.errorRate')">
                {{
                  report.requests
                    ? (((report.status4xx + report.status5xx) * 100) / report.requests).toFixed(1)
                    : '0.0'
                }}%
              </NStatistic>
              <NText depth="3" style="font-size: 12px">
                {{
                  t('traffic.errorSplit', {
                    four: report.status4xx,
                    five: report.status5xx,
                  })
                }}
              </NText>
            </NCard>
          </NGridItem>
        </NGrid>

        <NCard size="small" :title="t('traffic.overTime')">
          <LineChart
            :labels="labels"
            :series="series"
            height="220px"
            :formatter="(v) => String(v)"
          />
        </NCard>

        <NTabs type="line" size="small">
          <NTabPane name="paths" :tab="t('traffic.topPaths')">
            <NDataTable
              :columns="rankColumns(() => report!.topPaths, t('traffic.path'))"
              :data="report.topPaths"
              :row-key="(row: TrafficCount) => row.key"
              size="small"
            />
          </NTabPane>
          <NTabPane name="ips" :tab="t('traffic.topIps')">
            <NDataTable
              :columns="rankColumns(() => report!.topIps, t('traffic.ip'))"
              :data="report.topIps"
              :row-key="(row: TrafficCount) => row.key"
              size="small"
            />
          </NTabPane>
          <NTabPane name="referrers" :tab="t('traffic.topReferrers')">
            <NDataTable
              :columns="rankColumns(() => report!.topReferrers, t('traffic.referrer'))"
              :data="report.topReferrers"
              :row-key="(row: TrafficCount) => row.key"
              size="small"
            />
          </NTabPane>
          <NTabPane name="agents" :tab="t('traffic.topAgents')">
            <NDataTable
              :columns="rankColumns(() => report!.topAgents, t('traffic.agent'))"
              :data="report.topAgents"
              :row-key="(row: TrafficCount) => row.key"
              size="small"
            />
          </NTabPane>
          <NTabPane name="failures" :tab="t('traffic.failures')">
            <NDataTable
              v-if="report.failures.length"
              :columns="failureColumns"
              :data="report.failures"
              :row-key="(row: TrafficFailure) => `${row.time}-${row.path}`"
              size="small"
              max-height="300"
            />
            <NEmpty v-else :description="t('traffic.noFailures')" />
          </NTabPane>
        </NTabs>
      </NSpace>
    </NSpin>
  </NSpace>
</template>
