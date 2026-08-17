<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NEmpty,
  NGi,
  NGrid,
  NRadioButton,
  NRadioGroup,
  NProgress,
  NSpace,
  NSpin,
  NTag,
  NText,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { monitorApi, type HistorySample, type SystemInfo } from '@/api'
import { useMonitorStream } from '@/composables/useMonitorStream'
import StatCard from '@/components/StatCard.vue'
import LineChart, { type Series } from '@/components/LineChart.vue'
import {
  formatBytes,
  formatClock,
  formatRate,
  formatUptime,
} from '@/utils/format'

const { t } = useI18n()
const { connected, latest, buffer } = useMonitorStream()

const system = ref<SystemInfo | null>(null)
const history = ref<HistorySample[]>([])
const historyWindow = ref('1h')
const loadingHistory = ref(false)

const windowOptions = ['1h', '6h', '24h', '7d'] as const

onMounted(async () => {
  const overview = await monitorApi.overview().catch(() => null)
  if (overview) system.value = overview.system
  await loadHistory()
})

async function loadHistory(): Promise<void> {
  loadingHistory.value = true
  try {
    history.value = await monitorApi.history(historyWindow.value)
  } catch {
    history.value = []
  } finally {
    loadingHistory.value = false
  }
}

watch(historyWindow, loadHistory)

const uptime = computed(() => {
  if (!system.value?.bootTime) return '—'
  return formatUptime(Date.now() / 1000 - system.value.bootTime)
})

const memoryDetail = computed(() => {
  const snap = latest.value
  if (!snap?.memoryTotal) return undefined
  return t('dashboard.usedOf', {
    used: formatBytes(snap.memoryUsed),
    total: formatBytes(snap.memoryTotal),
  })
})

const swapDetail = computed(() => {
  const snap = latest.value
  if (!snap?.swapTotal) return undefined
  return t('dashboard.usedOf', {
    used: formatBytes(snap.swapUsed),
    total: formatBytes(snap.swapTotal),
  })
})

const rootDisk = computed(() => latest.value?.disks?.[0])

const diskDetail = computed(() => {
  const disk = rootDisk.value
  if (!disk) return undefined
  return t('dashboard.usedOf', { used: formatBytes(disk.used), total: formatBytes(disk.total) })
})

const cpuDetail = computed(() => {
  const snap = latest.value
  if (!snap) return undefined
  return `${t('dashboard.load')} ${snap.load1.toFixed(2)} / ${snap.load5.toFixed(2)} / ${snap.load15.toFixed(2)}`
})

/** Nhãn trục thời gian của biểu đồ realtime. */
const liveLabels = computed(() => buffer.value.map((s) => formatClock(s.time)))

const liveCpuSeries = computed<Series[]>(() => [
  { name: t('dashboard.cpu'), data: buffer.value.map((s) => s.cpu), color: '#18a058' },
  { name: t('dashboard.memory'), data: buffer.value.map((s) => s.memory), color: '#2080f0' },
])

const liveNetSeries = computed<Series[]>(() => [
  { name: t('dashboard.download'), data: buffer.value.map((s) => s.netRecv), color: '#2080f0' },
  { name: t('dashboard.upload'), data: buffer.value.map((s) => s.netSent), color: '#f0a020' },
])

const historyLabels = computed(() => history.value.map((s) => formatClock(s.time)))

const historySeries = computed<Series[]>(() => [
  { name: t('dashboard.cpu'), data: history.value.map((s) => s.cpu), color: '#18a058' },
  { name: t('dashboard.memory'), data: history.value.map((s) => s.memory), color: '#2080f0' },
  { name: t('dashboard.disk'), data: history.value.map((s) => s.disk), color: '#f0a020' },
])
</script>

