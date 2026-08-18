<script setup lang="ts">
import { computed, h, onMounted, ref, watch } from 'vue'
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
  databaseApi,
  type DatabaseInfo,
  type DatabaseKind,
  type DatabaseServer,
  type DatabaseTable,
  type DatabaseUser,
  type QueryResult,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const servers = ref<DatabaseServer[]>([])
const selectedId = ref<number | null>(null)
const databases = ref<DatabaseInfo[]>([])
const users = ref<DatabaseUser[]>([])
const tables = ref<DatabaseTable[]>([])
const loading = ref(false)

const emptyServer = () => ({
  id: 0,
  name: '',
  kind: 'mysql' as DatabaseKind,
  host: '127.0.0.1',
  port: 3306,
  user: 'root',
  password: '',
  remark: '',
})

const serverEditor = ref({ show: false, saving: false, form: emptyServer() })
const dbCreator = ref({ show: false, saving: false, name: '' })
const userEditor = ref({
  show: false,
  saving: false,
  mode: 'create' as 'create' | 'password',
  form: { name: '', password: '', database: '', host: '%' },
})
const tableViewer = ref({ show: false, database: '' })

const console_ = ref({
  database: '',
  statement: 'SELECT 1;',
  running: false,
  result: null as QueryResult | null,
  error: '',
})

const selected = computed(() => servers.value.find((s) => s.id === selectedId.value) ?? null)
const isPostgres = computed(() => selected.value?.kind === 'postgres')

onMounted(loadServers)

/** Mô tả lỗi cho người đọc, ưu tiên thông báo nguyên văn từ máy chủ cơ sở dữ liệu. */
function describeError(err: unknown): string {
  if (!(err instanceof ApiError)) return t('error.network')
  const detail = err.params?.message
  if (typeof detail === 'string' && detail !== '') return detail
  if (err.code.startsWith('db.')) return t(err.code.replace('db.', 'dbErr.'), err.params ?? {})
  return translateError(err.code, err.params)
}

function report(err: unknown): void {
  message.error(describeError(err))
}

