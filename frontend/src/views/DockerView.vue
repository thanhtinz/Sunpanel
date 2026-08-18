<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDropdown,
  NEmpty,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NStatistic,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  dockerApi,
  type ContainerState,
  type DockerAction,
  type DockerContainer,
  type DockerImage,
  type DockerNetwork,
  type DockerVolume,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const status = ref({ available: true, socket: '', info: null as null | Record<string, number | string> })
const containers = ref<DockerContainer[]>([])
const images = ref<DockerImage[]>([])
const volumes = ref<DockerVolume[]>([])
const networks = ref<DockerNetwork[]>([])

const tab = ref('containers')
const loading = ref(false)
const busy = ref(false)
const showAll = ref(true)

const logs = ref({ show: false, name: '', content: '', loading: false })
const pull = ref({ show: false, image: '', busy: false })

/** Trạng thái nào tô màu gì — đỏ cho container chết để mắt bắt được ngay. */
const stateTag: Record<ContainerState, 'success' | 'default' | 'warning' | 'error'> = {
  running: 'success',
  exited: 'default',
  paused: 'warning',
  created: 'default',
  restarting: 'warning',
  dead: 'error',
}

const stateLabel: Record<ContainerState, string> = {
  running: 'running',
  exited: 'exited',
  paused: 'paused',
  created: 'createdState',
  restarting: 'restarting',
  dead: 'dead',
}

onMounted(load)

/** Mã lỗi Docker có tiền tố "docker." trùng khối chuỗi giao diện, nên bảng dịch
 * dùng "dockerErr." để hai bên không đè lên nhau. */
function translateDockerError(err: unknown): string {
  if (err instanceof ApiError && err.code.startsWith('docker.')) {
    return t(err.code.replace('docker.', 'dockerErr.'), err.params)
  }
  return err instanceof ApiError ? translateError(err.code, err.params) : t('error.network')
}

