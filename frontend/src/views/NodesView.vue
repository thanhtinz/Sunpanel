<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NRadioButton,
  NRadioGroup,
  NDivider,
  NDropdown,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  NText,
  useDialog,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  nodeApi,
  type Node,
  type NodeKind,
  type NodeSample,
  type RemoteResult,
} from '@/api'
import LineChart from '@/components/LineChart.vue'
import TerminalPane from '@/components/TerminalPane.vue'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()
const dialog = useDialog()

const nodes = ref<Node[]>([])
const loading = ref(false)

const emptyNode = () => ({
  id: 0,
  name: '',
  // Mặc định là SSH: đó là cách dùng được ngay với một VPS vừa mua về, còn
  // agent thì phải cài thêm phần mềm lên máy đích trước.
  kind: 'ssh' as NodeKind,
  address: 'https://',
  token: '',
  skipVerify: true,
  remark: '',
  host: '',
  port: 22,
  user: 'root',
  authType: 'password' as 'password' | 'key',
  secret: '',
  passphrase: '',
})

const editor = ref({ show: false, saving: false, form: emptyNode() })
const details = ref({ show: false, node: null as Node | null })
const terminal = ref({ show: false, node: null as Node | null })
const console_ = ref({
  show: false,
  node: null as Node | null,
  command: '',
  running: false,
  result: null as RemoteResult | null,
})

const kindOptions = computed(() => [
  { label: t('node.kindSsh'), value: 'ssh' },
  { label: t('node.kindAgent'), value: 'agent' },
])

const authOptions = computed(() => [
  { label: t('node.authPassword'), value: 'password' },
  { label: t('node.authKey'), value: 'key' },
])

onMounted(load)

function describeError(err: unknown): string {
  if (!(err instanceof ApiError)) return t('error.network')
  const detail = err.params?.message
  if (err.code.startsWith('node.')) {
    const base = t(err.code.replace('node.', 'nodeErr.'), err.params ?? {})
    // Ghép thêm nguyên văn lỗi mạng: "connection refused" hay "no route to host"
    // nói rõ phải sửa gì hơn hẳn một câu chung chung.
    return typeof detail === 'string' && detail !== '' ? `${base} (${detail})` : base
  }
  return translateError(err.code, err.params)
}

function report(err: unknown): void {
  message.error(describeError(err))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    nodes.value = await nodeApi.list()
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editor.value = { show: true, saving: false, form: emptyNode() }
}

function openEdit(node: Node): void {
  editor.value = {
    show: true,
    saving: false,
    form: {
      ...emptyNode(),
      id: node.id,
      name: node.name,
      kind: node.kind,
      address: node.address,
      // Token và mật khẩu không bao giờ được trả về từ máy chủ; để trống nghĩa
      // là giữ nguyên cái đã lưu.
      token: '',
      skipVerify: node.skipVerify,
      remark: node.remark,
      host: node.host ?? '',
      port: node.port ?? 22,
      user: node.user ?? 'root',
      authType: node.authType ?? 'password',
      secret: '',
    },
  }
}

const canSave = computed(() => {
  const form = editor.value.form
  if (!form.name.trim()) return false

  if (form.kind === 'ssh') {
    if (!form.host.trim() || !form.user.trim()) return false
    // Sửa một máy đã lưu thì được để trống bí mật; thêm mới thì không.
    return form.id !== 0 || form.secret.trim().length > 0
  }

  if (!/^https?:\/\/.+/.test(form.address.trim())) return false
  return form.id !== 0 || form.token.trim().length > 0
})