async function loadServers(): Promise<void> {
  loading.value = true
  try {
    servers.value = await databaseApi.servers()
    if (selectedId.value === null || !servers.value.some((s) => s.id === selectedId.value)) {
      selectedId.value = servers.value.find((s) => s.online)?.id ?? servers.value[0]?.id ?? null
    }
    await loadContents()
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/** Nội dung của máy chủ đang chọn: danh sách cơ sở dữ liệu và tài khoản. */
async function loadContents(): Promise<void> {
  const id = selectedId.value
  if (id === null || !selected.value?.online) {
    databases.value = []
    users.value = []
    return
  }

  try {
    const [dbList, userList] = await Promise.all([
      databaseApi.databases(id),
      databaseApi.users(id),
    ])
    databases.value = dbList
    users.value = userList

    // Cửa sổ SQL cần một cơ sở dữ liệu để chạy vào; chọn sẵn cái đầu tiên không
    // phải của hệ thống để người dùng không phải chọn tay mỗi lần.
    if (!console_.value.database) {
      console_.value.database = dbList.find((d) => !d.system)?.name ?? ''
    }
  } catch (err) {
    report(err)
  }
}

watch(selectedId, () => {
  console_.value.database = ''
  console_.value.result = null
  void loadContents()
})

const serverOptions = computed(() =>
  servers.value.map((server) => ({
    label: `${server.name} (${server.kind === 'mysql' ? 'MySQL/MariaDB' : 'PostgreSQL'})`,
    value: server.id,
  })),
)

const kindOptions = computed(() => [
  { label: 'MySQL / MariaDB', value: 'mysql' },
  { label: 'PostgreSQL', value: 'postgres' },
])

const databaseOptions = computed(() =>
  databases.value.filter((d) => !d.system).map((d) => ({ label: d.name, value: d.name })),
)

function openServerCreate(): void {
  serverEditor.value = { show: true, saving: false, form: emptyServer() }
}

function openServerEdit(server: DatabaseServer): void {
  serverEditor.value = {
    show: true,
    saving: false,
    form: {
      id: server.id,
      name: server.name,
      kind: server.kind,
      host: server.host,
      port: server.port,
      user: server.user,
      // Mật khẩu không bao giờ được trả về từ máy chủ; để trống nghĩa là giữ nguyên.
      password: '',
      remark: server.remark,
    },
  }
}

/** Đổi loại máy chủ thì cổng mặc định đổi theo, trừ khi người dùng đã tự sửa. */
watch(
  () => serverEditor.value.form.kind,
  (kind, previous) => {
    if (previous === undefined) return
    const form = serverEditor.value.form
    if (form.port === 3306 || form.port === 5432) {
      form.port = kind === 'postgres' ? 5432 : 3306
    }
    if (form.user === 'root' || form.user === 'postgres') {
      form.user = kind === 'postgres' ? 'postgres' : 'root'
    }
  },
)

const canSaveServer = computed(() => {
  const form = serverEditor.value.form
  if (!form.name.trim() || !form.host.trim() || !form.user.trim()) return false
  // Máy chủ mới bắt buộc phải có mật khẩu; khi sửa thì để trống là giữ nguyên.
  return form.id !== 0 || form.password.length > 0
})

async function saveServer(): Promise<void> {
  const form = serverEditor.value.form
  serverEditor.value.saving = true
  try {
    const payload = {
      name: form.name.trim(),
      kind: form.kind,
      host: form.host.trim(),
      port: form.port,
      user: form.user.trim(),
      password: form.password,
      remark: form.remark.trim(),
    }
    if (form.id === 0) {
      await databaseApi.createServer(payload)
      message.success(t('db.serverAdded'))
    } else {
      await databaseApi.updateServer(form.id, payload)
      message.success(t('db.serverUpdated'))
    }
    serverEditor.value.show = false
    await loadServers()
  } catch (err) {
    report(err)
  } finally {
    serverEditor.value.saving = false
  }
}

async function deleteServer(server: DatabaseServer): Promise<void> {
  try {
    await databaseApi.deleteServer(server.id)
    message.success(t('db.serverRemoved'))
    await loadServers()
  } catch (err) {
    report(err)
  }
}

async function createDatabase(): Promise<void> {
  const id = selectedId.value
  if (id === null) return

  dbCreator.value.saving = true
  try {
    await databaseApi.createDatabase(id, dbCreator.value.name.trim())
    message.success(t('db.databaseCreated'))
    dbCreator.value.show = false
    dbCreator.value.name = ''
    await loadContents()
  } catch (err) {
    report(err)
  } finally {
    dbCreator.value.saving = false
  }
}

async function dropDatabase(database: DatabaseInfo): Promise<void> {
  const id = selectedId.value
  if (id === null) return
  try {
    await databaseApi.dropDatabase(id, database.name)
    message.success(t('db.databaseDropped'))
    await loadContents()
  } catch (err) {
    report(err)
  }
}

async function showTables(database: DatabaseInfo): Promise<void> {
  const id = selectedId.value
  if (id === null) return

  tableViewer.value = { show: true, database: database.name }
  try {
    tables.value = await databaseApi.tables(id, database.name)
  } catch (err) {
    report(err)
    tableViewer.value.show = false
  }
}

function openUserCreate(): void {
  userEditor.value = {
    show: true,
    saving: false,
    mode: 'create',
    form: { name: '', password: '', database: '', host: '%' },
  }
}

function openPasswordChange(user: DatabaseUser): void {
  userEditor.value = {
    show: true,
    saving: false,
    mode: 'password',
    form: { name: user.name, password: '', database: '', host: user.host ?? '%' },
  }
}

async function saveUser(): Promise<void> {
  const id = selectedId.value
  if (id === null) return

  const editor = userEditor.value
  editor.saving = true
  try {
    if (editor.mode === 'create') {
      await databaseApi.createUser(id, editor.form)
      message.success(t('db.userCreated'))
    } else {
      await databaseApi.changePassword(id, editor.form)
      message.success(t('db.passwordChanged'))
    }
    editor.show = false
    await loadContents()
  } catch (err) {
    report(err)
  } finally {
    editor.saving = false
  }
}

async function dropUser(user: DatabaseUser): Promise<void> {
  const id = selectedId.value
  if (id === null) return
  try {
    await databaseApi.dropUser(id, user.name, user.host)
    message.success(t('db.userDropped'))
    await loadContents()
  } catch (err) {
    report(err)
  }
}

async function runQuery(): Promise<void> {
  const id = selectedId.value
  if (id === null) return

  console_.value.running = true
  console_.value.error = ''
  try {
    console_.value.result = await databaseApi.query(
      id,
      console_.value.database,
      console_.value.statement,
    )
  } catch (err) {
    console_.value.result = null
    // Lỗi cú pháp SQL hiện ngay trong cửa sổ chứ không phải một thông báo thoáng
    // qua: người dùng cần đọc lại nó trong lúc sửa câu lệnh. Ưu tiên nguyên văn
    // thông báo của máy chủ cơ sở dữ liệu — nó nói rõ sai ở đâu.
    console_.value.error = describeError(err)
  } finally {
    console_.value.running = false
  }
}

const serverColumns = computed<DataTableColumns<DatabaseServer>>(() => [
  { title: t('db.serverName'), key: 'name', width: 180 },
  {
    title: t('db.kind'),
    key: 'kind',
    width: 150,
    render: (row) => (row.kind === 'mysql' ? 'MySQL / MariaDB' : 'PostgreSQL'),
  },
  {
    title: t('db.address'),
    key: 'host',
    minWidth: 160,
    render: (row) => h('code', null, `${row.host}:${row.port}`),
  },
  { title: t('db.user'), key: 'user', width: 120 },
  {
    title: t('db.status'),
    key: 'online',
    minWidth: 200,
    render: (row) => {
      if (!row.online) {
        return h(
          NSpace,
          { size: 6, align: 'center' },
          {
            default: () => [
              h(NTag, { size: 'small', type: 'error', bordered: false }, { default: () => t('db.offline') }),
              h(NText, { depth: 3, style: 'font-size:12px' }, { default: () => row.error ?? '' }),
            ],
          },
        )
      }
      return h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => t('db.online') }),
            h(
              NText,
              { depth: 3, style: 'font-size:12px' },
              { default: () => (row.version ?? '').split(' ').slice(0, 2).join(' ') },
            ),
          ],
        },
      )
    },
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 150,
    render: (row) =>
      auth.canWrite
        ? h(
            NSpace,
            { size: 4 },
            {
              default: () => [
                h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openServerEdit(row) },
                  { default: () => t('common.edit') },
                ),
                h(
                  NPopconfirm,
                  { onPositiveClick: () => deleteServer(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('db.remove') },
                      ),
                    default: () => t('db.removeConfirm', { name: row.name }),
                  },
                ),
              ],
            },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
])