function report(err: unknown): void {
  message.error(translateDockerError(err))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const state = await dockerApi.status()
    status.value = { available: state.available, socket: state.socket, info: state.info as never }
    if (!state.available) return

    // Nạp song song: bốn lời gọi độc lập, chờ tuần tự chỉ làm trang lâu hiện.
    const [c, i, v, n] = await Promise.all([
      dockerApi.containers(showAll.value),
      dockerApi.images(),
      dockerApi.volumes(),
      dockerApi.networks(),
    ])
    containers.value = c
    images.value = i
    volumes.value = v
    networks.value = n
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

async function control(container: DockerContainer, action: DockerAction): Promise<void> {
  busy.value = true
  try {
    await dockerApi.control(container.id, action)
    message.success(t('docker.done'))
    await load()
  } catch (err) {
    report(err)
  } finally {
    busy.value = false
  }
}

async function openLogs(container: DockerContainer): Promise<void> {
  logs.value = { show: true, name: container.name, content: '', loading: true }
  try {
    const result = await dockerApi.logs(container.id, 500)
    logs.value.content = result.logs.trim() || t('docker.noLogs')
  } catch (err) {
    report(err)
  } finally {
    logs.value.loading = false
  }
}

async function submitPull(): Promise<void> {
  const image = pull.value.image.trim()
  if (!image) return

  pull.value.busy = true
  try {
    await dockerApi.pullImage(image)
    message.success(t('docker.pulled'))
    pull.value.show = false
    pull.value.image = ''
    await load()
  } catch (err) {
    report(err)
  } finally {
    pull.value.busy = false
  }
}

async function removeImage(image: DockerImage): Promise<void> {
  try {
    await dockerApi.removeImage(image.id, true)
    message.success(t('docker.done'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function removeVolume(volume: DockerVolume): Promise<void> {
  try {
    await dockerApi.removeVolume(volume.name)
    message.success(t('docker.done'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function prune(): Promise<void> {
  busy.value = true
  try {
    const result = await dockerApi.prune()
    message.success(t('docker.pruned', { size: formatBytes(result.freed) }))
    await load()
  } catch (err) {
    report(err)
  } finally {
    busy.value = false
  }
}

/** Thao tác nào khả dụng với container ở trạng thái hiện tại. */
function actionsFor(row: DockerContainer) {
  const options: { label: string; key: DockerAction; disabled?: boolean }[] = []

  if (row.state === 'running') {
    options.push(
      { label: t('docker.restart'), key: 'restart' },
      { label: t('docker.pause'), key: 'pause' },
      { label: t('docker.stop'), key: 'stop', disabled: row.protected },
    )
  } else if (row.state === 'paused') {
    options.push({ label: t('docker.unpause'), key: 'unpause' })
  } else {
    options.push({ label: t('docker.start'), key: 'start' })
  }

  options.push({ label: t('docker.remove'), key: 'remove', disabled: row.protected })
  return options
}

function formatPorts(row: DockerContainer): string {
  const ports = (row.ports ?? []).filter((p) => p.publicPort > 0)
  if (ports.length === 0) return '—'
  return ports.map((p) => `${p.publicPort}→${p.privatePort}/${p.type}`).join(', ')
}

const containerColumns = computed<DataTableColumns<DockerContainer>>(() => [
  {
    title: t('docker.name'),
    key: 'name',
    render: (row) =>
      h(NSpace, { size: 6, align: 'center' }, {
        default: () => [
          h(NText, { strong: true }, { default: () => row.name }),
          row.protected
            ? h(NTooltip, null, {
                trigger: () =>
                  h(NTag, { size: 'tiny', type: 'info', bordered: false },
                    { default: () => t('docker.protected') }),
                default: () => t('docker.protectedHint'),
              })
            : null,
          row.compose
            ? h(NTag, { size: 'tiny', bordered: false }, { default: () => row.compose })
            : null,
        ],
      }),
  },
  { title: t('docker.image'), key: 'image', ellipsis: { tooltip: true } },
  {
    title: t('docker.state'),
    key: 'state',
    width: 120,
    render: (row) =>
      h(NTag, { size: 'small', type: stateTag[row.state] ?? 'default' },
        { default: () => t(`docker.${stateLabel[row.state] ?? 'createdState'}`) }),
  },
  { title: t('docker.status'), key: 'status', width: 160 },
  {
    title: t('docker.ports'),
    key: 'ports',
    width: 170,
    render: (row) => h('span', { class: 'sp-metric' }, formatPorts(row)),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 130,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => openLogs(row) },
            { default: () => t('docker.logs') }),
          auth.canWrite
            ? h(NDropdown,
                {
                  trigger: 'click',
                  options: actionsFor(row),
                  onSelect: (key: DockerAction) => control(row, key),
                },
                { default: () => h(NButton, { size: 'tiny' }, { default: () => '⋯' }) })
            : null,
        ],
      }),
  },
])

const imageColumns = computed<DataTableColumns<DockerImage>>(() => [
  {
    title: t('docker.tags'),
    key: 'tags',
    render: (row) =>
      row.dangling
        ? h(NTag, { size: 'small', type: 'warning', bordered: false },
            { default: () => t('docker.dangling') })
        : h(NSpace, { size: 4 }, {
            default: () => (row.tags ?? []).map((tag) =>
              h(NTag, { size: 'small', bordered: false }, { default: () => tag })),
          }),
  },
  { title: 'ID', key: 'id', width: 150, render: (row) => h('code', null, row.id) },
  {
    title: t('docker.size'),
    key: 'size',
    width: 120,
    render: (row) => h('span', { class: 'sp-metric' }, formatBytes(row.size)),
  },
  {
    title: t('docker.created'),
    key: 'created',
    width: 175,
    render: (row) => formatDateTime(new Date(row.created * 1000).toISOString()),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 90,
    render: (row) =>
      auth.canWrite
        ? h(NPopconfirm, { onPositiveClick: () => removeImage(row) },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
                  { default: () => t('common.delete') }),
              default: () => t('docker.removeConfirm', { name: row.tags?.[0] ?? row.id }),
            })
        : null,
  },
])

const volumeColumns = computed<DataTableColumns<DockerVolume>>(() => [
  { title: t('docker.name'), key: 'name' },
  { title: t('docker.driver'), key: 'driver', width: 110 },
  { title: t('docker.mountpoint'), key: 'mountpoint', ellipsis: { tooltip: true } },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 90,
    render: (row) =>
      auth.canWrite
        ? h(NPopconfirm, { onPositiveClick: () => removeVolume(row) },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
                  { default: () => t('common.delete') }),
              default: () => t('docker.removeConfirm', { name: row.name }),
            })
        : null,
  },
])

const networkColumns = computed<DataTableColumns<DockerNetwork>>(() => [
  { title: t('docker.name'), key: 'name' },
  { title: t('docker.driver'), key: 'driver', width: 120 },
  { title: 'Scope', key: 'scope', width: 110 },
  {
    title: t('docker.subnets'),
    key: 'subnets',
    render: (row) => h('span', { class: 'sp-metric' }, (row.subnets ?? []).join(', ') || '—'),
  },
])
</script>

