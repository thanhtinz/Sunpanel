<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  NCard,
  NDataTable,
  NTabPane,
  NTabs,
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, auditApi, type AuditLog, type LoginLog } from '@/api'
import { translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const message = useMessage()

const tab = ref<'actions' | 'logins'>('actions')

const actionLogs = ref<AuditLog[]>([])
const loginLogs = ref<LoginLog[]>([])
const loading = ref(false)

const page = ref(1)
const pageSize = 20
const total = ref(0)

const pagination = computed(() => ({
  page: page.value,
  pageSize,
  itemCount: total.value,
  showSizePicker: false,
  onUpdatePage: (next: number) => {
    page.value = next
  },
}))

onMounted(load)

// Đổi tab hoặc đổi trang đều phải nạp lại; đổi tab thì quay về trang đầu.
watch(tab, () => {
  page.value = 1
  void load()
})
watch(page, load)

async function load(): Promise<void> {
  loading.value = true
  try {
    if (tab.value === 'actions') {
      const result = await auditApi.list(page.value, pageSize)
      actionLogs.value = result.items ?? []
      total.value = result.total
    } else {
      const result = await auditApi.logins(page.value, pageSize)
      loginLogs.value = result.items ?? []
      total.value = result.total
    }
  } catch (err) {
    message.error(
      err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'),
    )
  } finally {
    loading.value = false
  }
}

function resultTag(success: boolean) {
  return h(
    NTag,
    { size: 'small', type: success ? 'success' : 'error' },
    { default: () => (success ? t('audit.successful') : t('audit.failed')) },
  )
}

const actionColumns = computed<DataTableColumns<AuditLog>>(() => [
  { title: t('audit.time'), key: 'createdAt', render: (row) => formatDateTime(row.createdAt) },
  { title: t('audit.user'), key: 'username', render: (row) => row.username || '—' },
  { title: t('audit.action'), key: 'action' },
  {
    title: t('audit.resource'),
    key: 'resource',
    ellipsis: { tooltip: true },
    render: (row) => row.resource || '—',
  },
  { title: t('audit.ip'), key: 'ip', render: (row) => row.ip || '—' },
  { title: t('audit.result'), key: 'success', render: (row) => resultTag(row.success) },
])

const loginColumns = computed<DataTableColumns<LoginLog>>(() => [
  { title: t('audit.time'), key: 'createdAt', render: (row) => formatDateTime(row.createdAt) },
  { title: t('audit.user'), key: 'username', render: (row) => row.username || '—' },
  { title: t('audit.ip'), key: 'ip', render: (row) => row.ip || '—' },
  {
    title: t('audit.device'),
    key: 'userAgent',
    ellipsis: { tooltip: true },
    render: (row) => h(NText, { depth: 3 }, { default: () => row.userAgent || '—' }),
  },
  { title: t('audit.result'), key: 'success', render: (row) => resultTag(row.success) },
  {
    title: t('audit.reason'),
    key: 'reason',
    // Lý do thất bại là mã dịch từ máy chủ, phải dịch trước khi hiển thị.
    render: (row) => (row.reason ? translateError(row.reason) : '—'),
  },
])
</script>

<template>
  <!-- Thẻ không đặt tiêu đề: thanh tiêu đề của panel đã nói đây là trang nào,
       nhắc lại lần nữa chỉ tốn một dòng ngay chỗ cần cho nội dung. -->
  <NCard size="small">
    <NTabs v-model:value="tab" type="line" animated>
      <NTabPane name="actions" :tab="t('audit.actions')">
        <NDataTable
          :columns="actionColumns"
          :data="actionLogs"
          :loading="loading"
          :pagination="pagination"
          :row-key="(row: AuditLog) => row.id"
          remote
          size="small"
        />
      </NTabPane>

      <NTabPane name="logins" :tab="t('audit.logins')">
        <NDataTable
          :columns="loginColumns"
          :data="loginLogs"
          :loading="loading"
          :pagination="pagination"
          :row-key="(row: LoginLog) => row.id"
          remote
          size="small"
        />
      </NTabPane>
    </NTabs>
  </NCard>
</template>
