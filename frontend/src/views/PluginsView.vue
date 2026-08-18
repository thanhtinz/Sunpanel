<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NGi,
  NGrid,
  NSpace,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, pluginApi, type PluginManifest } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'

const { t, locale } = useI18n()
const route = useRoute()
const auth = useAuthStore()
const message = useMessage()

const plugins = ref<PluginManifest[]>([])
const dir = ref('')
const loading = ref(false)

/** Plugin đang mở, lấy từ tham số đường dẫn. */
const activeKey = computed(() => (route.params.key as string | undefined) ?? '')
const active = computed(() => plugins.value.find((p) => p.key === activeKey.value) ?? null)

/** Chuỗi trong tệp khai báo nằm phía máy chủ nên mang sẵn cả hai bản dịch. */
function localized(text?: { vi: string; en: string }): string {
  if (!text) return ''
  return locale.value === 'vi' ? text.vi : text.en
}

/** Vé mở khung nhúng, xin lại mỗi khi đổi plugin. */
const ticket = ref('')

const frameUrl = computed(() =>
  active.value && ticket.value
    ? pluginApi.uiUrl(active.value.key, active.value.uiPath, ticket.value)
    : '',
)

onMounted(async () => {
  await load()
  await requestTicket()
})

watch(activeKey, async () => {
  ticket.value = ''
  if (plugins.value.length === 0) await load()
  await requestTicket()
})

async function requestTicket(): Promise<void> {
  if (!activeKey.value) return
  try {
    ticket.value = (await pluginApi.ticket(activeKey.value)).ticket
  } catch (err) {
    report(err)
  }
}

function report(err: unknown): void {
  if (err instanceof ApiError && err.code.startsWith('plugin.')) {
    message.error(t(err.code.replace('plugin.', 'pluginErr.'), err.params ?? {}))
    return
  }
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const result = auth.isAdmin ? await pluginApi.all() : await pluginApi.list()
    plugins.value = result.plugins ?? []
    dir.value = result.dir
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

async function reload(): Promise<void> {
  try {
    await pluginApi.reload()
    message.success(t('plugin.reloaded'))
    await load()
  } catch (err) {
    report(err)
  }
}
</script>

<template>
  <!-- Đang mở một plugin cụ thể: nhúng giao diện của nó qua đường proxy. -->
  <NCard
    v-if="active"
    :title="localized(active.name)"
    size="small"
    class="plugin-frame-card"
  >
    <template #header-extra>
      <NSpace :size="8" align="center">
        <NText depth="3" style="font-size: 12px">v{{ active.version }}</NText>
        <NButton size="small" tag="a" :href="frameUrl" target="_blank">
          {{ t('plugin.openInTab') }}
        </NButton>
      </NSpace>
    </template>

    <!-- Plugin chạy ở tiến trình riêng nên giao diện của nó được nhúng trong
         khung cách ly, không chạy chung ngữ cảnh với panel. -->
    <iframe
      v-if="frameUrl"
      :src="frameUrl"
      class="plugin-frame"
      sandbox="allow-scripts allow-forms allow-same-origin allow-popups"
      :title="localized(active.name)"
    ></iframe>
  </NCard>

  <NCard v-else :title="t('plugin.title')" size="small">
    <template #header-extra>
      <NSpace :size="8">
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        <NButton v-if="auth.isAdmin" size="small" type="primary" @click="reload">
          {{ t('plugin.reload') }}
        </NButton>
      </NSpace>
    </template>

    <NAlert type="info" :bordered="false" style="margin-bottom: 14px">
      {{ t('plugin.hint', { dir }) }}
    </NAlert>

    <NGrid v-if="plugins.length > 0" cols="1 700:2 1100:3" :x-gap="14" :y-gap="14">
      <NGi v-for="item in plugins" :key="item.key">
        <NCard size="small" class="plugin-card">
          <div class="plugin-head">
            <div class="plugin-title">{{ localized(item.name) }}</div>
            <NTag
              size="tiny"
              :type="item.enabled ? 'success' : 'default'"
              :bordered="false"
            >
              {{ item.enabled ? t('common.enabled') : t('common.disabled') }}
            </NTag>
          </div>

          <p class="plugin-desc">{{ localized(item.description) }}</p>

          <div class="plugin-foot">
            <NText depth="3" style="font-size: 12px">
              v{{ item.version }}<template v-if="item.author"> · {{ item.author }}</template>
            </NText>
            <RouterLink
              v-if="item.enabled && item.uiPath"
              v-slot="{ navigate }"
              :to="{ name: 'plugin-detail', params: { key: item.key } }"
              custom
            >
              <NButton size="tiny" type="primary" @click="navigate">
                {{ t('plugin.open') }}
              </NButton>
            </RouterLink>
          </div>
        </NCard>
      </NGi>
    </NGrid>

    <NEmpty v-else :description="t('plugin.none')" />
  </NCard>
</template>

<style scoped>
.plugin-frame-card :deep(.n-card__content) {
  padding: 0;
}

.plugin-frame {
  display: block;
  width: 100%;
  /* Trừ đi chiều cao thanh tiêu đề panel và phần đệm để khung vừa màn hình. */
  height: calc(100vh - 190px);
  min-height: 400px;
  border: 0;
}

.plugin-card {
  height: 100%;
}

.plugin-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.plugin-title {
  font-weight: 600;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-desc {
  margin: 8px 0 14px;
  min-height: 40px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--sp-text-muted);
}

.plugin-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
</style>
