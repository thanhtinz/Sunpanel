<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NBreadcrumb,
  NBreadcrumbItem,
  NButton,
  NCard,
  NDataTable,
  NProgress,
  NSpace,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, diskApi, type DiskEntry, type DiskUsage } from '@/api'
import { translateError } from '@/locales'
import { formatBytes } from '@/utils/format'

const { t } = useI18n()
const message = useMessage()

const partitions = ref<DiskUsage[]>([])
const path = ref('/')
const entries = ref<DiskEntry[]>([])
const total = ref(0)
const partial = ref(false)
const files = ref(0)
const duration = ref(0)
const loading = ref(false)

const breadcrumbs = computed(() => {
  const parts = path.value.split('/').filter(Boolean)
  const crumbs = [{ label: '/', path: '/' }]
  let accumulated = ''
  for (const part of parts) {
    accumulated += `/${part}`
    crumbs.push({ label: part, path: accumulated })
  }
  return crumbs
})

onMounted(async () => {
  partitions.value = await diskApi.partitions().catch(() => [])
  await scan('/')
})

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function scan(target: string): Promise<void> {
  loading.value = true
  try {
    const result = await diskApi.usage(target)
    path.value = result.path
    entries.value = result.entries ?? []
    total.value = result.total
    partial.value = result.partial
    files.value = result.files
    duration.value = result.durationMs
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/** Màu theo tỉ lệ chiếm chỗ: mục ngốn hơn một nửa thư mục là thứ cần nhìn trước. */
function tone(percent: number): 'error' | 'warning' | 'default' {
  if (percent >= 50) return 'error'
  if (percent >= 20) return 'warning'
  return 'default'
}

const columns = computed<DataTableColumns<DiskEntry>>(() => [
  {
    title: t('disk.name'),
    key: 'name',
    render: (row) =>
      row.isDir
        ? h(
            NButton,
            { text: true, type: 'primary', onClick: () => scan(row.path) },
            { default: () => `${row.name}/` },
          )
        : h(NText, null, { default: () => row.name }),
  },
  {
    title: t('disk.size'),
    key: 'size',
    width: 130,
    render: (row) => formatBytes(row.size),
  },
  {
    title: t('disk.share'),
    key: 'percent',
    width: 240,
    render: (row) =>
      h(NSpace, { align: 'center', size: 8, wrap: false }, {
        default: () => [
          h(NProgress, {
            type: 'line',
            percentage: Math.min(100, Math.round(row.percent)),
            height: 6,
            showIndicator: false,
            status: tone(row.percent) === 'error' ? 'error' : tone(row.percent) === 'warning' ? 'warning' : 'default',
            style: 'width: 150px',
          }),
          h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => `${row.percent.toFixed(1)}%` }),
        ],
      }),
  },
  {
    title: t('disk.files'),
    key: 'files',
    width: 110,
    render: (row) => (row.isDir ? row.files.toLocaleString() : '—'),
  },
])

const partitionColumns = computed<DataTableColumns<DiskUsage>>(() => [
  { title: t('disk.mount'), key: 'mountpoint', width: 180 },
  { title: t('disk.device'), key: 'device', ellipsis: { tooltip: true } },
  { title: t('disk.fstype'), key: 'fstype', width: 110 },
  { title: t('disk.total'), key: 'total', width: 110, render: (row) => formatBytes(row.total) },
  { title: t('disk.used'), key: 'used', width: 110, render: (row) => formatBytes(row.used) },
  { title: t('disk.free'), key: 'free', width: 110, render: (row) => formatBytes(row.free) },
  {
    title: t('disk.usedPercent'),
    key: 'percent',
    width: 200,
    render: (row) =>
      h(NProgress, {
        type: 'line',
        percentage: Math.round(row.percent),
        height: 6,
        status: row.percent >= 90 ? 'error' : row.percent >= 75 ? 'warning' : 'default',
        style: 'width: 160px',
      }),
  },
  {
    title: '',
    key: 'actions',
    width: 110,
    render: (row) =>
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => scan(row.mountpoint) },
        { default: () => t('disk.analyze') }),
  },
])
</script>

<template>
  <NSpace vertical :size="16">
    <NCard size="small" :title="t('disk.partitions')">
      <NDataTable
        :columns="partitionColumns"
        :data="partitions"
        :row-key="(row: DiskUsage) => row.mountpoint"
        size="small"
      />
    </NCard>

    <NCard size="small">
      <template #header><span /></template>

      <template #header-extra>
        <NSpace align="center" :size="10">
          <NText depth="3" style="font-size: 12px">
            {{ t('disk.summary', { size: formatBytes(total), files: files.toLocaleString(), ms: duration }) }}
          </NText>
          <NButton size="small" :loading="loading" @click="scan(path)">{{ t('common.refresh') }}</NButton>
        </NSpace>
      </template>

      <NSpace vertical :size="12">
        <NBreadcrumb>
          <NBreadcrumbItem
            v-for="crumb in breadcrumbs"
            :key="crumb.path"
            style="cursor: pointer"
            @click="scan(crumb.path)"
          >
            {{ crumb.label }}
          </NBreadcrumbItem>
        </NBreadcrumb>

        <NAlert v-if="partial" type="warning" :bordered="false">
          {{ t('disk.partial') }}
        </NAlert>

        <NDataTable
          :columns="columns"
          :data="entries"
          :loading="loading"
          :row-key="(row: DiskEntry) => row.path"
          size="small"
          max-height="calc(100vh - 460px)"
          virtual-scroll
        />
      </NSpace>
    </NCard>
  </NSpace>
</template>
