<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NInput,
  NPopconfirm,
  NSpace,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, processApi, type PortListener, type ProcessInfo } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const tab = ref<'processes' | 'listeners'>('processes')
const processes = ref<ProcessInfo[]>([])
const listeners = ref<PortListener[]>([])
const total = ref(0)
const truncated = ref(false)
const loading = ref(false)
const search = ref('')
const live = ref(true)

/**
 * Chu kỳ làm mới bảng tiến trình.
 *
 * Phần trăm CPU là hiệu giữa hai lần đọc, nên bảng chỉ có số khi được đọc lại
 * đều đặn. Năm giây đủ dày để thấy một tiến trình đang ngốn CPU mà vẫn đủ thưa
 * để việc đọc /proc không tự nó thành tải.
 */
const refreshInterval = 5000
let timer: number | undefined

onMounted(() => {
  void load()
  start()
})

onUnmounted(stop)

// Rời khỏi bảng tiến trình thì dừng đọc lại: bảng cổng không cần làm tươi liên tục.
watch(tab, (value) => {
  if (value === 'processes') start()
  else stop()
  void load()
})

watch(live, (value) => (value ? start() : stop()))

// Gõ tìm kiếm thì hỏi lại máy chủ thay vì lọc tại chỗ: bảng đã bị cắt bớt ở
// phía máy chủ, nên lọc trên phần đã cắt sẽ bỏ sót đúng tiến trình cần tìm.
let searchTimer: number | undefined
watch(search, () => {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => void load(), 300)
})

function start(): void {
  stop()
  if (!live.value || tab.value !== 'processes') return
  timer = window.setInterval(() => void load(true), refreshInterval)
}

function stop(): void {
  window.clearInterval(timer)
  timer = undefined
}

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

/** silent dùng cho lần đọc tự động: không bật vòng quay để bảng không nháy mỗi 5 giây. */
async function load(silent = false): Promise<void> {
  if (!silent) loading.value = true
  try {
    if (tab.value === 'processes') {
      const result = await processApi.list(search.value.trim())
      processes.value = result.items
      total.value = result.total
      truncated.value = result.truncated
    } else {
      listeners.value = await processApi.listeners()
    }
  } catch (err) {
    report(err)
    stop()
  } finally {
    loading.value = false
  }
}

async function kill(row: ProcessInfo, force: boolean): Promise<void> {
  try {
    await processApi.kill(row.pid, force)
    message.success(t('processes.killed', { name: row.name }))
    await load(true)
  } catch (err) {
    report(err)
  }
}

/** Trạng thái tiến trình từ hệ điều hành, đặt tên theo cách người đọc hiểu được. */
const statusLabel: Record<string, string> = {
  R: 'running',
  S: 'sleeping',
  I: 'sleeping',
  D: 'waiting',
  T: 'stopped',
  Z: 'zombie',
  running: 'running',
  sleep: 'sleeping',
  idle: 'sleeping',
  stop: 'stopped',
  zombie: 'zombie',
}

function statusText(status: string): string {
  const key = statusLabel[status]
  return key ? t(`processes.${key}`) : status || '—'
}

