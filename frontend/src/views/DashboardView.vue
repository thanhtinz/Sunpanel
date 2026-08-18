<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NCard,
  NEmpty,
  NGi,
  NGrid,
  NRadioButton,
  NRadioGroup,
  NSpace,
  NSpin,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { monitorApi, type HistorySample, type SystemInfo } from '@/api'
import { useMonitorStream } from '@/composables/useMonitorStream'
import StatCard from '@/components/StatCard.vue'
import LineChart, { type Series } from '@/components/LineChart.vue'
import { palette } from '@/styles/theme'
import { useThemeStore } from '@/stores/theme'
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
  if (!snap?.swapTotal) return t('dashboard.swapOff')
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

/** Chuỗi xu hướng cho các thẻ chỉ số, lấy từ chính bộ đệm của luồng realtime. */
const cpuTrend = computed(() => buffer.value.map((s) => s.cpu))
const memoryTrend = computed(() => buffer.value.map((s) => s.memory))
const diskTrend = computed(() => buffer.value.map((s) => s.disk))
const swapTrend = computed(() => buffer.value.map((s) => s.swap))

/**
 * Biểu đồ đọc màu từ chính bảng màu của panel để đường vẽ không lệch tông với
 * phần còn lại của giao diện — và đổi theo chế độ tối, vì màu sáng dùng cho nền
 * trắng bị chìm hẳn trên nền đen.
 */
const theme = useThemeStore()
const chartColors = computed(() => (theme.isDark ? palette.dark : palette.light))

/** Nhãn trục thời gian của biểu đồ realtime. */
const liveLabels = computed(() => buffer.value.map((s) => formatClock(s.time)))

const liveCpuSeries = computed<Series[]>(() => [
  { name: t('dashboard.cpu'), data: buffer.value.map((s) => s.cpu), color: chartColors.value.action },
  { name: t('dashboard.memory'), data: buffer.value.map((s) => s.memory), color: chartColors.value.info },
])

const liveNetSeries = computed<Series[]>(() => [
  { name: t('dashboard.download'), data: buffer.value.map((s) => s.netRecv), color: chartColors.value.info },
  { name: t('dashboard.upload'), data: buffer.value.map((s) => s.netSent), color: chartColors.value.sun },
])

const historyLabels = computed(() => history.value.map((s) => formatClock(s.time)))

const historySeries = computed<Series[]>(() => [
  { name: t('dashboard.cpu'), data: history.value.map((s) => s.cpu), color: chartColors.value.action },
  { name: t('dashboard.memory'), data: history.value.map((s) => s.memory), color: chartColors.value.info },
  { name: t('dashboard.disk'), data: history.value.map((s) => s.disk), color: chartColors.value.sun },
])

/** Thông tin máy: cặp nhãn–giá trị, dựng phẳng để bố cục lưới điều khiển bề rộng. */
const facts = computed(() => [
  { key: 'hostname', label: t('dashboard.hostname'), value: system.value?.hostname ?? '—' },
  { key: 'platform', label: t('dashboard.platform'), value: system.value?.platform ?? '—' },
  { key: 'kernel', label: t('dashboard.kernel'), value: system.value?.kernel ?? '—' },
  { key: 'arch', label: t('dashboard.arch'), value: system.value?.arch ?? '—' },
  {
    key: 'cpuModel',
    label: t('dashboard.cpuModel'),
    value: system.value?.cpuModel
      ? `${system.value.cpuModel} (${t('dashboard.cores', { count: system.value.cpuCores })})`
      : '—',
  },
  { key: 'uptime', label: t('dashboard.uptime'), value: uptime.value },
])

function toneOf(value: number): string {
  if (value >= 90) return 'var(--sp-danger)'
  if (value >= 75) return 'var(--sp-warn)'
  return 'var(--sp-action)'
}
</script>

