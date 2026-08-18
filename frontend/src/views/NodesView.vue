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
import { ApiError, nodeApi, type Node } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const nodes = ref<Node[]>([])
const loading = ref(false)

const emptyNode = () => ({
  id: 0,
  name: '',
  address: 'https://',
  token: '',
  skipVerify: true,
  remark: '',
})

const editor = ref({ show: false, saving: false, form: emptyNode() })
const details = ref({ show: false, node: null as Node | null })

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
      id: node.id,
      name: node.name,
      address: node.address,
      // Token không bao giờ được trả về từ máy chủ; để trống nghĩa là giữ nguyên.
      token: '',
      skipVerify: node.skipVerify,
      remark: node.remark,
    },
  }
}

const canSave = computed(() => {
  const form = editor.value.form
  if (!form.name.trim() || !/^https?:\/\/.+/.test(form.address.trim())) return false
  return form.id !== 0 || form.token.trim().length > 0
})

async function save(): Promise<void> {
  const form = editor.value.form
  editor.value.saving = true
  try {
    const payload = {
      name: form.name.trim(),
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

function openDetails(node: Node): void {
  details.value = { show: true, node }
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
  { title: t('node.name'), key: 'name', width: 170 },
  {
    title: t('node.address'),
    key: 'address',
    minWidth: 200,
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
    title: t('node.agentVersion'),
    key: 'agentVersion',
    width: 150,
    render: (row) =>
      row.agentVersion
        ? h('code', { style: 'font-size:12px' }, row.agentVersion)
        : h(NText, { depth: 3 }, { default: () => '—' }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            h(
              NButton,
              { size: 'tiny', quaternary: true, disabled: !row.online, onClick: () => openDetails(row) },
              { default: () => t('node.details') },
            ),
            auth.isAdmin
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
                  { default: () => t('common.edit') },
                )
              : null,
            auth.isAdmin
              ? h(
                  NPopconfirm,
                  { onPositiveClick: () => remove(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('node.remove') },
                      ),
                    default: () => t('node.removeConfirm', { name: row.name }),
                  },
                )
              : null,
          ],
        },
      ),
  },
])
</script>

<template>
  <NCard :title="t('node.title')" size="small">
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
      <NAlert type="info" :bordered="false" style="margin-bottom: 14px">
        {{ t('node.setupHint') }}
      </NAlert>

      <NFormItem :label="t('node.name')">
        <NInput v-model:value="editor.form.name" autofocus />
      </NFormItem>

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
        {{ details.node.system.cpuModel }} ({{ details.node.system.cpuCores }})
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.memory')">
        {{ formatBytes(details.node.system.totalMemory) }}
      </NDescriptionsItem>
      <NDescriptionsItem v-if="details.node.system.virtualization" :label="t('node.virtualization')">
        {{ details.node.system.virtualization }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.agentUptime')">
        {{ formatUptime(details.node.uptime) }}
      </NDescriptionsItem>
      <NDescriptionsItem :label="t('node.lastSeen')">
        {{ formatDateTime(details.node.lastSeenAt) }}
      </NDescriptionsItem>
    </NDescriptions>
  </NModal>
</template>
