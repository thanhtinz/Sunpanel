<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDropdown,
  NInput,
  NModal,
  NSpace,
  NSwitch,
  NTag,
  NText,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  serviceApi,
  type ServiceAction,
  type ServiceState,
  type SystemService,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const services = ref<SystemService[]>([])
const loading = ref(false)
const managerAvailable = ref(true)
const managerName = ref('systemd')

const search = ref('')
const onlyRunning = ref(false)

const logs = ref({ show: false, name: '', content: '', loading: false })

/** Trạng thái nào tô màu gì — đỏ cho lỗi để mắt bắt được ngay. */
const stateTag: Record<ServiceState, 'success' | 'default' | 'error' | 'warning'> = {
  active: 'success',
  inactive: 'default',
  failed: 'error',
  activating: 'warning',
  deactivating: 'warning',
  unknown: 'default',
}

const stateLabel: Record<ServiceState, string> = {
  active: 'running',
  inactive: 'stopped',
  failed: 'failed',
  activating: 'starting',
  deactivating: 'stopping',
  unknown: 'unknown',
}

const filtered = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return services.value.filter((svc) => {
    if (onlyRunning.value && svc.state !== 'active') return false
    if (!keyword) return true
    return (
      svc.name.toLowerCase().includes(keyword) ||
      svc.description.toLowerCase().includes(keyword)
    )
  })
})

onMounted(async () => {
  const status = await serviceApi.status().catch(() => null)
  if (status) {
    managerAvailable.value = status.available
    managerName.value = status.manager
  }
  if (managerAvailable.value) await load()
})

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    services.value = await serviceApi.list()
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

async function control(name: string, action: ServiceAction): Promise<void> {
  try {
    await serviceApi.control(name, action)
    message.success(t('services.done'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function openLogs(name: string): Promise<void> {
  logs.value = { show: true, name, content: '', loading: true }
  try {
    const result = await serviceApi.logs(name)
    logs.value.content = result.logs.trim() || t('services.noLogs')
  } catch (err) {
    report(err)
    logs.value.content = ''
  } finally {
    logs.value.loading = false
  }
}

/** Thao tác nào khả dụng cho dịch vụ này. */
function actionsFor(row: SystemService) {
  const options: { label: string; key: ServiceAction; disabled?: boolean }[] = []

  if (row.state === 'active') {
    options.push({ label: t('services.restart'), key: 'restart' })
    options.push({ label: t('services.reload'), key: 'reload' })
    // Dịch vụ được bảo vệ vẫn hiện nút dừng nhưng khóa lại, kèm lời giải thích —
    // ẩn hẳn đi sẽ khiến người dùng tưởng panel bị lỗi.
    options.push({ label: t('services.stop'), key: 'stop', disabled: row.protected })
  } else {
    options.push({ label: t('services.start'), key: 'start' })
  }

  if (row.enabled) {
    options.push({ label: t('services.disable'), key: 'disable', disabled: row.protected })
  } else {
    options.push({ label: t('services.enable'), key: 'enable' })
  }

  return options
}

const columns = computed<DataTableColumns<SystemService>>(() => [
  {
    title: t('services.name'),
    key: 'name',
    render: (row) =>
      h(NSpace, { size: 6, align: 'center' }, {
        default: () => [
          h(NText, { strong: true }, { default: () => row.name }),
          // Nhãn kèm giải thích: người dùng thấy mục "Dừng" bị khóa cần biết ngay
          // vì sao, nếu không họ sẽ tưởng panel bị lỗi.
          row.protected
            ? h(NTooltip, null, {
                trigger: () =>
                  h(NTag, { size: 'tiny', type: 'info', bordered: false },
                    { default: () => t('services.protected') }),
                default: () => t('services.protectedHint'),
              })
            : null,
        ],
      }),
  },
  {
    title: t('services.description'),
    key: 'description',
    ellipsis: { tooltip: true },
    render: (row) => row.description || '—',
  },
  {
    title: t('services.state'),
    key: 'state',
    width: 130,
    render: (row) =>
      h(NTag, { size: 'small', type: stateTag[row.state] },
        { default: () => t(`services.${stateLabel[row.state]}`) }),
  },
  {
    title: t('services.autostart'),
    key: 'enabled',
    width: 120,
    render: (row) =>
      h(NText, { depth: row.enabled ? 1 : 3 },
        { default: () => (row.enabled ? t('common.enabled') : t('common.disabled')) }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 190,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => openLogs(row.name) },
            { default: () => t('services.logs') }),
          auth.canWrite
            ? h(NDropdown,
                {
                  trigger: 'click',
                  options: actionsFor(row),
                  onSelect: (key: ServiceAction) => control(row.name, key),
                },
                { default: () => h(NButton, { size: 'tiny' }, { default: () => '⋯' }) })
            : null,
        ],
      }),
  },
])
</script>

<template>
  <!-- Thẻ không đặt tiêu đề: thanh tiêu đề của panel đã nói đây là trang nào,
       nhắc lại lần nữa chỉ tốn một dòng ngay chỗ cần cho nội dung. -->
  <NCard size="small">
    <!-- Naive UI bỏ luôn thanh tiêu đề khi thẻ không có tiêu đề, kéo theo cả
         nhóm nút bên phải. Khe tiêu đề rỗng giữ thanh đó lại cho nhóm nút. -->
    <template #header><span /></template>

    <template #header-extra>
      <NSpace align="center" :size="10">
        <NSpace align="center" :size="6">
          <NSwitch v-model:value="onlyRunning" size="small" />
          <NText depth="3" style="font-size: 12px">{{ t('services.onlyRunning') }}</NText>
        </NSpace>
        <NInput
          v-model:value="search"
          size="small"
          clearable
          :placeholder="t('services.search')"
          style="width: 220px"
        />
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
      </NSpace>
    </template>

    <NAlert v-if="!managerAvailable" type="warning" :bordered="false">
      {{ t('services.managerUnavailable', { manager: managerName }) }}
    </NAlert>

    <NDataTable
      v-else
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :row-key="(row: SystemService) => row.name"
      size="small"
      max-height="calc(100vh - 280px)"
      virtual-scroll
    />
  </NCard>

  <NModal
    v-model:show="logs.show"
    preset="card"
    :title="t('services.logsOf', { name: logs.name })"
    style="width: 92vw; max-width: 1100px"
  >
    <pre class="logs">{{ logs.loading ? t('common.loading') : logs.content }}</pre>
  </NModal>
</template>

<style scoped>
.logs {
  max-height: 65vh;
  margin: 0;
  padding: 12px;
  overflow: auto;
  font-family: 'Cascadia Mono', Consolas, Menlo, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--sp-bg);
  border-radius: 6px;
}
</style>
