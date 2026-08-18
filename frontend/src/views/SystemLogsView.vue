<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NInput,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, logApi, type LogSource } from '@/api'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const message = useMessage()

const sources = ref<LogSource[]>([])
const current = ref<string | null>(null)
const lines = ref<string[]>([])
const offset = ref(0)
const loading = ref(false)
const follow = ref(true)
const keyword = ref('')
const pane = ref<HTMLElement | null>(null)

/**
 * Chu kỳ đọc thêm khi đang theo dõi.
 *
 * Mỗi lần đọc chỉ lấy phần mới thêm vào tệp kể từ vị trí lần trước, nên hai
 * giây là rẻ kể cả với tệp nhật ký hàng trăm MB.
 */
const followInterval = 2000
let timer: number | undefined

// Số dòng giữ lại trong khung xem.
//
// Một tệp nhật ký đang chạy có thể sinh vài nghìn dòng mỗi phút; giữ hết trong
// DOM là cách chắc chắn để tab trình duyệt đứng hình sau mười phút theo dõi.
const maxLines = 5000

const lineCount = computed(() => lines.value.length)

const shown = computed(() => {
  const needle = keyword.value.trim().toLowerCase()
  if (!needle) return lines.value
  return lines.value.filter((line) => line.toLowerCase().includes(needle))
})

const options = computed(() =>
  sources.value.map((source) => ({
    label: `${source.name} · ${formatBytes(source.size)}`,
    value: source.path,
  })),
)

const currentSource = computed(() => sources.value.find((item) => item.path === current.value))

onMounted(async () => {
  await loadSources()
  const first = sources.value[0]
  if (first) current.value = first.path
})

onUnmounted(stop)

watch(current, () => void open())
watch(follow, (value) => (value ? start() : stop()))

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function loadSources(): Promise<void> {
  try {
    sources.value = await logApi.sources()
  } catch (err) {
    report(err)
  }
}

/** Mở một tệp: đọc phần cuối rồi bật lại vòng theo dõi. */
async function open(): Promise<void> {
  stop()
  lines.value = []
  offset.value = 0
  if (!current.value) return

  loading.value = true
  try {
    const chunk = await logApi.tail(current.value)
    lines.value = chunk.lines
    offset.value = chunk.offset
    await scrollToEnd()
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
  if (follow.value) start()
}

function start(): void {
  stop()
  if (!current.value) return
  timer = window.setInterval(() => void poll(), followInterval)
}

function stop(): void {
  window.clearInterval(timer)
  timer = undefined
}

async function poll(): Promise<void> {
  if (!current.value) return

  try {
    const chunk = await logApi.since(current.value, offset.value)
    offset.value = chunk.offset

    // logrotate vừa thay tệp: nối tiếp vào phần cũ sẽ ghép nhật ký của hai tệp
    // khác nhau thành một dòng thời gian không có thật.
    if (chunk.truncated) {
      lines.value = chunk.lines
      message.info(t('systemLogs.rotated'))
    } else if (chunk.lines.length) {
      lines.value = lines.value.concat(chunk.lines).slice(-maxLines)
    } else {
      return
    }
    await scrollToEnd()
  } catch (err) {
    stop()
    follow.value = false
    report(err)
  }
}

/** Cuộn xuống cuối, nhưng chỉ khi người dùng đang ở cuối sẵn. */
async function scrollToEnd(): Promise<void> {
  const element = pane.value
  if (!element) return

  const atBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 80
  await nextTick()
  if (atBottom || !follow.value) element.scrollTop = element.scrollHeight
}
</script>

<template>
  <NCard size="small">
    <template #header><span /></template>

    <template #header-extra>
      <NSpace align="center" :size="10">
        <NSpace align="center" :size="6">
          <NSwitch v-model:value="follow" size="small" />
          <NText depth="3" style="font-size: 12px">{{ t('systemLogs.follow') }}</NText>
        </NSpace>
        <NInput
          v-model:value="keyword"
          size="small"
          clearable
          :placeholder="t('systemLogs.filter')"
          style="width: 200px"
        />
        <NButton size="small" :loading="loading" @click="open">{{ t('common.refresh') }}</NButton>
      </NSpace>
    </template>

    <NSpace vertical :size="12">
      <NSpace align="center" :size="10">
        <NSelect
          v-model:value="current"
          :options="options"
          filterable
          :placeholder="t('systemLogs.pick')"
          style="width: 340px"
        />
        <NTag v-if="currentSource" size="small" :bordered="false">
          {{ t('systemLogs.updated', { time: formatDateTime(new Date(currentSource.modifiedAt).toISOString()) }) }}
        </NTag>
        <NText depth="3" style="font-size: 12px">
          {{ t('systemLogs.lineCount', { count: lineCount }) }}
        </NText>
      </NSpace>

      <NAlert v-if="!sources.length" type="info" :bordered="false">
        {{ t('systemLogs.empty') }}
      </NAlert>

      <div v-else ref="pane" class="pane">
        <NEmpty v-if="!shown.length" :description="t('systemLogs.noLines')" style="padding: 40px 0" />
        <pre v-else class="lines">{{ shown.join('\n') }}</pre>
      </div>
    </NSpace>
  </NCard>
</template>

<style scoped>
.pane {
  height: calc(100vh - 300px);
  min-height: 320px;
  overflow: auto;
  border: 1px solid var(--sp-border);
  border-radius: var(--sp-radius-sm);
  background: var(--sp-surface-sunken);
}

.lines {
  margin: 0;
  padding: 12px 14px;
  font-family: var(--sp-font-mono);
  font-size: 12px;
  line-height: 1.65;
  color: var(--sp-text);
  white-space: pre;
}
</style>
