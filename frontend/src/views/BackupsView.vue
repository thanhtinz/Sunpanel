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
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  backupApi,
  databaseApi,
  type BackupDestination,
  type BackupObject,
  type BackupPlan,
  type BackupRun,
  type BackupSource,
  type DatabaseInfo,
  type DatabaseServer,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const plans = ref<BackupPlan[]>([])
const servers = ref<DatabaseServer[]>([])
const serverDatabases = ref<DatabaseInfo[]>([])
const loading = ref(false)

const emptyPlan = () => ({
  id: 0,
  name: '',
  source: 'database' as BackupSource,
  serverId: null as number | null,
  database: '',
  path: '',
  exclude: 'node_modules,*.log,cache',
  destination: 'local' as BackupDestination,
  destConfig: {
    dir: '',
    endpoint: '',
    region: '',
    bucket: '',
    prefix: '',
    accessKeyId: '',
    secretAccessKey: '',
    pathStyle: 'true',
    url: '',
    username: '',
    password: '',
  } as Record<string, string>,
  schedule: '0 3 * * *',
  keepCount: 7,
  keepDays: 30,
  enabled: true,
})

const editor = ref({ show: false, saving: false, checking: false, form: emptyPlan() })
const history = ref({ show: false, name: '', runs: [] as BackupRun[], loading: false })
const objects = ref({
  show: false,
  planId: 0,
  name: '',
  items: [] as BackupObject[],
  loading: false,
})
const restorer = ref({ show: false, planId: 0, object: '', target: '', running: false })

onMounted(load)

function describeError(err: unknown): string {
  if (!(err instanceof ApiError)) return t('error.network')
  const detail = err.params?.message
  if (typeof detail === 'string' && detail !== '') return detail
  if (err.code.startsWith('backup.')) {
    return t(err.code.replace('backup.', 'backupErr.'), err.params ?? {})
  }
  if (err.code.startsWith('db.')) return t(err.code.replace('db.', 'dbErr.'), err.params ?? {})
  return translateError(err.code, err.params)
}

