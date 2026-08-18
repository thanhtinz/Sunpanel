<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSpace,
  NTag,
  NText,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, uptimeApi, type UptimeCheck, type UptimeMonitor } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const monitors = ref<UptimeMonitor[]>([])
const loading = ref(false)

/** Lịch sử của từng mục, dùng vẽ dải trạng thái. */
const history = ref<Record<number, UptimeCheck[]>>({})

const editor = ref({ show: false, saving: false, form: emptyMonitor() })

function emptyMonitor() {
  return {
    id: 0,
    name: '',
    url: 'https://',
    intervalSeconds: 60,
    timeoutSeconds: 10,
    expectedStatus: 0,
    keyword: '',
    skipTlsVerify: false,
    failureThreshold: 2,
    enabled: true,
  }
}

/**
 * Chu kỳ làm mới bảng.
 *
 * Panel kiểm tra theo chu kỳ riêng của từng mục; trang này chỉ đọc lại kết quả,
 * nên hai mươi giây là đủ để thấy thay đổi mà không hỏi máy chủ liên tục.
 */
const refreshInterval = 20000
let timer: number | undefined

onMounted(async () => {
  await load()
  timer = window.setInterval(() => void load(true), refreshInterval)
})

onUnmounted(() => window.clearInterval(timer))

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(silent = false): Promise<void> {
  if (!silent) loading.value = true
  try {
    monitors.value = await uptimeApi.list()
    await Promise.all(
      monitors.value.map(async (monitor) => {
        history.value[monitor.id] = await uptimeApi.history(monitor.id, 40).catch(() => [])
      }),
    )
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editor.value = { show: true, saving: false, form: emptyMonitor() }
}

function openEdit(monitor: UptimeMonitor): void {
  editor.value = {
    show: true,
    saving: false,
    form: {
      id: monitor.id,
      name: monitor.name,
      url: monitor.url,
      intervalSeconds: monitor.intervalSeconds,
      timeoutSeconds: monitor.timeoutSeconds,
      expectedStatus: monitor.expectedStatus,
      keyword: monitor.keyword,
      skipTlsVerify: monitor.skipTlsVerify,
      failureThreshold: monitor.failureThreshold,
      enabled: monitor.enabled,
    },
  }
}

async function save(): Promise<void> {
  const form = editor.value.form
  editor.value.saving = true
  try {
    const payload = { ...form, name: form.name.trim(), url: form.url.trim() }
    if (form.id === 0) {
      await uptimeApi.create(payload)
      message.success(t('uptime.created'))
    } else {
      await uptimeApi.update(form.id, payload)
      message.success(t('uptime.updated'))
    }
    editor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

async function checkNow(monitor: UptimeMonitor): Promise<void> {
  try {
    await uptimeApi.check(monitor.id)
    await load(true)
  } catch (err) {
    report(err)
  }
}

async function remove(monitor: UptimeMonitor): Promise<void> {
  try {
    await uptimeApi.remove(monitor.id)
    message.success(t('uptime.deleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

/**
 * Hàng của bảng, gộp sẵn lịch sử vào từng mục.
 *
 * Cột dải trạng thái phải đọc dữ liệu từ chính hàng: nếu nó đọc thẳng từ biến
 * lịch sử bên ngoài thì bảng không biết mình cần vẽ lại, và dải trạng thái nằm
 * trống cho tới khi có thứ khác vô tình làm bảng cập nhật.
 */
type Row = UptimeMonitor & { recent: UptimeCheck[] }

const rows = computed<Row[]>(() =>
  monitors.value.map((monitor) => ({ ...monitor, recent: history.value[monitor.id] ?? [] })),
)

const statusTone: Record<string, 'success' | 'error' | 'default'> = {
  up: 'success',
  down: 'error',
  unknown: 'default',
}

/**
 * Dải trạng thái: mỗi vạch là một lần kiểm tra, cũ nhất bên trái.
 *
 * Kiểu viết thẳng vào thẻ chứ không dùng lớp CSS: các nút này được dựng bằng
 * h() trong hàm vẽ cột, nên chúng không mang thuộc tính phạm vi mà khối style
 * scoped dựa vào — quy tắc CSS sẽ không khớp và dải trạng thái nằm trống.
 */
function renderBar(row: Row) {
  const checks = row.recent
  if (!checks.length) return h(NText, { depth: 3 }, { default: () => '—' })

  return h(
    'div',
    { style: 'display: flex; gap: 2px; align-items: flex-end; height: 20px' },
    checks.map((check) =>
      h(NTooltip, null, {
        trigger: () =>
          h('span', {
            style: [
              'display: inline-block; width: 4px; height: 100%; border-radius: 1px',
              `background: ${check.up ? 'var(--sp-action)' : 'var(--sp-danger)'}`,
            ].join('; '),
          }),
        default: () =>
          `${formatDateTime(check.checkedAt)} · ${check.up ? t('uptime.up') : check.error || t('uptime.down')} · ${check.latencyMs} ms`,
      }),
    ),
  )
}

const columns = computed<DataTableColumns<Row>>(() => [
  {
    title: t('uptime.name'),
    key: 'name',
    width: 220,
    render: (row) =>
      h(NSpace, { size: 6, align: 'center', wrap: false }, {
        default: () => [
          h(NTag, { size: 'small', type: statusTone[row.status], bordered: false }, {
            default: () => t(`uptime.${row.status}`),
          }),
          h(NText, { strong: true }, { default: () => row.name }),
          row.enabled ? null : h(NTag, { size: 'tiny', bordered: false }, { default: () => t('uptime.paused') }),
        ],
      }),
  },
  { title: t('uptime.url'), key: 'url', ellipsis: { tooltip: true } },
  {
    title: t('uptime.recent'),
    key: 'bar',
    width: 220,
    render: renderBar,
  },
  {
    title: t('uptime.uptime24h'),
    key: 'uptime24h',
    width: 110,
    render: (row) => (row.checks ? `${row.uptime24h.toFixed(1)}%` : '—'),
  },
  {
    title: t('uptime.latency'),
    key: 'lastLatencyMs',
    width: 120,
    render: (row) => (row.lastCheckedAt ? `${row.lastLatencyMs} ms` : '—'),
  },
  {
    title: t('uptime.lastCheck'),
    key: 'lastCheckedAt',
    width: 190,
    render: (row) => {
      if (!row.lastCheckedAt) return '—'
      // Lỗi gần nhất quan trọng hơn thời điểm: nó nói vì sao trang đang hỏng.
      if (row.status === 'down' && row.lastError) {
        return h(NSpace, { vertical: true, size: 0 }, {
          default: () => [
            h(NText, { type: 'error', style: 'font-size: 12px' }, { default: () => row.lastError }),
            h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => formatDateTime(row.lastCheckedAt) }),
          ],
        })
      }
      return h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => formatDateTime(row.lastCheckedAt) })
    },
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row) => {
      if (!auth.canWrite) return null
      return h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => checkNow(row) },
            { default: () => t('uptime.checkNow') }),
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
            { default: () => t('common.edit') }),
          h(NPopconfirm, { onPositiveClick: () => remove(row) }, {
            trigger: () =>
              h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
                { default: () => t('common.delete') }),
            default: () => t('uptime.deleteConfirm', { name: row.name }),
          }),
        ],
      })
    },
  },
])

