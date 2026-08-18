<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
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
  alertApi,
  type AlertEvent,
  type AlertLevel,
  type AlertMetric,
  type AlertRule,
  type NotifyChannel,
  type NotifyKind,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const channels = ref<NotifyChannel[]>([])
const rules = ref<AlertRule[]>([])
const events = ref<AlertEvent[]>([])
const loading = ref(false)

const emptyChannel = () => ({
  id: 0,
  name: '',
  kind: 'telegram' as NotifyKind,
  enabled: true,
  config: {
    host: '',
    port: '587',
    username: '',
    password: '',
    from: '',
    to: '',
    token: '',
    chatId: '',
    url: '',
    headers: '',
  } as Record<string, string>,
})

const emptyRule = () => ({
  id: 0,
  name: '',
  metric: 'cpu' as AlertMetric,
  threshold: 90,
  durationMinutes: 5,
  cooldownMinutes: 60,
  level: 'warning' as AlertLevel,
  channels: '',
  enabled: true,
})

const channelEditor = ref({ show: false, saving: false, form: emptyChannel() })
const ruleEditor = ref({ show: false, saving: false, form: emptyRule() })

onMounted(load)

function describeError(err: unknown): string {
  if (!(err instanceof ApiError)) return t('error.network')
  const detail = err.params?.message
  if (typeof detail === 'string' && detail !== '') return detail
  if (err.code.startsWith('alert.')) {
    return t(err.code.replace('alert.', 'alertErr.'), err.params ?? {})
  }
  return translateError(err.code, err.params)
}