const databaseColumns = computed<DataTableColumns<DatabaseInfo>>(() => [
  {
    title: t('db.databaseName'),
    key: 'name',
    minWidth: 200,
    render: (row) =>
      h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h('span', null, row.name),
            row.system
              ? h(NTag, { size: 'tiny', bordered: false }, { default: () => t('db.system') })
              : null,
          ],
        },
      ),
  },
  { title: t('db.charset'), key: 'charset', width: 160 },
  {
    title: t('db.size'),
    key: 'sizeBytes',
    width: 120,
    render: (row) => formatBytes(row.sizeBytes),
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
              { size: 'tiny', quaternary: true, onClick: () => showTables(row) },
              { default: () => t('db.tables') },
            ),
            // Cơ sở dữ liệu hệ thống không cho xóa; ẩn hẳn nút thay vì để người
            // dùng bấm rồi nhận lỗi.
            auth.canWrite && !row.system
              ? h(
                  NPopconfirm,
                  { onPositiveClick: () => dropDatabase(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('db.drop') },
                      ),
                    default: () => t('db.dropConfirm', { name: row.name }),
                  },
                )
              : null,
          ],
        },
      ),
  },
])

const userColumns = computed<DataTableColumns<DatabaseUser>>(() => [
  {
    title: t('db.userName'),
    key: 'name',
    minWidth: 180,
    render: (row) =>
      h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h('span', null, row.name),
            row.superUser
              ? h(NTag, { size: 'tiny', type: 'warning', bordered: false }, { default: () => t('db.superUser') })
              : null,
          ],
        },
      ),
  },
  { title: t('db.host'), key: 'host', width: 140, render: (row) => row.host || '—' },
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
                  { size: 'tiny', quaternary: true, onClick: () => openPasswordChange(row) },
                  { default: () => t('db.changePassword') },
                ),
                h(
                  NPopconfirm,
                  { onPositiveClick: () => dropUser(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('common.delete') },
                      ),
                    default: () => t('db.dropUserConfirm', { name: row.name }),
                  },
                ),
              ],
            },
          )
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
])

const tableColumns = computed<DataTableColumns<DatabaseTable>>(() => [
  { title: t('db.tableName'), key: 'name', minWidth: 200 },
  { title: t('db.rows'), key: 'rows', width: 120 },
  { title: t('db.size'), key: 'sizeBytes', width: 120, render: (row) => formatBytes(row.sizeBytes) },
])

/** Cột của bảng kết quả truy vấn, dựng theo tên cột máy chủ trả về. */
const resultColumns = computed<DataTableColumns<Record<string, unknown>>>(() =>
  (console_.value.result?.columns ?? []).map((name, index) => ({
    title: name,
    key: String(index),
    minWidth: 120,
    ellipsis: { tooltip: true },
    render: (row: Record<string, unknown>) => {
      const value = row[String(index)]
      // NULL và chuỗi rỗng nhìn giống hệt nhau nếu cả hai đều hiện ra trống.
      if (value === null || value === undefined) {
        return h(NText, { depth: 3, italic: true }, { default: () => 'NULL' })
      }
      return String(value)
    },
  })),
)