async function save(): Promise<void> {
  const form = editor.value.form
  editor.value.saving = true
  try {
    const payload =
      form.kind === 'ssh'
        ? {
            name: form.name.trim(),
            kind: form.kind,
            host: form.host.trim(),
            port: form.port,
            user: form.user.trim(),
            authType: form.authType,
            secret: form.secret,
            passphrase: form.passphrase,
            remark: form.remark.trim(),
          }
        : {
            name: form.name.trim(),
            kind: form.kind,
            address: form.address.trim(),
            token: form.token.trim(),
            skipVerify: form.skipVerify,
            remark: form.remark.trim(),
          }
    if (form.id === 0) {
      await nodeApi.create(payload)
      message.success(t('node.added'))
    } else {
      await nodeApi.update(form.id, payload)
      message.success(t('node.updated'))
    }
    editor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

async function remove(node: Node): Promise<void> {
  try {
    await nodeApi.remove(node.id)
    message.success(t('node.removed'))
    await load()
  } catch (err) {
    report(err)
  }
}

const history = ref<NodeSample[]>([])
const historyLoading = ref(false)

async function openDetails(node: Node): Promise<void> {
  details.value = { show: true, node }
  history.value = []
  if (node.kind !== 'ssh') return

  historyLoading.value = true
  try {
    // Lấy một mẫu ngay: biểu đồ của máy vừa thêm không phải chờ hết một chu kỳ
    // mới có điểm đầu tiên.
    await nodeApi.sample(node.id).catch(() => undefined)
    history.value = await nodeApi.history(node.id, 24)
  } catch (err) {
    report(err)
  } finally {
    historyLoading.value = false
  }
}

/** Nhãn trục hoành: giờ và phút của từng mẫu. */
const historyLabels = computed(() =>
  history.value.map((sample) =>
    new Date(sample.at).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' }),
  ),
)

const historySeries = computed(() => [
  { name: t('node.cpuPercent'), data: history.value.map((s) => s.cpu), color: '#15a34a' },
  { name: t('node.memoryPercent'), data: history.value.map((s) => s.memory), color: '#2563eb' },
  { name: t('node.diskPercent'), data: history.value.map((s) => s.disk), color: '#e8930c' },
])

function openTerminal(node: Node): void {
  terminal.value = { show: true, node }
}

function openConsole(node: Node): void {
  console_.value = { show: true, node, command: '', running: false, result: null }
}

async function runCommand(): Promise<void> {
  const state = console_.value
  if (!state.node || !state.command.trim()) return

  state.running = true
  try {
    state.result = await nodeApi.exec(state.node.id, state.command)
  } catch (err) {
    report(err)
  } finally {
    state.running = false
  }
}

/** Tên hệ điều hành đầy đủ.
 *
 * gopsutil trả platform đã kèm sẵn số phiên bản trên phần lớn distro, nên ghép
 * thêm version sẽ ra "ubuntu 24.04 24.04". */
function osLabel(system: { platform: string; version: string }): string {
  if (!system.version || system.platform.includes(system.version)) return system.platform
  return `${system.platform} ${system.version}`
}

/** Thời gian agent đã chạy, đổi từ nano giây sang chuỗi đọc được. */
function formatUptime(nanoseconds?: number): string {
  if (!nanoseconds) return '—'
  const seconds = Math.floor(nanoseconds / 1e9)
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return t('node.uptimeDays', { days, hours })
  if (hours > 0) return t('node.uptimeHours', { hours, minutes })
  return t('node.uptimeMinutes', { minutes })
}

const columns = computed<DataTableColumns<Node>>(() => [
  { title: t('node.name'), key: 'name', width: 150 },
  {
    title: t('node.kind'),
    key: 'kind',
    width: 90,
    render: (row) =>
      h(
        NTag,
        { size: 'small', bordered: false, type: row.kind === 'ssh' ? 'info' : 'default' },
        { default: () => (row.kind === 'ssh' ? t('node.kindSsh') : t('node.kindAgent')) },
      ),
  },
  {
    title: t('node.address'),
    key: 'address',
    minWidth: 190,
    render: (row) => h('code', { style: 'font-size:12px' }, row.address),
  },
  {
    title: t('node.status'),
    key: 'online',
    minWidth: 240,
    render: (row) => {
      if (!row.online) {
        return h(
          NSpace,
          { size: 6, align: 'center' },
          {
            default: () => [
              h(NTag, { size: 'small', type: 'error', bordered: false }, { default: () => t('node.offline') }),
              h(
                NText,
                { depth: 3, style: 'font-size:12px' },
                { default: () => row.lastError ?? '' },
              ),
            ],
          },
        )
      }
      return h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => t('node.online') }),
            h(
              NText,
              { depth: 3, style: 'font-size:12px' },
              { default: () => `${row.hostname} · ${row.os} · ${row.arch}` },
            ),
          ],
        },
      )
    },
  },
  {
    title: t('node.load'),
    key: 'load',
    width: 170,
    render: (row) => {
      // Máy nối bằng agent báo phiên bản agent, máy nối bằng SSH báo mức dùng
      // tài nguyên: mỗi kiểu chỉ có đúng con số mà nó đọc được.
      if (row.kind === 'agent') {
        return row.agentVersion
          ? h('code', { style: 'font-size:12px' }, row.agentVersion)
          : h(NText, { depth: 3 }, { default: () => '—' })
      }
      if (!row.online) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(
        NText,
        { depth: 3, style: 'font-size:12px' },
        {
          default: () =>
            `${t('node.loadShort')} ${row.load1?.toFixed(2) ?? '—'} · ` +
            `${formatBytes(row.memoryUsed ?? 0)}/${formatBytes(row.memoryTotal ?? 0)}`,
        },
      )
    },
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 240,
    render: (row) =>
      h(
        NSpace,
        { size: 4, wrap: false },
        {
          default: () => [
            // Hai thao tác hay dùng nhất nằm ngoài; phần còn lại vào menu, nếu
            // không cột thao tác rộng hơn cả cột địa chỉ và tràn ra ngoài bảng.
            auth.isAdmin && row.kind === 'ssh'
              ? h(
                  NButton,
                  {
                    size: 'tiny',
                    quaternary: true,
                    disabled: !row.online,
                    onClick: () => openTerminal(row),
                  },
                  { default: () => t('node.terminal') },
                )
              : null,
            auth.isAdmin && row.kind === 'ssh'
              ? h(
                  NButton,
                  {
                    size: 'tiny',
                    quaternary: true,
                    disabled: !row.online,
                    onClick: () => openConsole(row),
                  },
                  { default: () => t('node.runCommand') },
                )
              : null,
            h(
              NDropdown,
              {
                options: rowActions(row),
                trigger: 'click',
                onSelect: (key: string) => runRowAction(key, row),
              },
              {
                default: () =>
                  h(NButton, { size: 'tiny', quaternary: true }, { default: () => t('node.more') }),
              },
            ),
          ],
        },
      ),
  },
])