function report(err: unknown): void {
  message.error(describeError(err))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [planList, serverList] = await Promise.all([backupApi.list(), databaseApi.servers()])
    plans.value = planList
    servers.value = serverList
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

const sourceOptions = computed(() => [
  { label: t('backup.sourceDatabase'), value: 'database' },
  { label: t('backup.sourceDirectory'), value: 'directory' },
])

const destinationOptions = computed(() => [
  { label: t('backup.destLocal'), value: 'local' },
  { label: t('backup.destS3'), value: 's3' },
  { label: t('backup.destWebdav'), value: 'webdav' },
])

const serverOptions = computed(() =>
  servers.value.map((server) => ({ label: server.name, value: server.id })),
)

const databaseOptions = computed(() =>
  serverDatabases.value.filter((d) => !d.system).map((d) => ({ label: d.name, value: d.name })),
)

/** Danh sách cơ sở dữ liệu của máy chủ đang chọn, để người dùng khỏi gõ tay. */
async function loadServerDatabases(serverId: number | null): Promise<void> {
  serverDatabases.value = []
  if (serverId === null) return
  try {
    serverDatabases.value = await databaseApi.databases(serverId)
  } catch (err) {
    report(err)
  }
}

function openCreate(): void {
  editor.value = { show: true, saving: false, checking: false, form: emptyPlan() }
  serverDatabases.value = []
}

function openEdit(plan: BackupPlan): void {
  const form = emptyPlan()
  editor.value = {
    show: true,
    saving: false,
    checking: false,
    form: {
      ...form,
      id: plan.id,
      name: plan.name,
      source: plan.source,
      serverId: plan.serverId || null,
      database: plan.database,
      path: plan.path,
      exclude: plan.exclude,
      destination: plan.destination,
      schedule: plan.schedule,
      keepCount: plan.keepCount,
      keepDays: plan.keepDays,
      enabled: plan.enabled,
    },
  }
  void loadServerDatabases(plan.serverId || null)
}

const canSave = computed(() => {
  const form = editor.value.form
  if (!form.name.trim()) return false
  if (form.source === 'database') return form.serverId !== null && form.database !== ''
  return form.path.trim().startsWith('/')
})

/** Chỉ gửi phần cấu hình của đúng loại nơi lưu đang chọn. */
function destConfigFor(form: ReturnType<typeof emptyPlan>): Record<string, string> {
  const config = form.destConfig
  switch (form.destination) {
    case 's3':
      return {
        endpoint: config.endpoint ?? '',
        region: config.region ?? '',
        bucket: config.bucket ?? '',
        prefix: config.prefix ?? '',
        accessKeyId: config.accessKeyId ?? '',
        secretAccessKey: config.secretAccessKey ?? '',
        pathStyle: config.pathStyle ?? 'true',
      }
    case 'webdav':
      return {
        url: config.url ?? '',
        username: config.username ?? '',
        password: config.password ?? '',
      }
    default:
      return config.dir ? { dir: config.dir } : {}
  }
}

function payloadFor(form: ReturnType<typeof emptyPlan>) {
  return {
    name: form.name.trim(),
    source: form.source,
    serverId: form.serverId ?? 0,
    database: form.database,
    path: form.path.trim(),
    exclude: form.exclude,
    destination: form.destination,
    destConfig: destConfigFor(form),
    schedule: form.schedule.trim(),
    keepCount: form.keepCount,
    keepDays: form.keepDays,
    enabled: form.enabled,
  }
}

async function checkDestination(): Promise<void> {
  editor.value.checking = true
  try {
    await backupApi.check(payloadFor(editor.value.form))
    message.success(t('backup.destinationOk'))
  } catch (err) {
    report(err)
  } finally {
    editor.value.checking = false
  }
}

async function save(): Promise<void> {
  const form = editor.value.form
  editor.value.saving = true
  try {
    if (form.id === 0) {
      await backupApi.create(payloadFor(form))
      message.success(t('backup.created'))
    } else {
      await backupApi.update(form.id, payloadFor(form))
      message.success(t('backup.updated'))
    }
    editor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

async function runNow(plan: BackupPlan): Promise<void> {
  message.info(t('backup.running', { name: plan.name }))
  try {
    const run = await backupApi.run(plan.id)
    message.success(t('backup.done', { size: formatBytes(run.sizeBytes) }))
  } catch (err) {
    report(err)
  } finally {
    await load()
  }
}

async function toggle(plan: BackupPlan, enabled: boolean): Promise<void> {
  try {
    await backupApi.setEnabled(plan.id, enabled)
    await load()
  } catch (err) {
    report(err)
    await load()
  }
}

async function remove(plan: BackupPlan): Promise<void> {
  try {
    await backupApi.remove(plan.id)
    message.success(t('backup.deleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function openHistory(plan: BackupPlan): Promise<void> {
  history.value = { show: true, name: plan.name, runs: [], loading: true }
  try {
    history.value.runs = await backupApi.runs(plan.id)
  } catch (err) {
    report(err)
  } finally {
    history.value.loading = false
  }
}

async function openObjects(plan: BackupPlan): Promise<void> {
  objects.value = { show: true, planId: plan.id, name: plan.name, items: [], loading: true }
  try {
    objects.value.items = await backupApi.objects(plan.id)
  } catch (err) {
    report(err)
    objects.value.show = false
  } finally {
    objects.value.loading = false
  }
}

function openRestore(object: BackupObject): void {
  restorer.value = {
    show: true,
    planId: objects.value.planId,
    object: object.name,
    target: '',
    running: false,
  }
}

async function restore(): Promise<void> {
  restorer.value.running = true
  try {
    await backupApi.restore(restorer.value.planId, restorer.value.object, restorer.value.target)
    message.success(t('backup.restored'))
    restorer.value.show = false
  } catch (err) {
    report(err)
  } finally {
    restorer.value.running = false
  }
}

async function deleteObject(object: BackupObject): Promise<void> {
  try {
    await backupApi.deleteObject(objects.value.planId, object.name)
    message.success(t('backup.objectDeleted'))
    objects.value.items = objects.value.items.filter((item) => item.name !== object.name)
  } catch (err) {
    report(err)
  }
}

const columns = computed<DataTableColumns<BackupPlan>>(() => [
  { title: t('backup.name'), key: 'name', width: 170 },
  {
    title: t('backup.source'),
    key: 'source',
    minWidth: 170,
    render: (row) =>
      h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h(
              NTag,
              { size: 'tiny', bordered: false },
              {
                default: () =>
                  row.source === 'database' ? t('backup.sourceDatabase') : t('backup.sourceDirectory'),
              },
            ),
            h('code', { style: 'font-size:12px' }, row.source === 'database' ? row.database : row.path),
          ],
        },
      ),
  },
  {
    title: t('backup.destination'),
    key: 'destination',
    width: 130,
    render: (row) => t(`backup.dest${row.destination.charAt(0).toUpperCase()}${row.destination.slice(1)}`),
  },
  {
    title: t('backup.schedule'),
    key: 'schedule',
    width: 130,
    render: (row) =>
      row.schedule
        ? h('code', { class: 'sp-metric' }, row.schedule)
        : h(NText, { depth: 3 }, { default: () => t('backup.manualOnly') }),
  },
  {
    title: t('backup.lastRun'),
    key: 'lastRunAt',
    width: 175,
    render: (row) => {
      if (!row.lastRunAt) return h(NText, { depth: 3 }, { default: () => t('common.never') })
      return h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h(
              NTag,
              { size: 'tiny', type: row.lastSuccess ? 'success' : 'error' },
              { default: () => (row.lastSuccess ? t('backup.succeeded') : t('backup.failed')) },
            ),
            h(
              NText,
              { depth: 3, style: 'font-size:12px' },
              { default: () => formatDateTime(row.lastRunAt) },
            ),
          ],
        },
      )
    },
  },
  {
    title: t('backup.enabled'),
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
    width: 290,
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            h(
              NButton,
              { size: 'tiny', quaternary: true, onClick: () => openHistory(row) },
              { default: () => t('backup.history') },
            ),
            auth.canWrite
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openObjects(row) },
                  { default: () => t('backup.copies') },
                )
              : null,
            auth.canWrite
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => runNow(row) },
                  { default: () => t('backup.runNow') },
                )
              : null,
            auth.canWrite
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
                  { default: () => t('common.edit') },
                )
              : null,
            auth.canWrite
              ? h(
                  NPopconfirm,
                  { onPositiveClick: () => remove(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('common.delete') },
                      ),
                    default: () => t('backup.deleteConfirm', { name: row.name }),
                  },
                )
              : null,
          ],
        },
      ),
  },
])