<template>
  <NCard :title="t('docker.title')" size="small">
    <template #header-extra>
      <NSpace align="center" :size="10">
        <template v-if="status.available && auth.canWrite">
          <NButton size="small" @click="pull.show = true">{{ t('docker.pull') }}</NButton>
          <NPopconfirm @positive-click="prune">
            <template #trigger>
              <NButton size="small" :loading="busy">{{ t('docker.prune') }}</NButton>
            </template>
            {{ t('docker.pruneConfirm') }}
          </NPopconfirm>
        </template>
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
      </NSpace>
    </template>

    <NAlert v-if="!status.available" type="warning" :bordered="false">
      {{ t('docker.unavailableHint', { socket: status.socket }) }}
    </NAlert>

    <template v-else>
      <NSpace v-if="status.info" :size="28" class="summary">
        <NStatistic :label="t('docker.containers')">
          <span class="sp-metric">
            {{ status.info.containersRunning }} / {{ status.info.containers }}
          </span>
        </NStatistic>
        <NStatistic :label="t('docker.images')">
          <span class="sp-metric">{{ status.info.images }}</span>
        </NStatistic>
        <NStatistic :label="t('docker.diskUsed')">
          <span class="sp-metric">{{ formatBytes(Number(status.info.diskUsed)) }}</span>
          <template #suffix>
            <NText depth="3" style="font-size: 12px">
              · {{ formatBytes(Number(status.info.reclaimable)) }} {{ t('docker.reclaimable') }}
            </NText>
          </template>
        </NStatistic>
        <NStatistic label="Docker">
          <span class="sp-metric">{{ status.info.version }}</span>
        </NStatistic>
      </NSpace>

      <NTabs v-model:value="tab" type="line" animated>
        <NTabPane name="containers" :tab="t('docker.containers')">
          <NSpace justify="end" align="center" :size="8" class="tab-toolbar">
            <NSwitch v-model:value="showAll" size="small" @update:value="load" />
            <NText depth="3" style="font-size: 12px">{{ t('docker.showAll') }}</NText>
          </NSpace>
          <NDataTable
            v-if="containers.length > 0"
            :columns="containerColumns"
            :data="containers"
            :loading="loading"
            :row-key="(row: DockerContainer) => row.id"
            size="small"
          />
          <NEmpty v-else :description="t('docker.noContainers')" style="padding: 40px 0" />
        </NTabPane>

        <NTabPane name="images" :tab="t('docker.images')">
          <NDataTable
            v-if="images.length > 0"
            :columns="imageColumns"
            :data="images"
            :loading="loading"
            :row-key="(row: DockerImage) => row.id"
            size="small"
          />
          <NEmpty v-else :description="t('docker.noImages')" style="padding: 40px 0" />
        </NTabPane>

        <NTabPane name="volumes" :tab="t('docker.volumes')">
          <NDataTable
            :columns="volumeColumns"
            :data="volumes"
            :loading="loading"
            :row-key="(row: DockerVolume) => row.name"
            size="small"
          />
        </NTabPane>

        <NTabPane name="networks" :tab="t('docker.networks')">
          <NDataTable
            :columns="networkColumns"
            :data="networks"
            :loading="loading"
            :row-key="(row: DockerNetwork) => row.id"
            size="small"
          />
        </NTabPane>
      </NTabs>
    </template>
  </NCard>

  <NModal
    v-model:show="logs.show"
    preset="card"
    :title="t('docker.logsOf', { name: logs.name })"
    style="width: 92vw; max-width: 1100px"
  >
    <pre class="logs">{{ logs.loading ? t('common.loading') : logs.content }}</pre>
  </NModal>

  <NModal
    v-model:show="pull.show"
    preset="card"
    :title="t('docker.pull')"
    style="max-width: 460px"
  >
    <NSpace vertical :size="14">
      <NInput
        v-model:value="pull.image"
        :placeholder="t('docker.pullImage')"
        class="sp-metric"
        @keydown.enter="submitPull"
      />
      <NText depth="3" style="font-size: 12px">{{ t('docker.pullHint') }}</NText>
      <NButton
        type="primary"
        block
        :loading="pull.busy"
        :disabled="!pull.image.trim()"
        @click="submitPull"
      >
        {{ t('docker.pull') }}
      </NButton>
    </NSpace>
  </NModal>
</template>

<style scoped>
.summary {
  margin-bottom: 16px;
}

.tab-toolbar {
  margin-bottom: 10px;
}

.logs {
  max-height: 65vh;
  margin: 0;
  padding: 12px;
  overflow: auto;
  font-family: 'Cascadia Mono', Consolas, Menlo, 'Liberation Mono', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--sp-bg);
  border-radius: 6px;
}
</style>
