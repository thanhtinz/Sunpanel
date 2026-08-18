<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NGi,
  NGrid,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTabPane,
  NTabs,
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  appStoreApi,
  type CatalogApp,
  type ComposeStatus,
  type InstalledApp,
} from '@/api'
import AppIcon from '@/components/AppIcon.vue'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'

const { t, locale } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const status = ref<ComposeStatus>({ available: true, version: '' })
const catalog = ref<CatalogApp[]>([])
const categories = ref<string[]>([])
const installed = ref<InstalledApp[]>([])
const loading = ref(false)
const category = ref<string>('')
const search = ref('')

/** Chuỗi trong danh mục ứng dụng nằm ở phía máy chủ nên mang sẵn cả hai bản dịch. */
function localized(text?: { vi: string; en: string }): string {
  if (!text) return ''
  return locale.value === 'vi' ? text.vi : text.en
}

const installer = ref({
  show: false,
  saving: false,
  app: null as CatalogApp | null,
  name: '',
  values: {} as Record<string, string>,
})

const logs = ref({ show: false, name: '', content: '', loading: false })
const secrets = ref({ show: false, name: '', values: {} as Record<string, string> })

onMounted(load)

function report(err: unknown): void {
  if (err instanceof ApiError && err.code.startsWith('app.')) {
    message.error(t(err.code.replace('app.', 'appErr.'), err.params ?? {}))
    return
  }
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [composeStatus, catalogResult, list] = await Promise.all([
      appStoreApi.status(),
      appStoreApi.catalog(),
      appStoreApi.list(),
    ])
    status.value = composeStatus
    catalog.value = catalogResult.apps ?? []
    categories.value = catalogResult.categories ?? []
    installed.value = list
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/**
 * Lọc theo cả nhóm lẫn từ khóa.
 *
 * Từ khóa quét cả mô tả chứ không riêng tên: người dùng thường nhớ ứng dụng
 * dùng để làm gì ("sao lưu", "mật khẩu") hơn là nhớ đúng cái tên của nó.
 */
const visibleCatalog = computed(() => {
  const keyword = search.value.trim().toLowerCase()

  return catalog.value.filter((app) => {
    if (category.value && app.category !== category.value) return false
    if (!keyword) return true

    return [app.key, app.name.vi, app.name.en, app.description.vi, app.description.en]
      .join(' ')
      .toLowerCase()
      .includes(keyword)
  })
})

/** Các nhóm kèm số ứng dụng, để không ai bấm vào một nhóm rỗng. */
const categoryTabs = computed(() => [
  { key: '', label: t('apps.allCategories'), count: catalog.value.length },
  ...categories.value.map((name) => ({
    key: name,
    label: t(`apps.category.${name}`),
    count: catalog.value.filter((app) => app.category === name).length,
  })),
])

/** Số bản đã cài của mỗi ứng dụng, để thẻ trong danh mục nói được điều đó. */
const installCount = computed(() => {
  const counts: Record<string, number> = {}
  for (const app of installed.value) {
    counts[app.appKey] = (counts[app.appKey] ?? 0) + 1
  }
  return counts
})

function openInstaller(app: CatalogApp): void {
  const values: Record<string, string> = {}
  for (const field of app.fields ?? []) {
    values[field.key] = field.default ?? ''
  }
  // Tên gợi ý là định danh ứng dụng; lần cài thứ hai người dùng tự đổi.
  installer.value = { show: true, saving: false, app, name: app.key, values }
}

const canInstall = computed(() => {
  const form = installer.value
  if (!form.app || !/^[a-z0-9][a-z0-9_-]{0,39}$/.test(form.name)) return false
  return (form.app.fields ?? []).every(
    (field) => !field.required || field.generate || (form.values[field.key] ?? '').trim() !== '',
  )
})

async function install(): Promise<void> {
  const form = installer.value
  if (!form.app) return

  form.saving = true
  try {
    await appStoreApi.install({
      appKey: form.app.key,
      name: form.name.trim(),
      values: form.values,
    })
    message.success(t('apps.installed'))
    form.show = false
    await load()
  } catch (err) {
    report(err)
    // Cài hỏng vẫn có thể để lại bản ghi kèm lỗi, nên phải nạp lại danh sách.
    await load()
  } finally {
    form.saving = false
  }
}

async function control(app: InstalledApp, action: 'start' | 'stop' | 'restart'): Promise<void> {
  try {
    await appStoreApi.control(app.id, action)
    message.success(t('apps.actionDone'))
    await load()
  } catch (err) {
    report(err)
    await load()
  }
}

async function uninstall(app: InstalledApp, removeData: boolean): Promise<void> {
  try {
    await appStoreApi.uninstall(app.id, removeData)
    message.success(t('apps.uninstalled'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function showLogs(app: InstalledApp): Promise<void> {
  logs.value = { show: true, name: app.name, content: '', loading: true }
  try {
    logs.value.content = (await appStoreApi.logs(app.id)).logs
  } catch (err) {
    report(err)
    logs.value.show = false
  } finally {
    logs.value.loading = false
  }
}

async function showSecrets(app: InstalledApp): Promise<void> {
  try {
    const values = await appStoreApi.params(app.id)
    secrets.value = { show: true, name: app.name, values }
  } catch (err) {
    report(err)
  }
}

async function copy(value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value)
    message.success(t('common.copied'))
  } catch {
    message.error(t('apps.copyFailed'))
  }
}

/** Địa chỉ mở ứng dụng, dựng từ tên máy đang xem panel. */
function appUrl(app: InstalledApp): string {
  return `http://${window.location.hostname}:${app.port}`
}

const columns = computed<DataTableColumns<InstalledApp>>(() => [
  { title: t('apps.name'), key: 'name', width: 160 },
  {
    title: t('apps.app'),
    key: 'appKey',
    minWidth: 160,
    render: (row) => localized(row.app?.name) || row.appKey,
  },
  {
    title: t('apps.state'),
    key: 'state',
    width: 140,
    render: (row) => {
      if (row.lastError) {
        return h(NTag, { size: 'small', type: 'error', bordered: false }, { default: () => t('apps.failed') })
      }
      return h(
        NTag,
        { size: 'small', type: row.running ? 'success' : 'default', bordered: false },
        { default: () => (row.running ? t('apps.running') : row.state || t('apps.stopped')) },
      )
    },
  },
  {
    title: t('apps.address'),
    key: 'port',
    width: 170,
    render: (row) => {
      if (!row.port) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(
        'a',
        { href: appUrl(row), target: '_blank', rel: 'noreferrer noopener', class: 'app-link' },
        `:${row.port}`,
      )
    },
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 330,
    render: (row) => {
      if (!auth.canWrite) {
        return h(NText, { depth: 3 }, { default: () => t('apps.readonly') })
      }
      return h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'tiny',
                quaternary: true,
                onClick: () => control(row, row.running ? 'stop' : 'start'),
              },
              { default: () => (row.running ? t('apps.stop') : t('apps.start')) },
            ),
            h(
              NButton,
              { size: 'tiny', quaternary: true, onClick: () => control(row, 'restart') },
              { default: () => t('apps.restart') },
            ),
            h(
              NButton,
              { size: 'tiny', quaternary: true, onClick: () => showLogs(row) },
              { default: () => t('apps.logs') },
            ),
            h(
              NButton,
              { size: 'tiny', quaternary: true, onClick: () => showSecrets(row) },
              { default: () => t('apps.credentials') },
            ),
            h(
              NPopconfirm,
              { onPositiveClick: () => uninstall(row, uninstallData.value) },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      quaternary: true,
                      type: 'error',
                      onClick: () => (uninstallData.value = false),
                    },
                    { default: () => t('apps.uninstall') },
                  ),
                default: () =>
                  h(NSpace, { vertical: true, size: 8 }, {
                    default: () => [
                      h('div', null, t('apps.uninstallConfirm', { name: row.name })),
                      h(
                        NCheckbox,
                        {
                          checked: uninstallData.value,
                          'onUpdate:checked': (value: boolean) => (uninstallData.value = value),
                        },
                        { default: () => t('apps.removeData') },
                      ),
                    ],
                  }),
              },
            ),
          ],
        },
      )
    },
  },
])

