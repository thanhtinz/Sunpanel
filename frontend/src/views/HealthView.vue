<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NProgress,
  NSpace,
  NSpin,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, healthApi, type HealthItem, type HealthReport } from '@/api'
import { translateError } from '@/locales'

const { t } = useI18n()
const message = useMessage()
const router = useRouter()

const report = ref<HealthReport | null>(null)
const loading = ref(false)

onMounted(load)

async function load(): Promise<void> {
  loading.value = true
  try {
    report.value = await healthApi.report()
  } catch (err) {
    message.error(
      err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'),
    )
  } finally {
    loading.value = false
  }
}

/**
 * Màu của vòng điểm.
 *
 * Ngưỡng đặt theo mức nặng chứ không theo con số tròn: một mục nghiêm trọng trừ
 * 18 điểm, nên dưới 80 nghĩa là chắc chắn đang có thứ hỏng.
 */
const scoreStatus = computed(() => {
  const score = report.value?.score ?? 100
  if (score < 80) return 'error'
  if (score < 95) return 'warning'
  return 'success'
})

const tone: Record<string, 'success' | 'warning' | 'error'> = {
  ok: 'success',
  warn: 'warning',
  critical: 'error',
}

/** Các mục chia theo lĩnh vực, giữ nguyên thứ tự nặng trước. */
const groups = computed(() => {
  const order = ['resource', 'panel', 'web', 'data']
  const items = report.value?.items ?? []
  return order
    .map((group) => ({ group, items: items.filter((item) => item.group === group) }))
    .filter((entry) => entry.items.length > 0)
})

function describe(item: HealthItem): string {
  return t(`health.detail.${item.detail}`, item.params ?? {})
}

function open(item: HealthItem): void {
  if (item.route) void router.push({ name: item.route })
}
</script>

<template>
  <NSpin :show="loading">
    <NSpace vertical :size="16">
      <NCard size="small">
        <template #header><span /></template>
        <template #header-extra>
          <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        </template>

        <NSpace align="center" :size="24" :wrap="false" class="score-row">
          <!-- Vòng tròn có status khác mặc định sẽ tự vẽ một biểu tượng thay cho
               con số; ở đây con số mới là thứ người dùng cần đọc. -->
          <NProgress
            type="circle"
            :percentage="report?.score ?? 0"
            :status="scoreStatus"
            :stroke-width="8"
            style="width: 108px"
          >
            <NText strong style="font-size: 22px">{{ report?.score ?? 0 }}</NText>
          </NProgress>
          <NSpace vertical :size="6">
            <NText strong style="font-size: 16px">{{ t('health.title') }}</NText>
            <NText depth="3" style="font-size: 13px">
              {{
                report && report.criticals + report.warnings === 0
                  ? t('health.allGood')
                  : t('health.summary', {
                      criticals: report?.criticals ?? 0,
                      warnings: report?.warnings ?? 0,
                    })
              }}
            </NText>
          </NSpace>
        </NSpace>
      </NCard>

      <NCard
        v-for="entry in groups"
        :key="entry.group"
        size="small"
        :title="t(`health.group.${entry.group}`)"
      >
        <div v-for="item in entry.items" :key="item.key" class="health-row">
          <NSpace align="center" :size="10" :wrap="false">
            <NTag :type="tone[item.level]" size="small" :bordered="false" class="health-level">
              {{ t(`health.level.${item.level}`) }}
            </NTag>
            <NSpace vertical :size="2">
              <NText strong>{{ t(`health.item.${item.key}`) }}</NText>
              <NText depth="3" style="font-size: 12px">{{ describe(item) }}</NText>
            </NSpace>
          </NSpace>

          <NButton
            v-if="item.route && item.level !== 'ok'"
            size="tiny"
            quaternary
            @click="open(item)"
          >
            {{ t('health.fix') }}
          </NButton>
        </div>
      </NCard>
    </NSpace>
  </NSpin>
</template>

<style scoped>
.score-row {
  padding: 6px 4px;
}

.health-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--sp-border);
}

.health-row:last-child {
  border-bottom: none;
}

.health-level {
  min-width: 84px;
  justify-content: center;
}
</style>