function report(err: unknown): void {
  message.error(describeError(err))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [channelList, ruleList, eventList] = await Promise.all([
      alertApi.channels(),
      alertApi.rules(),
      alertApi.events(),
    ])
    channels.value = channelList
    rules.value = ruleList
    events.value = eventList
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

const kindOptions = computed(() => [
  { label: t('alert.kindTelegram'), value: 'telegram' },
  { label: t('alert.kindEmail'), value: 'email' },
  { label: t('alert.kindWebhook'), value: 'webhook' },
])

const metricOptions = computed(() => [
  { label: t('alert.metricCpu'), value: 'cpu' },
  { label: t('alert.metricMemory'), value: 'memory' },
  { label: t('alert.metricDisk'), value: 'disk' },
  { label: t('alert.metricLoad'), value: 'load' },
])

const levelOptions = computed(() => [
  { label: t('alert.levelInfo'), value: 'info' },
  { label: t('alert.levelWarning'), value: 'warning' },
  { label: t('alert.levelCritical'), value: 'critical' },
])

const channelOptions = computed(() =>
  channels.value.map((channel) => ({ label: channel.name, value: channel.name })),
)

function openChannelCreate(): void {
  channelEditor.value = { show: true, saving: false, form: emptyChannel() }
}

function openChannelEdit(channel: NotifyChannel): void {
  const form = emptyChannel()
  channelEditor.value = {
    show: true,
    saving: false,
    // Cấu hình không bao giờ được trả về từ máy chủ; để trống nghĩa là giữ nguyên.
    form: { ...form, id: channel.id, name: channel.name, kind: channel.kind, enabled: channel.enabled },
  }
}

/** Chỉ gửi phần cấu hình của đúng loại kênh đang chọn. */
function configFor(form: ReturnType<typeof emptyChannel>): Record<string, string> {
  const config = form.config
  switch (form.kind) {
    case 'email':
      return {
        host: config.host ?? '',
        port: config.port ?? '587',
        username: config.username ?? '',
        password: config.password ?? '',
        from: config.from ?? '',
        to: config.to ?? '',
      }
    case 'telegram':
      return { token: config.token ?? '', chatId: config.chatId ?? '' }
    default:
      return { url: config.url ?? '', headers: config.headers ?? '' }
  }
}

/** Cấu hình rỗng khi sửa nghĩa là giữ nguyên; khi tạo mới thì phải điền. */
function hasConfig(form: ReturnType<typeof emptyChannel>): boolean {
  const config = configFor(form)
  switch (form.kind) {
    case 'email':
      return Boolean(config.host && config.from && config.to)
    case 'telegram':
      return Boolean(config.token && config.chatId)
    default:
      return Boolean(config.url)
  }
}

const canSaveChannel = computed(() => {
  const form = channelEditor.value.form
  if (!form.name.trim()) return false
  return form.id !== 0 || hasConfig(form)
})

async function saveChannel(): Promise<void> {
  const form = channelEditor.value.form
  channelEditor.value.saving = true
  try {
    const payload = {
      name: form.name.trim(),
      kind: form.kind,
      config: hasConfig(form) ? configFor(form) : undefined,
      enabled: form.enabled,
    }
    if (form.id === 0) {
      await alertApi.createChannel(payload)
      message.success(t('alert.channelCreated'))
    } else {
      await alertApi.updateChannel(form.id, payload)
      message.success(t('alert.channelUpdated'))
    }
    channelEditor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    channelEditor.value.saving = false
  }
}

async function testChannel(channel: NotifyChannel): Promise<void> {
  try {
    await alertApi.testChannel(channel.id)
    message.success(t('alert.testSent'))
  } catch (err) {
    report(err)
  } finally {
    await load()
  }
}

async function deleteChannel(channel: NotifyChannel): Promise<void> {
  try {
    await alertApi.deleteChannel(channel.id)
    message.success(t('alert.channelDeleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

function openRuleCreate(): void {
  ruleEditor.value = { show: true, saving: false, form: emptyRule() }
}

function openRuleEdit(rule: AlertRule): void {
  ruleEditor.value = { show: true, saving: false, form: { ...rule } }
}

async function saveRule(): Promise<void> {
  const form = ruleEditor.value.form
  ruleEditor.value.saving = true
  try {
    const payload = {
      name: form.name.trim(),
      metric: form.metric,
      threshold: form.threshold,
      durationMinutes: form.durationMinutes,
      cooldownMinutes: form.cooldownMinutes,
      level: form.level,
      channels: form.channels,
      enabled: form.enabled,
    }
    if (form.id === 0) {
      await alertApi.createRule(payload)
      message.success(t('alert.ruleCreated'))
    } else {
      await alertApi.updateRule(form.id, payload)
      message.success(t('alert.ruleUpdated'))
    }
    ruleEditor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    ruleEditor.value.saving = false
  }
}

async function toggleRule(rule: AlertRule, enabled: boolean): Promise<void> {
  try {
    await alertApi.updateRule(rule.id, { ...rule, enabled })
    await load()
  } catch (err) {
    report(err)
    await load()
  }
}

async function deleteRule(rule: AlertRule): Promise<void> {
  try {
    await alertApi.deleteRule(rule.id)
    message.success(t('alert.ruleDeleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

function levelTag(level: AlertLevel) {
  const type = level === 'critical' ? 'error' : level === 'warning' ? 'warning' : 'info'
  return h(
    NTag,
    { size: 'tiny', type, bordered: false },
    { default: () => t(`alert.level${level.charAt(0).toUpperCase()}${level.slice(1)}`) },
  )
}

const channelColumns = computed<DataTableColumns<NotifyChannel>>(() => [
  { title: t('alert.channelName'), key: 'name', width: 180 },
  {
    title: t('alert.kind'),
    key: 'kind',
    width: 140,
    render: (row) => t(`alert.kind${row.kind.charAt(0).toUpperCase()}${row.kind.slice(1)}`),
  },
  {
    title: t('alert.lastSent'),
    key: 'lastSentAt',
    minWidth: 220,
    render: (row) => {
      if (row.lastError) {
        // Một kênh hỏng mà không ai biết cũng tệ như không có cảnh báo.
        return h(
          NSpace,
          { size: 6, align: 'center' },
          {
            default: () => [
              h(NTag, { size: 'tiny', type: 'error', bordered: false }, { default: () => t('alert.channelBroken') }),
              h(NText, { depth: 3, style: 'font-size:12px' }, { default: () => row.lastError ?? '' }),
            ],
          },
        )
      }
      if (!row.lastSentAt) return h(NText, { depth: 3 }, { default: () => t('common.never') })
      return h(NText, { depth: 3, style: 'font-size:12px' }, { default: () => formatDateTime(row.lastSentAt) })
    },
  },
  {
    title: t('alert.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) =>
      h(
        NTag,
        { size: 'tiny', type: row.enabled ? 'success' : 'default', bordered: false },
        { default: () => (row.enabled ? t('common.enabled') : t('common.disabled')) },
      ),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row) =>
      auth.canWrite
        ? h(
            NSpace,
            { size: 4 },
            {
              default: () => [
                h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => testChannel(row) },
                  { default: () => t('alert.test') },
                ),
                h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openChannelEdit(row) },
                  { default: () => t('common.edit') },
                ),
                h(
                  NPopconfirm,
                  { onPositiveClick: () => deleteChannel(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('common.delete') },
                      ),
                    default: () => t('alert.channelDeleteConfirm', { name: row.name }),
                  },
                ),
              ],
            },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
])

const ruleColumns = computed<DataTableColumns<AlertRule>>(() => [
  { title: t('alert.ruleName'), key: 'name', width: 170 },
  {
    title: t('alert.condition'),
    key: 'metric',
    minWidth: 240,
    render: (row) =>
      t('alert.conditionText', {
        metric: t(`alert.metric${row.metric.charAt(0).toUpperCase()}${row.metric.slice(1)}`),
        threshold: row.threshold,
        unit: row.metric === 'load' ? '' : '%',
        minutes: row.durationMinutes,
      }),
  },
  { title: t('alert.level'), key: 'level', width: 110, render: (row) => levelTag(row.level) },
  {
    title: t('alert.targets'),
    key: 'channels',
    minWidth: 150,
    render: (row) =>
      row.channels
        ? row.channels
        : h(NText, { depth: 3 }, { default: () => t('alert.allChannels') }),
  },
  {
    title: t('alert.lastFired'),
    key: 'lastFiredAt',
    width: 170,
    render: (row) =>
      row.lastFiredAt
        ? formatDateTime(row.lastFiredAt)
        : h(NText, { depth: 3 }, { default: () => t('common.never') }),
  },
  {
    title: t('alert.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) =>
      h(NSwitch, {
        value: row.enabled,
        size: 'small',
        disabled: !auth.canWrite,
        'onUpdate:value': (value: boolean) => toggleRule(row, value),
      }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 130,
    render: (row) =>
      auth.canWrite
        ? h(
            NSpace,
            { size: 4 },
            {
              default: () => [
                h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openRuleEdit(row) },
                  { default: () => t('common.edit') },
                ),
                h(
                  NPopconfirm,
                  { onPositiveClick: () => deleteRule(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('common.delete') },
                      ),
                    default: () => t('alert.ruleDeleteConfirm', { name: row.name }),
                  },
                ),
              ],
            },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
])

const eventColumns = computed<DataTableColumns<AlertEvent>>(() => [
  {
    title: t('alert.time'),
    key: 'createdAt',
    width: 175,
    render: (row) => formatDateTime(row.createdAt),
  },
  { title: t('alert.level'), key: 'level', width: 110, render: (row) => levelTag(row.level) },
  { title: t('alert.event'), key: 'title', minWidth: 260, ellipsis: { tooltip: true } },
  {
    title: t('alert.delivery'),
    key: 'delivered',
    minWidth: 200,
    render: (row) => {
      if (row.failed === 0) {
        return h(
          NTag,
          { size: 'tiny', type: 'success', bordered: false },
          { default: () => t('alert.deliveredTo', { count: row.delivered }) },
        )
      }
      return h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h(
              NTag,
              { size: 'tiny', type: 'error', bordered: false },
              { default: () => t('alert.deliveryFailed', { count: row.failed }) },
            ),
            h(NText, { depth: 3, style: 'font-size:12px' }, { default: () => row.error ?? '' }),
          ],
        },
      )
    },
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
      <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
    </template>

    <NTabs type="line" animated>
      <NTabPane name="rules" :tab="t('alert.rules')">
        <NSpace vertical :size="12">
          <NButton v-if="auth.canWrite" size="small" type="primary" @click="openRuleCreate">
            {{ t('alert.createRule') }}
          </NButton>

          <NAlert v-if="channels.length === 0" type="warning" :bordered="false">
            {{ t('alert.noChannelsWarning') }}
          </NAlert>

          <NDataTable
            :columns="ruleColumns"
            :data="rules"
            :loading="loading"
            :row-key="(row: AlertRule) => row.id"
            size="small"
          >
            <template #empty>
              <NEmpty :description="t('alert.noRules')" />
            </template>
          </NDataTable>
        </NSpace>
      </NTabPane>

      <NTabPane name="channels" :tab="t('alert.channels')">
        <NSpace vertical :size="12">
          <NButton v-if="auth.canWrite" size="small" type="primary" @click="openChannelCreate">
            {{ t('alert.createChannel') }}
          </NButton>

          <NDataTable
            :columns="channelColumns"
            :data="channels"
            :loading="loading"
            :row-key="(row: NotifyChannel) => row.id"
            size="small"
          >
            <template #empty>
              <NEmpty :description="t('alert.noChannels')" />
            </template>
          </NDataTable>
        </NSpace>
      </NTabPane>

      <NTabPane name="events" :tab="t('alert.history')">
        <NDataTable
          :columns="eventColumns"
          :data="events"
          :loading="loading"
          :row-key="(row: AlertEvent) => row.id"
          size="small"
          max-height="60vh"
        >
          <template #empty>
            <NEmpty :description="t('alert.noEvents')" />
          </template>
        </NDataTable>
      </NTabPane>
    </NTabs>
  </NCard>

  <NModal
    v-model:show="channelEditor.show"
    preset="card"
    :title="channelEditor.form.id === 0 ? t('alert.createChannel') : t('alert.editChannel')"
    style="width: 92vw; max-width: 560px"
  >
    <NForm @submit.prevent="saveChannel">
      <NFormItem :label="t('alert.channelName')">
        <NInput v-model:value="channelEditor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('alert.kind')">
        <NSelect v-model:value="channelEditor.form.kind" :options="kindOptions" />
      </NFormItem>

      <NAlert
        v-if="channelEditor.form.id !== 0"
        type="info"
        :bordered="false"
        style="margin-bottom: 14px"
      >
        {{ t('alert.configKeepHint') }}
      </NAlert>

      <template v-if="channelEditor.form.kind === 'telegram'">
        <NFormItem :label="t('alert.token')" :feedback="t('alert.tokenHelp')">
          <NInput
            v-model:value="channelEditor.form.config.token"
            type="password"
            show-password-on="click"
          />
        </NFormItem>
        <NFormItem :label="t('alert.chatId')" :feedback="t('alert.chatIdHelp')">
          <NInput v-model:value="channelEditor.form.config.chatId" />
        </NFormItem>
      </template>

      <template v-else-if="channelEditor.form.kind === 'email'">
        <NSpace :size="12">
          <NFormItem :label="t('alert.smtpHost')">
            <NInput v-model:value="channelEditor.form.config.host" placeholder="smtp.gmail.com" />
          </NFormItem>
          <NFormItem :label="t('alert.smtpPort')">
            <NInput v-model:value="channelEditor.form.config.port" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('alert.smtpUser')">
          <NInput v-model:value="channelEditor.form.config.username" />
        </NFormItem>
        <NFormItem :label="t('alert.smtpPassword')" :feedback="t('alert.smtpPasswordHelp')">
          <NInput
            v-model:value="channelEditor.form.config.password"
            type="password"
            show-password-on="click"
          />
        </NFormItem>
        <NFormItem :label="t('alert.from')">
          <NInput v-model:value="channelEditor.form.config.from" />
        </NFormItem>
        <NFormItem :label="t('alert.to')" :feedback="t('alert.toHelp')">
          <NInput v-model:value="channelEditor.form.config.to" />
        </NFormItem>
      </template>

      <template v-else>
        <NFormItem :label="t('alert.webhookUrl')">
          <NInput v-model:value="channelEditor.form.config.url" placeholder="https://..." />
        </NFormItem>
        <NFormItem :label="t('alert.headers')" :feedback="t('alert.headersHelp')">
          <NInput v-model:value="channelEditor.form.config.headers" type="textarea" :rows="2" />
        </NFormItem>
      </template>

      <NFormItem :label="t('alert.enabled')">
        <NSwitch v-model:value="channelEditor.form.enabled" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="channelEditor.saving"
        :disabled="!canSaveChannel"
      >
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="ruleEditor.show"
    preset="card"
    :title="ruleEditor.form.id === 0 ? t('alert.createRule') : t('alert.editRule')"
    style="width: 92vw; max-width: 560px"
  >
    <NForm @submit.prevent="saveRule">
      <NFormItem :label="t('alert.ruleName')">
        <NInput v-model:value="ruleEditor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('alert.metric')">
        <NSelect v-model:value="ruleEditor.form.metric" :options="metricOptions" />
      </NFormItem>

      <NFormItem :label="t('alert.threshold')" :feedback="t('alert.thresholdHelp')">
        <NInputNumber v-model:value="ruleEditor.form.threshold" :min="0" style="width: 100%" />
      </NFormItem>

      <NSpace :size="12">
        <NFormItem :label="t('alert.duration')" :feedback="t('alert.durationHelp')">
          <NInputNumber v-model:value="ruleEditor.form.durationMinutes" :min="0" />
        </NFormItem>
        <NFormItem :label="t('alert.cooldown')" :feedback="t('alert.cooldownHelp')">
          <NInputNumber v-model:value="ruleEditor.form.cooldownMinutes" :min="1" />
        </NFormItem>
      </NSpace>

      <NFormItem :label="t('alert.level')">
        <NSelect v-model:value="ruleEditor.form.level" :options="levelOptions" />
      </NFormItem>

      <NFormItem :label="t('alert.targets')" :feedback="t('alert.targetsHelp')">
        <NSelect
          :options="channelOptions"
          multiple
          clearable
          :placeholder="t('alert.allChannels')"
          :value="ruleEditor.form.channels ? ruleEditor.form.channels.split(',') : []"
          @update:value="(value: string[]) => (ruleEditor.form.channels = (value ?? []).join(','))"
        />
      </NFormItem>

      <NFormItem :label="t('alert.enabled')">
        <NSwitch v-model:value="ruleEditor.form.enabled" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="ruleEditor.saving"
        :disabled="!ruleEditor.form.name.trim() || ruleEditor.form.threshold <= 0"
      >
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>
</template>