/** Xóa dữ liệu là thao tác không hoàn tác được nên luôn bắt đầu từ trạng thái tắt. */
const uninstallData = ref(false)
</script>

<template>
  <!-- Thẻ không đặt tiêu đề: thanh tiêu đề của panel đã nói đây là trang nào,
       nhắc lại lần nữa chỉ tốn một dòng ngay chỗ cần cho nội dung. -->
  <NCard size="small">
    <NAlert v-if="!status.available" type="warning" :bordered="false" style="margin-bottom: 12px">
      {{ t('apps.composeUnavailable') }}
    </NAlert>

    <NTabs type="line" animated>
      <template #suffix>
        <NSpace :size="8" align="center">
          <NText v-if="status.version" depth="3" style="font-size: 12px">
            Compose {{ status.version }}
          </NText>
          <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        </NSpace>
      </template>

      <NTabPane name="store" :tab="t('apps.store')">
        <div class="store">
          <div class="store-tools">
            <NInput
              v-model:value="search"
              :placeholder="t('apps.searchPlaceholder')"
              clearable
              size="small"
              class="store-search"
            />
            <span class="store-count sp-metric">
              {{ t('apps.countShown', { shown: visibleCatalog.length, total: catalog.length }) }}
            </span>
          </div>

          <div class="chips">
            <button
              v-for="tab in categoryTabs"
              :key="tab.key"
              type="button"
              class="chip"
              :class="{ 'chip-on': category === tab.key }"
              @click="category = tab.key"
            >
              {{ tab.label }}
              <span class="chip-count sp-metric">{{ tab.count }}</span>
            </button>
          </div>

          <NGrid v-if="visibleCatalog.length > 0" cols="1 640:2 1060:3" :x-gap="14" :y-gap="14">
            <NGi v-for="app in visibleCatalog" :key="app.key">
              <NCard size="small" class="store-card">
                <div class="store-head">
                  <AppIcon :icon="app.icon" :name="localized(app.name)" :size="44" />

                  <div class="store-ident">
                    <div class="store-title">{{ localized(app.name) }}</div>
                    <div class="store-meta sp-metric">
                      {{ t(`apps.category.${app.category}`) }} · v{{ app.version }}
                      <template v-if="installCount[app.key]">
                        · {{ t('apps.installedCount', { count: installCount[app.key] }) }}
                      </template>
                    </div>
                  </div>
                </div>

                <p class="store-desc">{{ localized(app.description) }}</p>

                <div class="store-foot">
                  <a
                    v-if="app.website"
                    class="store-site"
                    :href="app.website"
                    target="_blank"
                    rel="noreferrer noopener"
                  >
                    {{ t('apps.homepage') }}
                  </a>
                  <span v-else></span>

                  <NButton
                    v-if="auth.canWrite"
                    size="small"
                    type="primary"
                    :disabled="!status.available"
                    @click="openInstaller(app)"
                  >
                    {{ t('apps.install') }}
                  </NButton>
                </div>
              </NCard>
            </NGi>
          </NGrid>

          <NEmpty v-else :description="t('apps.noMatch')" style="padding: 40px 0" />
        </div>
      </NTabPane>

      <NTabPane name="installed" :tab="t('apps.installedTab')">
        <NDataTable
          :columns="columns"
          :data="installed"
          :loading="loading"
          :row-key="(row: InstalledApp) => row.id"
          size="small"
        >
          <template #empty>
            <NEmpty :description="t('apps.noneInstalled')" />
          </template>
        </NDataTable>
      </NTabPane>
    </NTabs>
  </NCard>

  <NModal
    v-model:show="installer.show"
    preset="card"
    :title="t('apps.installTitle', { name: localized(installer.app?.name) })"
    style="width: 92vw; max-width: 620px"
  >
    <NForm v-if="installer.app" class="installer-form" @submit.prevent="install">
      <div class="installer-head">
        <AppIcon :icon="installer.app.icon" :name="localized(installer.app.name)" :size="40" />
        <p class="installer-desc">{{ localized(installer.app.description) }}</p>
      </div>

      <NAlert type="info" :bordered="false" style="margin-bottom: 14px">
        {{ t('apps.pullHint', { images: (installer.app.images ?? []).join(', ') }) }}
      </NAlert>

      <NFormItem :label="t('apps.name')" :feedback="t('apps.nameHelp')">
        <NInput v-model:value="installer.name" autofocus />
      </NFormItem>

      <NFormItem
        v-for="field in installer.app.fields ?? []"
        :key="field.key"
        :label="localized(field.label) || field.key"
        :feedback="localized(field.help) || (field.generate ? t('apps.generateHint') : '')"
      >
        <NSelect
          v-if="field.type === 'select'"
          v-model:value="installer.values[field.key]"
          :options="(field.options ?? []).map((o) => ({ label: localized(o.label) || o.value, value: o.value }))"
        />
        <NInputNumber
          v-else-if="field.type === 'port' || field.type === 'number'"
          :value="Number(installer.values[field.key]) || null"
          :min="field.type === 'port' ? 1 : undefined"
          :max="field.type === 'port' ? 65535 : undefined"
          style="width: 100%"
          @update:value="(value: number | null) => (installer.values[field.key] = value === null ? '' : String(value))"
        />
        <NInput
          v-else
          v-model:value="installer.values[field.key]"
          :type="field.type === 'password' ? 'password' : 'text'"
          show-password-on="click"
          :placeholder="field.generate ? t('apps.generatePlaceholder') : ''"
        />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="installer.saving"
        :disabled="!canInstall"
      >
        {{ installer.saving ? t('apps.installing') : t('apps.install') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="logs.show"
    preset="card"
    :title="t('apps.logsOf', { name: logs.name })"
    style="width: 94vw; max-width: 1000px"
  >
    <pre class="app-logs">{{ logs.content || t('apps.noLogs') }}</pre>
  </NModal>

  <NModal
    v-model:show="secrets.show"
    preset="card"
    :title="t('apps.credentialsOf', { name: secrets.name })"
    style="width: 92vw; max-width: 560px"
  >
    <NAlert type="warning" :bordered="false" style="margin-bottom: 12px">
      {{ t('apps.credentialsHint') }}
    </NAlert>

    <div v-for="(value, key) in secrets.values" :key="key" class="secret-row">
      <span class="secret-key">{{ key }}</span>
      <code class="secret-value">{{ value }}</code>
      <NButton size="tiny" quaternary @click="copy(value)">{{ t('common.copy') }}</NButton>
    </div>
  </NModal>
</template>

<style scoped>
.store {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.store-tools {
  display: flex;
  align-items: center;
  gap: 12px;
}

.store-search {
  max-width: 300px;
}

/* Trên điện thoại ô tìm kiếm cần cả bề ngang; con số "hiện bao nhiêu trên bao
   nhiêu" là thông tin phụ nên nhường chỗ. */
@media (max-width: 560px) {
  .store-search {
    max-width: none;
  }

  .store-count {
    display: none;
  }
}

.store-count {
  font-size: 12px;
  color: var(--sp-text-faint);
}

/* Nhóm ứng dụng hiện thành hàng nút thay vì ô chọn xổ xuống: mười nhóm giấu
   sau một ô chọn thì không ai biết chợ có gì cho tới khi bấm vào. */
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  font: inherit;
  font-size: 13px;
  color: var(--sp-text-muted);
  cursor: pointer;
  background: var(--sp-surface-sunken);
  border: 1px solid transparent;
  border-radius: 999px;
  transition: color 0.14s ease, background 0.14s ease, border-color 0.14s ease;
}

.chip:hover {
  color: var(--sp-text);
  border-color: var(--sp-border-strong);
}

.chip-on {
  font-weight: 600;
  color: var(--sp-action);
  background: color-mix(in srgb, var(--sp-action) 12%, transparent);
  border-color: color-mix(in srgb, var(--sp-action) 34%, transparent);
}

.chip-count {
  font-size: 11px;
  opacity: 0.7;
}

.store-card {
  height: 100%;
  transition: border-color 0.14s ease, transform 0.14s ease;
}

.store-card:hover {
  border-color: var(--sp-border-strong);
  transform: translateY(-1px);
}

.store-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.store-ident {
  min-width: 0;
}

.store-title {
  font-size: 15px;
  font-weight: 600;
  /* Tên dài phải cắt bớt chứ không được đẩy phần còn lại ra khỏi thẻ. */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.store-meta {
  margin-top: 2px;
  overflow: hidden;
  font-size: 12px;
  color: var(--sp-text-faint);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.store-desc {
  margin: 12px 0 14px;
  /* Chiều cao tối thiểu giữ hàng nút của mọi thẻ trong lưới thẳng một đường. */
  min-height: 42px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--sp-text-muted);
}

.store-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.store-site {
  font-size: 12px;
  color: var(--sp-text-faint);
  text-decoration: none;
  border-bottom: 1px dashed currentColor;
}

.store-site:hover {
  color: var(--sp-action);
}

/* Dòng gợi ý của một trường dính sát nhãn của trường kế tiếp, khiến biểu mẫu
   đọc như một khối chữ liền. Nới khoảng dưới để mỗi trường thành một cụm. */
.installer-form :deep(.n-form-item-feedback-wrapper) {
  min-height: 0;
  padding: 3px 0 10px;
}

.installer-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 14px;
}

.installer-desc {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  color: var(--sp-text-muted);
}

.app-link {
  color: inherit;
  text-decoration: none;
  border-bottom: 1px dashed currentColor;
}

.app-logs {
  max-height: 65vh;
  margin: 0;
  padding: 12px;
  overflow: auto;
  font-family: var(--sp-font-mono);
  font-size: 12px;
  line-height: 1.6;
  border-radius: 6px;
  background: rgba(128, 128, 128, 0.08);
}

.secret-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 0;
  border-bottom: 1px solid var(--sp-border);
}

.secret-key {
  flex: 0 0 40%;
  font-size: 13px;
  color: var(--sp-text-muted);
}

.secret-value {
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  font-size: 12px;
  white-space: nowrap;
}
</style>