<template>
  <NSpace vertical :size="16">
    <NGrid :cols="'1 600:2 1100:4'" :x-gap="16" :y-gap="16">
      <NGi>
        <StatCard
          :label="t('dashboard.cpu')"
          :percent="latest?.cpu ?? 0"
          :detail="cpuDetail"
          :series="cpuTrend"
        />
      </NGi>
      <NGi>
        <StatCard
          :label="t('dashboard.memory')"
          :percent="latest?.memory ?? 0"
          :detail="memoryDetail"
          :series="memoryTrend"
        />
      </NGi>
      <NGi>
        <StatCard
          :label="t('dashboard.disk')"
          :percent="latest?.disk ?? 0"
          :detail="diskDetail"
          :series="diskTrend"
        />
      </NGi>
      <NGi>
        <StatCard
          :label="t('dashboard.swap')"
          :percent="latest?.swap ?? 0"
          :detail="swapDetail"
          :series="swapTrend"
        />
      </NGi>
    </NGrid>

    <NGrid :cols="'1 1100:2'" :x-gap="16" :y-gap="16">
      <NGi>
        <NCard :title="`${t('dashboard.cpu')} / ${t('dashboard.memory')}`" size="small">
          <template #header-extra>
            <span class="live" :class="{ 'live-on': connected }">
              <span class="live-dot"></span>
              {{ connected ? t('dashboard.realtime') : t('dashboard.disconnected') }}
            </span>
          </template>
          <LineChart :labels="liveLabels" :series="liveCpuSeries" unit="%" :max="100" />
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('dashboard.network')" size="small">
          <template #header-extra>
            <span class="live" :class="{ 'live-on': connected }">
              <span class="live-dot"></span>
              {{ connected ? t('dashboard.realtime') : t('dashboard.disconnected') }}
            </span>
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
          <dl class="facts">
            <template v-for="fact in facts" :key="fact.key">
              <dt class="sp-eyebrow">{{ fact.label }}</dt>
              <dd class="fact-value">{{ fact.value }}</dd>
            </template>
          </dl>
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('dashboard.disk')" size="small">
          <div class="mounts">
            <div v-for="disk in latest?.disks ?? []" :key="disk.mountpoint" class="mount">
              <div class="mount-head">
                <span class="mount-path">{{ disk.mountpoint }}</span>
                <span class="mount-usage sp-metric">
                  {{ t('dashboard.usedOf', {
                    used: formatBytes(disk.used),
                    total: formatBytes(disk.total),
                  }) }}
                  <span class="mount-percent" :style="{ color: toneOf(disk.percent) }">
                    {{ disk.percent.toFixed(0) }}%
                  </span>
                </span>
              </div>
              <div class="mount-track">
                <div
                  class="mount-fill"
                  :style="{ width: `${Math.min(100, disk.percent)}%`, background: toneOf(disk.percent) }"
                ></div>
              </div>
            </div>

            <NEmpty v-if="!latest?.disks?.length" :description="t('common.loading')" />
          </div>
        </NCard>
      </NGi>
    </NGrid>
  </NSpace>
</template>

<style scoped>
/* Nhãn "đang chạy" của biểu đồ realtime: chấm đứng yên khi mất kết nối, để số
   liệu đóng băng không trông giống số liệu đang chảy. */
.live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--sp-text-faint);
}

.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--sp-text-faint);
}

.live-on .live-dot {
  background: var(--sp-action);
  animation: live-pulse 2.4s ease-in-out infinite;
}

@keyframes live-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

/* Nhãn nằm trên, giá trị nằm dưới: tên máy và mẫu CPU dài hơn nhiều so với chỗ
   một cột nhãn bên trái chừa lại, và cắt bớt chúng thì mất đúng phần phân biệt. */
.facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 20px;
  margin: 0;
}

.facts dt {
  margin-bottom: 3px;
}

.fact-value {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--sp-text);
  overflow-wrap: anywhere;
}

.mounts {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.mount-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 6px;
}

.mount-path {
  overflow: hidden;
  font-size: 13px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mount-usage {
  flex: none;
  font-size: 12px;
  color: var(--sp-text-muted);
}

.mount-percent {
  margin-left: 6px;
  font-weight: 600;
}

.mount-track {
  height: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--sp-surface-sunken);
}

.mount-fill {
  height: 100%;
  transition: width 0.4s ease, background 0.4s ease;
}

@media (max-width: 600px) {
  .facts {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
