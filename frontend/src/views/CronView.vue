<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, cronApi, type CronJob, type CronRun } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const jobs = ref<CronJob[]>([])
const loading = ref(false)

const emptyForm = () => ({
  id: 0,
  name: '',
  schedule: '0 3 * * *',
  command: '',
  workDir: '',
  timeoutSeconds: 0,
})

const editor = ref({ show: false, saving: false, form: emptyForm() })
/** Các lần chạy kế tiếp, xem trước ngay khi người dùng gõ biểu thức. */
const preview = ref<{ next: string[]; error: string }>({ next: [], error: '' })

const history = ref({ show: false, name: '', runs: [] as CronRun[], loading: false })

onMounted(load)

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

/** Mã lỗi cron từ máy chủ có tiền tố "cron.", còn bảng dịch dùng "cronErr." để
 * không đụng với khối chuỗi giao diện "cron.". */
function translateCronError(err: unknown): string {
  if (err instanceof ApiError && err.code.startsWith('cron.')) {
    return t(err.code.replace('cron.', 'cronErr.'))
  }
  return err instanceof ApiError ? translateError(err.code, err.params) : t('error.network')
}

async function load(): Promise<void> {
  loading.value = true
  try {
    jobs.value = await cronApi.list()
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editor.value = { show: true, saving: false, form: emptyForm() }
}

function openEdit(job: CronJob): void {
  editor.value = {
    show: true,
    saving: false,
    form: {
      id: job.id,
      name: job.name,
      schedule: job.schedule,
      command: job.command,
      workDir: job.workDir,
      timeoutSeconds: job.timeoutSeconds,
    },
  }
}

// Xem trước lịch mỗi khi biểu thức đổi, để người dùng biết ngay mình gõ đúng chưa.
watch(
  () => editor.value.form.schedule,
  async (schedule) => {
    if (!editor.value.show || !schedule.trim()) {
      preview.value = { next: [], error: '' }
      return
    }
    try {
      const result = await cronApi.validate(schedule)
      preview.value = { next: result.next, error: '' }
    } catch (err) {
      preview.value = { next: [], error: translateCronError(err) }
    }
  },
  { immediate: true },
)

const canSave = computed(() => {
  const form = editor.value.form
  return (
    form.name.trim() !== '' &&
    form.command.trim() !== '' &&
    preview.value.error === '' &&
    !editor.value.saving
  )
})

async function save(): Promise<void> {
  if (!canSave.value) return

  editor.value.saving = true
  const form = editor.value.form
  const payload = {
    name: form.name.trim(),
    schedule: form.schedule.trim(),
    command: form.command.trim(),
    workDir: form.workDir.trim(),
    timeoutSeconds: form.timeoutSeconds,
  }

  try {
    if (form.id === 0) {
      await cronApi.create(payload)
      message.success(t('cron.created'))
    } else {
      await cronApi.update(form.id, payload)
      message.success(t('cron.updated'))
    }
    editor.value.show = false
    await load()
  } catch (err) {
    message.error(translateCronError(err))
  } finally {
    editor.value.saving = false
  }
}

async function toggle(job: CronJob, enabled: boolean): Promise<void> {
  try {
    await cronApi.setEnabled(job.id, enabled)
    await load()
  } catch (err) {
    report(err)
  }
}

async function runNow(job: CronJob): Promise<void> {
  try {
    const run = await cronApi.runNow(job.id)
    if (run.success) {
      message.success(t('cron.ran'))
    } else {
      message.warning(`${t('cron.failed')} — ${t('cron.exitCode')} ${run.exitCode}`)
    }
    await load()
    // Mở luôn lịch sử để người dùng thấy đầu ra mà không phải bấm thêm.
    await openHistory(job)
  } catch (err) {
    report(err)
  }
}

async function remove(job: CronJob): Promise<void> {
  try {
    await cronApi.remove(job.id)
    message.success(t('cron.deleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function openHistory(job: CronJob): Promise<void> {
  history.value = { show: true, name: job.name, runs: [], loading: true }
  try {
    history.value.runs = await cronApi.runs(job.id)
  } catch (err) {
    report(err)
  } finally {
    history.value.loading = false
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

const columns = computed<DataTableColumns<CronJob>>(() => [
  { title: t('cron.name'), key: 'name' },
  {
    title: t('cron.schedule'),
    key: 'schedule',
    width: 130,
    render: (row) => h('code', { class: 'sp-metric' }, row.schedule),
  },
  {
    title: t('cron.command'),
    key: 'command',
    ellipsis: { tooltip: true },
    render: (row) => h('code', null, row.command),
  },
  {
    title: t('cron.lastRun'),
    key: 'lastRunAt',
    width: 190,
    render: (row) => {
      if (!row.lastRunAt) return h(NText, { depth: 3 }, { default: () => t('cron.never') })
      return h(NSpace, { size: 6, align: 'center' }, {
        default: () => [
          h(NTag, { size: 'tiny', type: row.lastSuccess ? 'success' : 'error' },
            { default: () => (row.lastSuccess ? t('cron.succeeded') : t('cron.failed')) }),
          h(NText, { depth: 3, style: 'font-size:12px' },
            { default: () => formatDateTime(row.lastRunAt) }),
        ],
      })
    },
  },
  {
    title: t('cron.nextRun'),
    key: 'nextRunAt',
    width: 175,
    render: (row) =>
      row.nextRunAt
        ? h('span', { class: 'sp-metric' }, formatDateTime(row.nextRunAt))
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
  {
    title: t('cron.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) =>
      h(NSwitch, {
        value: row.enabled,
        size: 'small',
        disabled: !auth.canWrite,
        'onUpdate:value': (value: boolean) => toggle(row, value),
      }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 260,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => openHistory(row) },
            { default: () => t('cron.history') }),
          auth.canWrite
            ? h(NButton, { size: 'tiny', quaternary: true, onClick: () => runNow(row) },
                { default: () => t('cron.runNow') })
            : null,
          auth.canWrite
            ? h(NButton, { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
                { default: () => t('common.edit') })
            : null,
          auth.canWrite
            ? h(NPopconfirm, { onPositiveClick: () => remove(row) },
                {
                  trigger: () =>
                    h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
                      { default: () => t('common.delete') }),
                  default: () => t('cron.deleteConfirm', { name: row.name }),
                })
            : null,
        ],
      }),
  },
])

const runColumns = computed<DataTableColumns<CronRun>>(() => [
  { title: t('cron.started'), key: 'startedAt', width: 175, render: (row) => formatDateTime(row.startedAt) },
  {
    title: t('cron.result'),
    key: 'success',
    width: 110,
    render: (row) =>
      h(NTag, { size: 'small', type: row.success ? 'success' : 'error' },
        { default: () => (row.success ? t('cron.succeeded') : t('cron.failed')) }),
  },
  { title: t('cron.exitCode'), key: 'exitCode', width: 90 },
  { title: t('cron.duration'), key: 'durationMs', width: 100, render: (row) => formatDuration(row.durationMs) },
  {
    title: t('cron.triggeredBy'),
    key: 'triggeredBy',
    width: 110,
    render: (row) => (row.triggeredBy === 'manual' ? t('cron.byManual') : t('cron.bySchedule')),
  },
  {
    title: t('cron.output'),
    key: 'output',
    render: (row) =>
      h('code', { class: 'run-output' }, (row.output || row.error || '').trim() || t('cron.noOutput')),
  },
])
</script>

<template>
  <NCard :title="t('cron.title')" size="small">
    <template #header-extra>
      <NSpace :size="8">
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        <NButton v-if="auth.canWrite" size="small" type="primary" @click="openCreate">
          {{ t('cron.create') }}
        </NButton>
      </NSpace>
    </template>

    <NDataTable
      :columns="columns"
      :data="jobs"
      :loading="loading"
      :row-key="(row: CronJob) => row.id"
      size="small"
    />
  </NCard>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="editor.form.id === 0 ? t('cron.create') : t('cron.edit')"
    style="width: 92vw; max-width: 640px"
  >
    <NForm @submit.prevent="save">
      <NFormItem :label="t('cron.name')">
        <NInput v-model:value="editor.form.name" autofocus />
      </NFormItem>

      <NFormItem
        :label="t('cron.schedule')"
        :validation-status="preview.error ? 'error' : undefined"
        :feedback="preview.error || t('cron.scheduleHelp')"
      >
        <NInput v-model:value="editor.form.schedule" class="sp-metric" />
      </NFormItem>

      <div v-if="preview.next.length > 0" class="preview">
        <NText depth="3" style="font-size: 12px">{{ t('cron.nextRuns') }}:</NText>
        <NSpace :size="6" style="margin-top: 4px">
          <NTag v-for="at in preview.next" :key="at" size="tiny" :bordered="false">
            {{ formatDateTime(at) }}
          </NTag>
        </NSpace>
      </div>

      <NFormItem :label="t('cron.command')" :feedback="t('cron.commandHelp')">
        <NInput
          v-model:value="editor.form.command"
          type="textarea"
          :rows="3"
          class="sp-metric"
        />
      </NFormItem>

      <NFormItem :label="t('cron.workDir')" :feedback="t('cron.workDirHelp')">
        <NInput v-model:value="editor.form.workDir" />
      </NFormItem>

      <NFormItem :label="t('cron.timeout')">
        <NInputNumber v-model:value="editor.form.timeoutSeconds" :min="0" style="width: 100%" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="editor.saving"
        :disabled="!canSave"
      >
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="history.show"
    preset="card"
    :title="t('cron.historyOf', { name: history.name })"
    style="width: 94vw; max-width: 1200px"
  >
    <NDataTable
      :columns="runColumns"
      :data="history.runs"
      :loading="history.loading"
      :row-key="(row: CronRun) => row.id"
      size="small"
      max-height="60vh"
    />
  </NModal>
</template>

<style scoped>
.preview {
  margin: -8px 0 16px;
}

.run-output {
  display: block;
  max-height: 60px;
  overflow: auto;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