const columns = computed<DataTableColumns<ProcessInfo>>(() => [
  { title: t('processes.pid'), key: 'pid', width: 90 },
  {
    title: t('processes.name'),
    key: 'name',
    width: 200,
    render: (row) =>
      h(NSpace, { size: 6, align: 'center', wrap: false }, {
        default: () => [
          h(NText, { strong: true }, { default: () => row.name }),
          row.protected
            ? h(NTooltip, null, {
                trigger: () =>
                  h(NTag, { size: 'tiny', type: 'info', bordered: false },
                    { default: () => t('processes.protected') }),
                default: () => t('processes.protectedHint'),
              })
            : null,
        ],
      }),
  },
  { title: t('processes.user'), key: 'username', width: 120, render: (row) => row.username || '—' },
  {
    title: t('processes.cpu'),
    key: 'cpu',
    width: 100,
    sorter: (a, b) => a.cpu - b.cpu,
    render: (row) => `${row.cpu.toFixed(1)}%`,
  },
  {
    title: t('processes.memory'),
    key: 'memoryRss',
    width: 130,
    sorter: (a, b) => a.memoryRss - b.memoryRss,
    render: (row) => `${formatBytes(row.memoryRss)} · ${row.memoryPercent.toFixed(1)}%`,
  },
  {
    title: t('processes.status'),
    key: 'status',
    width: 110,
    render: (row) => statusText(row.status),
  },
  { title: t('processes.command'), key: 'command', ellipsis: { tooltip: true } },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 170,
    render: (row) => {
      if (!auth.canWrite) return null
      // Tiến trình được bảo vệ vẫn hiện nút nhưng khóa lại kèm lời giải thích;
      // ẩn hẳn đi thì người dùng chỉ thấy một ô trống không hiểu vì sao.
      const button = (label: string, force: boolean) =>
        h(NPopconfirm,
          { onPositiveClick: () => kill(row, force) },
          {
            trigger: () =>
              h(NButton,
                { size: 'tiny', quaternary: true, type: force ? 'error' : 'default', disabled: row.protected },
                { default: () => label }),
            default: () =>
              t(force ? 'processes.forceConfirm' : 'processes.killConfirm', {
                name: row.name,
                pid: row.pid,
              }),
          })
      return h(NSpace, { size: 4 }, {
        default: () => [button(t('processes.kill'), false), button(t('processes.force'), true)],
      })
    },
  },
])

const listenerColumns = computed<DataTableColumns<PortListener>>(() => [
  { title: t('processes.port'), key: 'port', width: 100, sorter: (a, b) => a.port - b.port },
  {
    title: t('processes.protocol'),
    key: 'protocol',
    width: 110,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => row.protocol }),
  },
  {
    title: t('processes.address'),
    key: 'address',
    width: 200,
    render: (row) => row.address || '*',
  },
  { title: t('processes.pid'), key: 'pid', width: 100, render: (row) => row.pid || '—' },
  { title: t('processes.name'), key: 'process', render: (row) => row.process || '—' },
])
</script>

<template>
  <NCard size="small">
    <template #header><span /></template>

    <template #header-extra>
      <NSpace align="center" :size="10">
        <NSpace v-if="tab === 'processes'" align="center" :size="6">
          <NSwitch v-model:value="live" size="small" />
          <NText depth="3" style="font-size: 12px">{{ t('processes.live') }}</NText>
        </NSpace>
        <NInput
          v-if="tab === 'processes'"
          v-model:value="search"
          size="small"
          clearable
          :placeholder="t('processes.search')"
          style="width: 220px"
        />
        <NButton size="small" :loading="loading" @click="load()">{{ t('common.refresh') }}</NButton>
      </NSpace>
    </template>

    <NTabs v-model:value="tab" type="line" size="small">
      <NTabPane name="processes" :tab="t('processes.tabProcesses')">
        <NAlert v-if="truncated" type="info" :bordered="false" style="margin-bottom: 12px">
          {{ t('processes.truncated', { shown: processes.length, total }) }}
        </NAlert>

        <NDataTable
          :columns="columns"
          :data="processes"
          :loading="loading"
          :row-key="(row: ProcessInfo) => row.pid"
          size="small"
          max-height="calc(100vh - 300px)"
          virtual-scroll
        />
      </NTabPane>

      <NTabPane name="listeners" :tab="t('processes.tabListeners')">
        <NDataTable
          :columns="listenerColumns"
          :data="listeners"
          :loading="loading"
          :row-key="(row: PortListener) => `${row.protocol}-${row.address}-${row.port}-${row.pid}`"
          size="small"
          max-height="calc(100vh - 300px)"
          virtual-scroll
        />
      </NTabPane>
    </NTabs>
  </NCard>
</template>