const runColumns = computed<DataTableColumns<BackupRun>>(() => [
  {
    title: t('backup.started'),
    key: 'startedAt',
    width: 175,
    render: (row) => formatDateTime(row.startedAt),
  },
  {
    title: t('backup.result'),
    key: 'success',
    width: 110,
    render: (row) =>
      h(
        NTag,
        { size: 'tiny', type: row.success ? 'success' : 'error' },
        { default: () => (row.success ? t('backup.succeeded') : t('backup.failed')) },
      ),
  },
  { title: t('backup.size'), key: 'sizeBytes', width: 110, render: (row) => formatBytes(row.sizeBytes) },
  {
    title: t('backup.duration'),
    key: 'durationMs',
    width: 110,
    render: (row) => `${(row.durationMs / 1000).toFixed(1)}s`,
  },
  {
    title: t('backup.detail'),
    key: 'object',
    minWidth: 240,
    render: (row) =>
      row.error
        ? h('span', { class: 'run-error' }, row.error)
        : h('code', { style: 'font-size:12px' }, row.object),
  },
])

const objectColumns = computed<DataTableColumns<BackupObject>>(() => [
  { title: t('backup.file'), key: 'name', minWidth: 260, ellipsis: { tooltip: true } },
  { title: t('backup.size'), key: 'sizeBytes', width: 110, render: (row) => formatBytes(row.sizeBytes) },
  {
    title: t('backup.createdAt'),
    key: 'modTime',
    width: 175,
    render: (row) => formatDateTime(row.modTime),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 180,
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            h(
              NButton,
              { size: 'tiny', quaternary: true, type: 'warning', onClick: () => openRestore(row) },
              { default: () => t('backup.restore') },
            ),
            h(
              NPopconfirm,
              { onPositiveClick: () => deleteObject(row) },
              {
                trigger: () =>
                  h(
                    NButton,
                    { size: 'tiny', quaternary: true, type: 'error' },
                    { default: () => t('common.delete') },
                  ),
                default: () => t('backup.objectDeleteConfirm'),
              },
            ),
          ],
        },
      ),
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
      <NSpace :size="8">
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        <NButton v-if="auth.canWrite" size="small" type="primary" @click="openCreate">
          {{ t('backup.create') }}
        </NButton>
      </NSpace>
    </template>

    <NDataTable
      :columns="columns"
      :data="plans"
      :loading="loading"
      :row-key="(row: BackupPlan) => row.id"
      size="small"
    >
      <template #empty>
        <NEmpty :description="t('backup.none')" />
      </template>
    </NDataTable>
  </NCard>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="editor.form.id === 0 ? t('backup.create') : t('backup.edit')"
    style="width: 92vw; max-width: 620px"
  >
    <NForm @submit.prevent="save">
      <NFormItem :label="t('backup.name')">
        <NInput v-model:value="editor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('backup.source')">
        <NSelect v-model:value="editor.form.source" :options="sourceOptions" />
      </NFormItem>

      <template v-if="editor.form.source === 'database'">
        <NFormItem :label="t('backup.server')">
          <NSelect
            v-model:value="editor.form.serverId"
            :options="serverOptions"
            :placeholder="t('db.selectServer')"
            @update:value="loadServerDatabases"
          />
        </NFormItem>

        <NFormItem :label="t('backup.database')">
          <NSelect
            v-model:value="editor.form.database"
            :options="databaseOptions"
            :placeholder="t('db.selectDatabase')"
          />
        </NFormItem>
      </template>

      <template v-else>
        <NFormItem :label="t('backup.path')" :feedback="t('backup.pathHelp')">
          <NInput v-model:value="editor.form.path" placeholder="/www/wwwroot/example.com" />
        </NFormItem>

        <NFormItem :label="t('backup.exclude')" :feedback="t('backup.excludeHelp')">
          <NInput v-model:value="editor.form.exclude" />
        </NFormItem>
      </template>

      <NFormItem :label="t('backup.destination')">
        <NSelect v-model:value="editor.form.destination" :options="destinationOptions" />
      </NFormItem>

      <template v-if="editor.form.destination === 'local'">
        <NFormItem :label="t('backup.dir')" :feedback="t('backup.dirHelp')">
          <NInput v-model:value="editor.form.destConfig.dir" />
        </NFormItem>
        <NAlert type="warning" :bordered="false" style="margin-bottom: 14px">
          {{ t('backup.localWarning') }}
        </NAlert>
      </template>

      <template v-else-if="editor.form.destination === 's3'">
        <NFormItem :label="t('backup.endpoint')" :feedback="t('backup.endpointHelp')">
          <NInput
            v-model:value="editor.form.destConfig.endpoint"
            placeholder="https://s3.ap-southeast-1.amazonaws.com"
          />
        </NFormItem>
        <NSpace :size="12">
          <NFormItem :label="t('backup.bucket')">
            <NInput v-model:value="editor.form.destConfig.bucket" />
          </NFormItem>
          <NFormItem :label="t('backup.region')">
            <NInput v-model:value="editor.form.destConfig.region" placeholder="ap-southeast-1" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('backup.prefix')" :feedback="t('backup.prefixHelp')">
          <NInput v-model:value="editor.form.destConfig.prefix" />
        </NFormItem>
        <NFormItem :label="t('backup.accessKey')">
          <NInput v-model:value="editor.form.destConfig.accessKeyId" />
        </NFormItem>
        <NFormItem :label="t('backup.secretKey')">
          <NInput
            v-model:value="editor.form.destConfig.secretAccessKey"
            type="password"
            show-password-on="click"
          />
        </NFormItem>
      </template>

      <template v-else>
        <NFormItem :label="t('backup.webdavUrl')" :feedback="t('backup.webdavUrlHelp')">
          <NInput v-model:value="editor.form.destConfig.url" />
        </NFormItem>
        <NFormItem :label="t('backup.username')">
          <NInput v-model:value="editor.form.destConfig.username" />
        </NFormItem>
        <NFormItem :label="t('backup.password')">
          <NInput
            v-model:value="editor.form.destConfig.password"
            type="password"
            show-password-on="click"
          />
        </NFormItem>
      </template>

      <NFormItem :label="t('backup.schedule')" :feedback="t('backup.scheduleHelp')">
        <NInput v-model:value="editor.form.schedule" class="sp-metric" />
      </NFormItem>

      <NSpace :size="12">
        <NFormItem :label="t('backup.keepCount')" :feedback="t('backup.keepCountHelp')">
          <NInputNumber v-model:value="editor.form.keepCount" :min="0" />
        </NFormItem>
        <NFormItem :label="t('backup.keepDays')" :feedback="t('backup.keepDaysHelp')">
          <NInputNumber v-model:value="editor.form.keepDays" :min="0" />
        </NFormItem>
      </NSpace>

      <NSpace :size="8" style="margin-bottom: 14px">
        <NButton size="small" :loading="editor.checking" @click="checkDestination">
          {{ t('backup.testDestination') }}
        </NButton>
      </NSpace>

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
    :title="t('backup.historyOf', { name: history.name })"
    style="width: 94vw; max-width: 1100px"
  >
    <NDataTable
      :columns="runColumns"
      :data="history.runs"
      :loading="history.loading"
      :row-key="(row: BackupRun) => row.id"
      size="small"
      max-height="60vh"
    >
      <template #empty>
        <NEmpty :description="t('backup.noRuns')" />
      </template>
    </NDataTable>
  </NModal>

  <NModal
    v-model:show="objects.show"
    preset="card"
    :title="t('backup.copiesOf', { name: objects.name })"
    style="width: 94vw; max-width: 900px"
  >
    <NDataTable
      :columns="objectColumns"
      :data="objects.items"
      :loading="objects.loading"
      :row-key="(row: BackupObject) => row.name"
      size="small"
      max-height="60vh"
    >
      <template #empty>
        <NEmpty :description="t('backup.noCopies')" />
      </template>
    </NDataTable>
  </NModal>

  <NModal
    v-model:show="restorer.show"
    preset="card"
    :title="t('backup.restore')"
    style="width: 92vw; max-width: 520px"
  >
    <NAlert type="error" :bordered="false" style="margin-bottom: 14px">
      {{ t('backup.restoreWarning') }}
    </NAlert>

    <NForm @submit.prevent="restore">
      <NFormItem :label="t('backup.file')">
        <NInput :value="restorer.object" readonly />
      </NFormItem>

      <NFormItem :label="t('backup.target')" :feedback="t('backup.targetHelp')">
        <NInput v-model:value="restorer.target" :placeholder="t('backup.targetPlaceholder')" />
      </NFormItem>

      <NButton type="warning" block attr-type="submit" :loading="restorer.running">
        {{ t('backup.confirmRestore') }}
      </NButton>
    </NForm>
  </NModal>
</template>

<style scoped>
.run-error {
  display: block;
  max-height: 60px;
  overflow: auto;
  font-size: 12px;
  color: var(--sp-text-muted);
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