<template>
  <NSpace vertical :size="16">
    <NSpace align="center" justify="space-between">
      <NText strong style="font-size: 18px">{{ t('dashboard.title') }}</NText>
      <NTag :type="connected ? 'success' : 'warning'" size="small" round>
        {{ connected ? t('dashboard.connected') : t('dashboard.disconnected') }}
      </NTag>
    </NSpace>

    <NGrid :cols="'1 600:2 1100:4'" :x-gap="16" :y-gap="16">
      <NGi>
        <StatCard :label="t('dashboard.cpu')" :percent="latest?.cpu ?? 0" :detail="cpuDetail" />
      </NGi>
      <NGi>
        <StatCard
          :label="t('dashboard.memory')"
          :percent="latest?.memory ?? 0"
          :detail="memoryDetail"
        />
      </NGi>
      <NGi>
        <StatCard :label="t('dashboard.disk')" :percent="latest?.disk ?? 0" :detail="diskDetail" />
      </NGi>
      <NGi>
        <StatCard :label="t('dashboard.swap')" :percent="latest?.swap ?? 0" :detail="swapDetail" />
      </NGi>
    </NGrid>

    <NGrid :cols="'1 1100:2'" :x-gap="16" :y-gap="16">
      <NGi>
        <NCard :title="`${t('dashboard.cpu')} / ${t('dashboard.memory')}`" size="small">
          <template #header-extra>
            <NText depth="3" style="font-size: 12px">{{ t('dashboard.realtime') }}</NText>
          </template>
          <LineChart :labels="liveLabels" :series="liveCpuSeries" unit="%" :max="100" />
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('dashboard.network')" size="small">
          <template #header-extra>
            <NText depth="3" style="font-size: 12px">{{ t('dashboard.realtime') }}</NText>
          </template>
          <LineChart :labels="liveLabels" :series="liveNetSeries" :formatter="formatRate" />
        </NCard>
      </NGi>
    </NGrid>

    <NCard :title="t('dashboard.history')" size="small">
      <template #header-extra>
        <NRadioGroup v-model:value="historyWindow" size="small">
          <NRadioButton v-for="w in windowOptions" :key="w" :value="w">
            {{ t(`dashboard.window.${w}`) }}
          </NRadioButton>
        </NRadioGroup>
      </template>

      <NSpin :show="loadingHistory">
        <LineChart
          v-if="history.length > 0"
          :labels="historyLabels"
          :series="historySeries"
          unit="%"
          :max="100"
          height="280px"
        />
        <NEmpty v-else :description="t('common.empty')" style="padding: 48px 0" />
      </NSpin>
    </NCard>

    <NGrid :cols="'1 1100:2'" :x-gap="16" :y-gap="16">
      <NGi>
        <NCard :title="t('dashboard.systemInfo')" size="small">
          <NDescriptions :column="1" label-placement="left" size="small">
            <NDescriptionsItem :label="t('dashboard.hostname')">
              {{ system?.hostname ?? '—' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('dashboard.platform')">
              {{ system?.platform ?? '—' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('dashboard.kernel')">
              {{ system?.kernel ?? '—' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('dashboard.cpuModel')">
              {{ system?.cpuModel ?? '—' }}
              <NText v-if="system?.cpuCores" depth="3">
                ({{ t('dashboard.cores', { count: system.cpuCores }) }})
              </NText>
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('dashboard.arch')">
              {{ system?.arch ?? '—' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('dashboard.uptime')">{{ uptime }}</NDescriptionsItem>
          </NDescriptions>
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('dashboard.disk')" size="small">
          <NSpace vertical :size="14">
            <div v-for="disk in latest?.disks ?? []" :key="disk.mountpoint">
              <NSpace justify="space-between" align="center" :size="8">
                <NText>{{ disk.mountpoint }}</NText>
                <NText depth="3" class="sp-metric" style="font-size: 12px">
                  {{ t('dashboard.usedOf', {
                    used: formatBytes(disk.used),
                    total: formatBytes(disk.total),
                  }) }}
                </NText>
              </NSpace>
              <NProgress
                type="line"
                :percentage="disk.percent"
                :height="6"
                :show-indicator="false"
                :status="disk.percent >= 90 ? 'error' : disk.percent >= 75 ? 'warning' : 'success'"
              />
            </div>
            <NEmpty v-if="!latest?.disks?.length" :description="t('common.loading')" />
          </NSpace>
        </NCard>
      </NGi>
    </NGrid>
  </NSpace>
</template>