const resultRows = computed(() =>
  (console_.value.result?.rows ?? []).map((row) => {
    const record: Record<string, unknown> = {}
    row.forEach((value, index) => (record[String(index)] = value))
    return record
  }),
)
</script>

<template>
  <NCard :title="t('db.title')" size="small">
    <template #header-extra>
      <NSpace :size="8">
        <NButton size="small" :loading="loading" @click="loadServers">
          {{ t('common.refresh') }}
        </NButton>
        <NButton v-if="auth.canWrite" size="small" type="primary" @click="openServerCreate">
          {{ t('db.addServer') }}
        </NButton>
      </NSpace>
    </template>

    <NTabs type="line" animated>
      <NTabPane name="servers" :tab="t('db.servers')">
        <NDataTable
          :columns="serverColumns"
          :data="servers"
          :loading="loading"
          :row-key="(row: DatabaseServer) => row.id"
          size="small"
        >
          <template #empty>
            <NEmpty :description="t('db.noServers')" />
          </template>
        </NDataTable>
      </NTabPane>

      <NTabPane name="databases" :tab="t('db.databases')">
        <NSpace vertical :size="12">
          <NSpace :size="8" align="center">
            <NSelect
              v-model:value="selectedId"
              :options="serverOptions"
              :placeholder="t('db.selectServer')"
              style="width: 280px"
              size="small"
            />
            <NButton
              v-if="auth.canWrite && selected?.online"
              size="small"
              type="primary"
              @click="dbCreator.show = true"
            >
              {{ t('db.createDatabase') }}
            </NButton>
          </NSpace>

          <NAlert v-if="selected && !selected.online" type="error" :bordered="false">
            {{ t('db.serverOffline', { error: selected.error ?? '' }) }}
          </NAlert>

          <NDataTable
            :columns="databaseColumns"
            :data="databases"
            :row-key="(row: DatabaseInfo) => row.name"
            size="small"
          >
            <template #empty>
              <NEmpty :description="t('db.noDatabases')" />
            </template>
          </NDataTable>
        </NSpace>
      </NTabPane>

      <NTabPane name="users" :tab="t('db.users')">
        <NSpace vertical :size="12">
          <NSpace :size="8" align="center">
            <NSelect
              v-model:value="selectedId"
              :options="serverOptions"
              :placeholder="t('db.selectServer')"
              style="width: 280px"
              size="small"
            />
            <NButton
              v-if="auth.canWrite && selected?.online"
              size="small"
              type="primary"
              @click="openUserCreate"
            >
              {{ t('db.createUser') }}
            </NButton>
          </NSpace>

          <NDataTable
            :columns="userColumns"
            :data="users"
            :row-key="(row: DatabaseUser) => `${row.name}@${row.host ?? ''}`"
            size="small"
          >
            <template #empty>
              <NEmpty :description="t('db.noUsers')" />
            </template>
          </NDataTable>
        </NSpace>
      </NTabPane>

      <NTabPane v-if="auth.canWrite" name="console" :tab="t('db.console')">
        <NSpace vertical :size="12">
          <NSpace :size="8" align="center">
            <NSelect
              v-model:value="selectedId"
              :options="serverOptions"
              style="width: 260px"
              size="small"
            />
            <NSelect
              v-model:value="console_.database"
              :options="databaseOptions"
              :placeholder="t('db.selectDatabase')"
              style="width: 220px"
              size="small"
            />
            <NButton
              size="small"
              type="primary"
              :loading="console_.running"
              :disabled="!console_.statement.trim() || !selected?.online"
              @click="runQuery"
            >
              {{ t('db.run') }}
            </NButton>
          </NSpace>

          <NInput
            v-model:value="console_.statement"
            type="textarea"
            :rows="6"
            class="sql-input"
            :placeholder="t('db.statementPlaceholder')"
          />

          <NAlert type="warning" :bordered="false">
            {{ t('db.consoleWarning') }}
          </NAlert>

          <NAlert v-if="console_.error" type="error" :bordered="false">
            {{ console_.error }}
          </NAlert>

          <template v-if="console_.result">
            <NSpace :size="10" align="center">
              <NText depth="3" style="font-size: 12px">
                {{ t('db.queryStats', {
                  rows: console_.result.rows.length,
                  ms: console_.result.durationMs,
                }) }}
              </NText>
              <NTag v-if="console_.result.truncated" size="tiny" type="warning" :bordered="false">
                {{ t('db.truncated') }}
              </NTag>
              <NText v-if="console_.result.rowsAffected > 0" depth="3" style="font-size: 12px">
                {{ t('db.rowsAffected', { count: console_.result.rowsAffected }) }}
              </NText>
            </NSpace>

            <NDataTable
              v-if="console_.result.columns.length > 0"
              :columns="resultColumns"
              :data="resultRows"
              size="small"
              max-height="45vh"
              :scroll-x="Math.max(600, console_.result.columns.length * 160)"
            />
          </template>
        </NSpace>
      </NTabPane>
    </NTabs>
  </NCard>

  <NModal
    v-model:show="serverEditor.show"
    preset="card"
    :title="serverEditor.form.id === 0 ? t('db.addServer') : t('db.editServer')"
    style="width: 92vw; max-width: 560px"
  >
    <NForm @submit.prevent="saveServer">
      <NFormItem :label="t('db.serverName')">
        <NInput v-model:value="serverEditor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('db.kind')">
        <NSelect v-model:value="serverEditor.form.kind" :options="kindOptions" />
      </NFormItem>

      <NSpace :size="12">
        <NFormItem :label="t('db.hostLabel')">
          <NInput v-model:value="serverEditor.form.host" />
        </NFormItem>
        <NFormItem :label="t('db.port')">
          <NInputNumber v-model:value="serverEditor.form.port" :min="1" :max="65535" />
        </NFormItem>
      </NSpace>

      <NFormItem :label="t('db.user')" :feedback="t('db.userHelp')">
        <NInput v-model:value="serverEditor.form.user" />
      </NFormItem>

      <NFormItem
        :label="t('db.password')"
        :feedback="serverEditor.form.id === 0 ? '' : t('db.passwordKeepHelp')"
      >
        <NInput
          v-model:value="serverEditor.form.password"
          type="password"
          show-password-on="click"
        />
      </NFormItem>

      <NFormItem :label="t('db.remark')">
        <NInput v-model:value="serverEditor.form.remark" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="serverEditor.saving"
        :disabled="!canSaveServer"
      >
        {{ t('db.saveAndTest') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="dbCreator.show"
    preset="card"
    :title="t('db.createDatabase')"
    style="width: 92vw; max-width: 440px"
  >
    <NForm @submit.prevent="createDatabase">
      <NFormItem :label="t('db.databaseName')" :feedback="t('db.nameHelp')">
        <NInput v-model:value="dbCreator.name" autofocus />
      </NFormItem>
      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="dbCreator.saving"
        :disabled="!dbCreator.name.trim()"
      >
        {{ t('common.create') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="userEditor.show"
    preset="card"
    :title="userEditor.mode === 'create' ? t('db.createUser') : t('db.changePassword')"
    style="width: 92vw; max-width: 480px"
  >
    <NForm @submit.prevent="saveUser">
      <NFormItem :label="t('db.userName')">
        <NInput
          v-model:value="userEditor.form.name"
          :disabled="userEditor.mode === 'password'"
          autofocus
        />
      </NFormItem>

      <NFormItem :label="t('db.password')">
        <NInput v-model:value="userEditor.form.password" type="password" show-password-on="click" />
      </NFormItem>

      <NFormItem
        v-if="userEditor.mode === 'create'"
        :label="t('db.grantOn')"
        :feedback="t('db.grantHelp')"
      >
        <NSelect
          v-model:value="userEditor.form.database"
          :options="databaseOptions"
          clearable
          :placeholder="t('db.grantNone')"
        />
      </NFormItem>

      <NFormItem v-if="!isPostgres" :label="t('db.host')" :feedback="t('db.hostPatternHelp')">
        <NInput v-model:value="userEditor.form.host" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="userEditor.saving"
        :disabled="!userEditor.form.name.trim() || !userEditor.form.password"
      >
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="tableViewer.show"
    preset="card"
    :title="t('db.tablesOf', { name: tableViewer.database })"
    style="width: 94vw; max-width: 800px"
  >
    <NDataTable
      :columns="tableColumns"
      :data="tables"
      :row-key="(row: DatabaseTable) => row.name"
      size="small"
      max-height="60vh"
    >
      <template #empty>
        <NEmpty :description="t('db.noTables')" />
      </template>
    </NDataTable>
  </NModal>
</template>

<style scoped>
.sql-input :deep(textarea) {
  font-family: var(--sp-font-mono);
  font-size: 13px;
}
</style>