const expiringCerts = computed(() =>
  monitors.value.filter((monitor) => monitor.certExpiresIn >= 0 && monitor.certExpiresIn <= 14),
)
</script>

<template>
  <NCard size="small">
    <template #header><span /></template>

    <template #header-extra>
      <NSpace align="center" :size="10">
        <NButton size="small" :loading="loading" @click="load()">{{ t('common.refresh') }}</NButton>
        <NButton v-if="auth.canWrite" size="small" type="primary" @click="openCreate">
          {{ t('uptime.create') }}
        </NButton>
      </NSpace>
    </template>

    <NSpace vertical :size="12">
      <NAlert v-for="monitor in expiringCerts" :key="monitor.id" type="warning" :bordered="false">
        {{ t('uptime.certExpiring', { name: monitor.name, days: monitor.certExpiresIn }) }}
      </NAlert>

      <NDataTable
        :columns="columns"
        :data="rows"
        :loading="loading"
        :row-key="(row: Row) => row.id"
        size="small"
      />
    </NSpace>
  </NCard>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="editor.form.id === 0 ? t('uptime.create') : t('uptime.edit')"
    style="width: 92vw; max-width: 560px"
  >
    <NForm label-placement="top" @submit.prevent="save">
      <NFormItem :label="t('uptime.name')">
        <NInput v-model:value="editor.form.name" autofocus placeholder="Trang chủ" />
      </NFormItem>

      <NFormItem :label="t('uptime.url')">
        <NInput v-model:value="editor.form.url" placeholder="https://example.com" />
      </NFormItem>

      <NSpace :size="12">
        <NFormItem :label="t('uptime.interval')">
          <NInputNumber v-model:value="editor.form.intervalSeconds" :min="30" :max="86400" />
        </NFormItem>
        <NFormItem :label="t('uptime.timeout')">
          <NInputNumber v-model:value="editor.form.timeoutSeconds" :min="1" :max="120" />
        </NFormItem>
        <NFormItem :label="t('uptime.threshold')" >
          <NInputNumber v-model:value="editor.form.failureThreshold" :min="1" :max="10" />
        </NFormItem>
      </NSpace>

      <NFormItem :label="t('uptime.expectedStatus')" :feedback="t('uptime.expectedStatusHelp')">
        <NInputNumber v-model:value="editor.form.expectedStatus" :min="0" :max="599" style="width: 100%" />
      </NFormItem>

      <NFormItem :label="t('uptime.keyword')" :feedback="t('uptime.keywordHelp')">
        <NInput v-model:value="editor.form.keyword" />
      </NFormItem>

      <NSpace vertical :size="8" style="margin-bottom: 14px">
        <NCheckbox v-model:checked="editor.form.skipTlsVerify">{{ t('uptime.skipTls') }}</NCheckbox>
        <NCheckbox v-model:checked="editor.form.enabled">{{ t('uptime.enabled') }}</NCheckbox>
      </NSpace>

      <NButton type="primary" block attr-type="submit" :loading="editor.saving">
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>
</template>