/** Các thao tác phụ của một máy chủ. */
function rowActions(node: Node) {
  return [
    { label: t('node.details'), key: 'details', disabled: !node.online },
    ...(auth.isAdmin
      ? [
          { label: t('common.edit'), key: 'edit' },
          { type: 'divider', key: 'ngan' },
          { label: t('node.remove'), key: 'remove' },
        ]
      : []),
  ]
}

function runRowAction(key: string, node: Node): void {
  switch (key) {
    case 'details':
      void openDetails(node)
      break
    case 'edit':
      openEdit(node)
      break
    case 'remove':
      // Gỡ máy chủ là thao tác không lấy lại được thông tin đăng nhập đã lưu,
      // nên nó vẫn phải hỏi lại một lần dù đã nằm trong menu.
      dialog.warning({
        title: t('node.remove'),
        content: t('node.removeConfirm', { name: node.name }),
        positiveText: t('node.remove'),
        negativeText: t('common.cancel'),
        onPositiveClick: () => void remove(node),
      })
      break
  }
}
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
        <NButton v-if="auth.isAdmin" size="small" type="primary" @click="openCreate">
          {{ t('node.add') }}
        </NButton>
      </NSpace>
    </template>

    <NAlert type="info" :bordered="false" style="margin-bottom: 12px">
      {{ t('node.hint') }}
    </NAlert>

    <NDataTable
      :columns="columns"
      :data="nodes"
      :loading="loading"
      :row-key="(row: Node) => row.id"
      size="small"
    >
      <template #empty>
        <NEmpty :description="t('node.none')" />
      </template>
    </NDataTable>
  </NCard>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="editor.form.id === 0 ? t('node.add') : t('node.edit')"
    style="width: 92vw; max-width: 560px"
  >
    <NForm @submit.prevent="save">
      <NFormItem :label="t('node.kind')">
        <NRadioGroup v-model:value="editor.form.kind" :disabled="editor.form.id !== 0">
          <NRadioButton v-for="option in kindOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>

      <NAlert type="info" :bordered="false" style="margin-bottom: 14px">
        {{ editor.form.kind === 'ssh' ? t('node.sshHint') : t('node.setupHint') }}
      </NAlert>

      <NFormItem :label="t('node.name')">
        <NInput v-model:value="editor.form.name" autofocus />
      </NFormItem>

      <template v-if="editor.form.kind === 'ssh'">
        <NSpace :size="12">
          <NFormItem :label="t('node.host')" :feedback="t('node.hostHelp')">
            <NInput v-model:value="editor.form.host" placeholder="203.0.113.10" style="width: 260px" />
          </NFormItem>
          <NFormItem :label="t('node.port')">
            <NInputNumber v-model:value="editor.form.port" :min="1" :max="65535" style="width: 120px" />
          </NFormItem>
        </NSpace>

        <NFormItem :label="t('node.user')">
          <NInput v-model:value="editor.form.user" placeholder="root" />
        </NFormItem>

        <NFormItem :label="t('node.authType')">
          <NSelect v-model:value="editor.form.authType" :options="authOptions" />
        </NFormItem>

        <NFormItem
          v-if="editor.form.authType === 'password'"
          :label="t('node.password')"
          :feedback="editor.form.id === 0 ? '' : t('node.secretKeepHelp')"
        >
          <NInput v-model:value="editor.form.secret" type="password" show-password-on="click" />
        </NFormItem>

        <template v-else>
          <NFormItem
            :label="t('node.privateKey')"
            :feedback="editor.form.id === 0 ? t('node.privateKeyHelp') : t('node.secretKeepHelp')"
          >
            <NInput
              v-model:value="editor.form.secret"
              type="textarea"
              :rows="4"
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
              class="sp-metric"
            />
          </NFormItem>
          <NFormItem :label="t('node.passphrase')" :feedback="t('node.passphraseHelp')">
            <NInput v-model:value="editor.form.passphrase" type="password" show-password-on="click" />
          </NFormItem>
        </template>
      </template>

      <template v-else>
        <NFormItem :label="t('node.address')" :feedback="t('node.addressHelp')">
          <NInput v-model:value="editor.form.address" placeholder="https://10.0.0.5:9528" />
        </NFormItem>

        <NFormItem
          :label="t('node.token')"
          :feedback="editor.form.id === 0 ? t('node.tokenHelp') : t('node.tokenKeepHelp')"
        >
          <NInput v-model:value="editor.form.token" type="password" show-password-on="click" />
        </NFormItem>

        <NFormItem :label="t('node.skipVerify')" :feedback="t('node.skipVerifyHelp')">
          <NSwitch v-model:value="editor.form.skipVerify" />
        </NFormItem>
      </template>

      <NFormItem :label="t('node.remark')">
        <NInput v-model:value="editor.form.remark" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="editor.saving"
        :disabled="!canSave"
      >
        {{ t('node.saveAndConnect') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="terminal.show"
    preset="card"
    :title="t('node.terminalOf', { name: terminal.node?.name ?? '' })"
    style="width: 94vw; max-width: 1000px"
  >
    <!-- Dựng lại từ đầu mỗi lần mở: một cửa sổ terminal đã đóng không nối lại
         được, và giữ nó lại chỉ để trống trơn. -->
    <TerminalPane v-if="terminal.show && terminal.node" :node-id="terminal.node.id" />
  </NModal>

  <NModal
    v-model:show="console_.show"
    preset="card"
    :title="t('node.runOn', { name: console_.node?.name ?? '' })"
    style="width: 94vw; max-width: 760px"
  >
    <NSpace vertical :size="12">
      <NInput
        v-model:value="console_.command"
        :placeholder="t('node.commandPlaceholder')"
        class="sp-metric"
        @keyup.enter="runCommand"
      />
      <NButton type="primary" :loading="console_.running" :disabled="!console_.command.trim()" @click="runCommand">
        {{ t('node.run') }}
      </NButton>

      <template v-if="console_.result">
        <NText depth="3" style="font-size: 12px">
          {{ t('node.exitCode', { code: console_.result.exitCode }) }}
        </NText>
        <pre v-if="console_.result.stdout" class="command-output">{{ console_.result.stdout }}</pre>
        <pre v-if="console_.result.stderr" class="command-output command-error">{{
          console_.result.stderr
        }}</pre>
      </template>
    </NSpace>
  </NModal>

  <NModal
    v-model:show="details.show"
    preset="card"
    :title="t('node.detailsOf', { name: details.node?.name ?? '' })"
    style="width: 92vw; max-width: 620px"
  >
    <NDescriptions v-if="details.node?.system" :column="1" label-placement="left" size="small">
      <NDescriptionsItem :label="t('node.hostname')">
        {{ details.node.system.hostname }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.osLabel')">
        {{ osLabel(details.node.system) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.kernel')">
        {{ details.node.system.kernel }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.arch')">
        {{ details.node.system.arch }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.cpu')">
        <!-- Máy nối bằng SSH không đọc được tên CPU; hiện "(4)" trơ trọi thì
             người xem tưởng dữ liệu hỏng. -->
        {{
          details.node.system.cpuModel
            ? `${details.node.system.cpuModel} (${details.node.system.cpuCores})`
            : t('node.cores', { count: details.node.system.cpuCores })
        }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.memory')">
        {{ formatBytes(details.node.system.totalMemory) }}
      </NDescriptionsItem>
      <NDescriptionsItem v-if="details.node.system.virtualization" :label="t('node.virtualization')">
        {{ details.node.system.virtualization }}
      </NDescriptionsItem>
      <NDescriptionsItem
        :label="details.node.kind === 'ssh' ? t('node.machineUptime') : t('node.agentUptime')"
      >
        {{ formatUptime(details.node.uptime) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.lastSeen')">
        {{ formatDateTime(details.node.lastSeenAt) }}
      </NDescriptionsItem>
      <NDescriptionsItem v-if="details.node.fingerprint" :label="t('node.fingerprint')">
        <NText class="sp-metric" style="font-size: 12px">{{ details.node.fingerprint }}</NText>
      </NDescriptionsItem>
    </NDescriptions>

    <template v-if="details.node?.kind === 'ssh'">
      <NDivider>{{ t('node.last24h') }}</NDivider>
      <NSpin :show="historyLoading">
        <LineChart
          v-if="history.length > 1"
          :labels="historyLabels"
          :series="historySeries"
          unit="%"
          :max="100"
          height="220px"
        />
        <NEmpty v-else :description="t('node.noHistory')" />
      </NSpin>
    </template>
  </NModal>
</template>

<style scoped>
.command-output {
  margin: 0;
  padding: 10px 12px;
  max-height: 320px;
  overflow: auto;
  font-family: var(--sp-font-mono);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--sp-surface-sunken);
  border-radius: var(--sp-radius-control);
}

.command-error {
  color: var(--sp-danger);
}
</style>
