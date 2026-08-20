<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NGrid,
  NGridItem,
  NPopconfirm,
  NSpace,
  NStatistic,
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  securityApi,
  type LoginBlock,
  type LoginOffender,
  type SecurityOverview,
} from '@/api'
import { translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const message = useMessage()

const data = ref<SecurityOverview | null>(null)
const loading = ref(false)

/**
 * Chu kỳ làm mới.
 *
 * Lệnh chặn tự hết hạn theo đồng hồ máy chủ, nên một trang đứng yên sẽ hiển thị
 * những địa chỉ đã được thả từ lâu.
 */
const refreshInterval = 15000
let timer: number | undefined

onMounted(async () => {
  await load()
  timer = window.setInterval(() => void load(true), refreshInterval)
})

onUnmounted(() => window.clearInterval(timer))

async function load(silent = false): Promise<void> {
  if (!silent) loading.value = true
  try {
    data.value = await securityApi.overview()
  } catch (err) {
    message.error(
      err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'),
    )
  } finally {
    loading.value = false
  }
}

async function unblock(ip: string): Promise<void> {
  try {
    await securityApi.unblock(ip)
    message.success(t('security.unblocked', { ip }))
    await load(true)
  } catch (err) {
    message.error(
      err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'),
    )
  }
}

/** Đổi số giây sang cách nói của con người: "10 phút" dễ đọc hơn "600 giây". */
function minutes(seconds: number): string {
  if (seconds < 60) return t('security.seconds', { n: seconds })
  return t('security.minutes', { n: Math.round(seconds / 60) })
}

/** Còn bao lâu nữa hết chặn, tính từ lúc vẽ. */
function remaining(until: string): string {
  const left = Math.round((new Date(until).getTime() - Date.now()) / 60000)
  return t('security.minutes', { n: Math.max(1, left) })
}

const blockColumns = computed<DataTableColumns<LoginBlock>>(() => [
  {
    title: t('security.ip'),
    key: 'ip',
    render: (row) => h(NText, { strong: true, style: 'font-family: var(--sp-font-mono)' }, { default: () => row.ip }),
  },
  { title: t('security.failures'), key: 'failures', width: 110 },
  {
    title: t('security.lastUser'),
    key: 'lastUser',
    width: 160,
    render: (row) => row.lastUser || '—',
  },
  {
    title: t('security.blockedAt'),
    key: 'blockedAt',
    width: 190,
    render: (row) => h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => formatDateTime(row.blockedAt) }),
  },
  {
    title: t('security.until'),
    key: 'until',
    width: 150,
    render: (row) => remaining(row.until),
  },
  {
    title: '',
    key: 'actions',
    width: 110,
    render: (row) =>
      h(NPopconfirm, { onPositiveClick: () => unblock(row.ip) }, {
        trigger: () =>
          h(NButton, { size: 'tiny', quaternary: true }, { default: () => t('security.unblock') }),
        default: () => t('security.unblockConfirm', { ip: row.ip }),
      }),
  },
])

const offenderColumns = computed<DataTableColumns<LoginOffender>>(() => [
  {
    title: t('security.ip'),
    key: 'ip',
    render: (row) =>
      h(NSpace, { size: 6, align: 'center', wrap: false }, {
        default: () => [
          h(NText, { style: 'font-family: var(--sp-font-mono)' }, { default: () => row.ip }),
          row.blocked
            ? h(NTag, { size: 'tiny', type: 'error', bordered: false }, { default: () => t('security.blocked') })
            : null,
        ],
      }),
  },
  { title: t('security.failures'), key: 'failures', width: 110 },
  {
    title: t('security.lastUser'),
    key: 'lastUser',
    width: 180,
    render: (row) => row.lastUser || '—',
  },
  {
    title: t('security.lastAt'),
    key: 'lastAt',
    width: 190,
    render: (row) => h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => formatDateTime(row.lastAt) }),
  },
])
</script>

<template>
  <NSpace vertical :size="16">
    <NAlert v-if="data && !data.enabled" type="warning" :bordered="false">
      {{ t('security.disabled') }}
    </NAlert>

    <NGrid v-if="data" :x-gap="16" :y-gap="16" cols="1 s:3" responsive="screen">
      <NGridItem>
        <NCard size="small" style="height: 100%">
          <NStatistic :label="t('security.failedLastDay')" :value="data.failedLastDay" />
        </NCard>
      </NGridItem>
      <NGridItem>
        <NCard size="small" style="height: 100%">
          <NStatistic :label="t('security.blockedNow')" :value="data.blocks.length" />
        </NCard>
      </NGridItem>
      <NGridItem>
        <NCard size="small" style="height: 100%">
          <NStatistic :label="t('security.rule')">
            <NText style="font-size: 18px">
              {{ t('security.ruleValue', { n: data.threshold, window: minutes(data.windowSeconds) }) }}
            </NText>
          </NStatistic>
          <NText depth="3" style="font-size: 12px">
            {{ t('security.ruleHint', { duration: minutes(data.durationSeconds) }) }}
          </NText>
        </NCard>
      </NGridItem>
    </NGrid>

    <NCard size="small" :title="t('security.blocks')">
      <template #header-extra>
        <NButton size="small" :loading="loading" @click="load()">{{ t('common.refresh') }}</NButton>
      </template>

      <NDataTable
        v-if="data && data.blocks.length"
        :columns="blockColumns"
        :data="data.blocks"
        :row-key="(row: LoginBlock) => row.ip"
        size="small"
      />
      <NEmpty v-else :description="t('security.noBlocks')" />
    </NCard>

    <NCard size="small" :title="t('security.offenders')">
      <NDataTable
        v-if="data && data.offenders.length"
        :columns="offenderColumns"
        :data="data.offenders"
        :row-key="(row: LoginOffender) => row.ip"
        size="small"
      />
      <NEmpty v-else :description="t('security.noOffenders')" />
    </NCard>
  </NSpace>
</template>
