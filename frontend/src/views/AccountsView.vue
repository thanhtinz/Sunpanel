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
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NSwitch,
  NTag,
  NText,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, systemAccountApi, type SshKey, type SystemAccount } from '@/api'
import { translateError } from '@/locales'

const { t } = useI18n()
const message = useMessage()

const accounts = ref<SystemAccount[]>([])
const loading = ref(false)
const available = ref(true)

// Tài khoản dịch vụ chiếm phần lớn danh sách nhưng gần như không ai đụng tới,
// nên mặc định ẩn đi; trang này mở ra để làm việc với tài khoản của người.
const showSystem = ref(false)

const editor = ref({
  show: false,
  saving: false,
  form: { name: '', comment: '', shell: '/bin/bash', password: '', createHome: true, sudo: false },
})

const password = ref({ show: false, name: '', value: '' })

const keys = ref({
  show: false,
  name: '',
  loading: false,
  items: [] as SshKey[],
  input: '',
  adding: false,
})

const visible = computed(() =>
  accounts.value.filter((account) => showSystem.value || !account.system),
)

onMounted(async () => {
  available.value = (await systemAccountApi.status().catch(() => ({ available: false }))).available
  if (available.value) await load()
})

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    accounts.value = await systemAccountApi.list()
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

async function run(operation: () => Promise<unknown>, done: string): Promise<void> {
  try {
    await operation()
    message.success(done)
    await load()
  } catch (err) {
    report(err)
  }
}

function openCreate(): void {
  editor.value = {
    show: true,
    saving: false,
    form: { name: '', comment: '', shell: '/bin/bash', password: '', createHome: true, sudo: false },
  }
}

async function create(): Promise<void> {
  editor.value.saving = true
  try {
    await systemAccountApi.create(editor.value.form)
    editor.value.show = false
    message.success(t('accounts.created'))
    await load()
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

function openPassword(account: SystemAccount): void {
  password.value = { show: true, name: account.name, value: '' }
}

async function savePassword(): Promise<void> {
  const { name, value } = password.value
  if (!value) return
  password.value.show = false
  await run(() => systemAccountApi.setPassword(name, value), t('accounts.passwordChanged'))
}

async function openKeys(account: SystemAccount): Promise<void> {
  keys.value = { show: true, name: account.name, loading: true, items: [], input: '', adding: false }
  try {
    // Phòng khi máy chủ trả null: một danh sách rỗng vẫn phải đếm được.
    keys.value.items = (await systemAccountApi.keys(account.name)) ?? []
  } catch (err) {
    report(err)
  } finally {
    keys.value.loading = false
  }
}

async function addKey(): Promise<void> {
  if (!keys.value.input.trim()) return

  keys.value.adding = true
  try {
    await systemAccountApi.addKey(keys.value.name, keys.value.input)
    keys.value.input = ''
    keys.value.items = (await systemAccountApi.keys(keys.value.name)) ?? []
    message.success(t('accounts.keyAdded'))
    await load()
  } catch (err) {
    report(err)
  } finally {
    keys.value.adding = false
  }
}

async function removeKey(key: SshKey): Promise<void> {
  try {
    await systemAccountApi.removeKey(keys.value.name, key.fingerprint)
    keys.value.items = (await systemAccountApi.keys(keys.value.name)) ?? []
    message.success(t('accounts.keyRemoved'))
    await load()
  } catch (err) {
    report(err)
  }
}

const columns = computed<DataTableColumns<SystemAccount>>(() => [
  {
    title: t('accounts.name'),
    key: 'name',
    width: 220,
    render: (row) =>
      h(NSpace, { size: 6, align: 'center', wrap: false }, {
        default: () => [
          h(NText, { strong: true }, { default: () => row.name }),
          row.sudo
            ? h(NTag, { size: 'tiny', type: 'warning', bordered: false }, { default: () => 'sudo' })
            : null,
          row.system
            ? h(NTag, { size: 'tiny', bordered: false }, { default: () => t('accounts.systemTag') })
            : null,
        ],
      }),
  },
  { title: 'UID', key: 'uid', width: 80 },
  {
    title: t('accounts.comment'),
    key: 'comment',
    ellipsis: { tooltip: true },
    render: (row) => row.comment || '—',
  },
  { title: t('accounts.shell'), key: 'shell', width: 150, ellipsis: { tooltip: true } },
  {
    title: t('accounts.login'),
    key: 'locked',
    width: 150,
    render: (row) => {
      if (row.locked) {
        return h(NTag, { size: 'small', type: 'error', bordered: false }, {
          default: () => t('accounts.locked'),
        })
      }
      // "Chưa đặt mật khẩu" khác hẳn "bị khóa": tài khoản chỉ đăng nhập bằng
      // khóa SSH là chuyện bình thường, không phải trạng thái cần sửa.
      if (row.noPassword) {
        return h(NTooltip, null, {
          trigger: () =>
            h(NTag, { size: 'small', bordered: false }, { default: () => t('accounts.keyOnly') }),
          default: () => t('accounts.keyOnlyHint'),
        })
      }
      return h(NTag, { size: 'small', type: 'success', bordered: false }, {
        default: () => t('accounts.active'),
      })
    },
  },
  {
    title: t('accounts.keys'),
    key: 'keys',
    width: 110,
    render: (row) =>
      h(NButton, { size: 'tiny', quaternary: true, onClick: () => openKeys(row) }, {
        default: () => t('accounts.keyCount', { count: row.keys }),
      }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 300,
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(NButton, { size: 'tiny', quaternary: true, onClick: () => openPassword(row) },
            { default: () => t('accounts.password') }),

          h(NPopconfirm,
            { onPositiveClick: () => run(() => systemAccountApi.setLocked(row.name, !row.locked), t('accounts.done')) },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true },
                  { default: () => (row.locked ? t('accounts.unlock') : t('accounts.lock')) }),
              default: () =>
                row.locked
                  ? t('accounts.unlockConfirm', { name: row.name })
                  : t('accounts.lockConfirm', { name: row.name }),
            }),

          h(NPopconfirm,
            { onPositiveClick: () => run(() => systemAccountApi.setSudo(row.name, !row.sudo), t('accounts.done')) },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true },
                  { default: () => (row.sudo ? t('accounts.removeSudo') : t('accounts.grantSudo')) }),
              default: () =>
                row.sudo
                  ? t('accounts.removeSudoConfirm', { name: row.name })
                  : t('accounts.grantSudoConfirm', { name: row.name }),
            }),

          h(NPopconfirm,
            { onPositiveClick: () => run(() => systemAccountApi.remove(row.name, true), t('accounts.deleted')) },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
                  { default: () => t('common.delete') }),
              default: () => t('accounts.deleteConfirm', { name: row.name }),
            }),
        ],
      }),
  },
])

const keyColumns = computed<DataTableColumns<SshKey>>(() => [
  { title: t('accounts.keyType'), key: 'type', width: 130 },
  {
    title: t('accounts.keyComment'),
    key: 'comment',
    ellipsis: { tooltip: true },
    render: (row) => row.comment || '—',
  },
  { title: t('accounts.fingerprint'), key: 'fingerprint', ellipsis: { tooltip: true } },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 100,
    render: (row) =>
      h(NPopconfirm,
        { onPositiveClick: () => removeKey(row) },
        {
          trigger: () =>
            h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
              { default: () => t('common.delete') }),
          default: () => t('accounts.keyRemoveConfirm'),
        }),
  },
])
</script>

<template>
  <NCard size="small">
    <template #header><span /></template>

    <template #header-extra>
      <NSpace align="center" :size="10">
        <NSpace align="center" :size="6">
          <NSwitch v-model:value="showSystem" size="small" />
          <NText depth="3" style="font-size: 12px">{{ t('accounts.showSystem') }}</NText>
        </NSpace>
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        <NButton size="small" type="primary" :disabled="!available" @click="openCreate">
          {{ t('accounts.create') }}
        </NButton>
      </NSpace>
    </template>

    <NAlert v-if="!available" type="warning" :bordered="false">
      {{ t('accounts.unavailable') }}
    </NAlert>

    <NDataTable
      v-else
      :columns="columns"
      :data="visible"
      :loading="loading"
      :row-key="(row: SystemAccount) => row.name"
      size="small"
      max-height="calc(100vh - 280px)"
    />
  </NCard>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="t('accounts.create')"
    style="width: 92vw; max-width: 520px"
  >
    <NForm label-placement="top" @submit.prevent="create">
      <NFormItem :label="t('accounts.name')" :feedback="t('accounts.nameHelp')">
        <NInput v-model:value="editor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('accounts.comment')">
        <NInput v-model:value="editor.form.comment" />
      </NFormItem>

      <NFormItem :label="t('accounts.shell')" :feedback="t('accounts.shellHelp')">
        <NInput v-model:value="editor.form.shell" />
      </NFormItem>

      <NFormItem :label="t('accounts.password')" :feedback="t('accounts.passwordHelp')">
        <NInput v-model:value="editor.form.password" type="password" show-password-on="click" />
      </NFormItem>

      <NSpace vertical :size="8" style="margin-bottom: 14px">
        <NCheckbox v-model:checked="editor.form.createHome">{{ t('accounts.createHome') }}</NCheckbox>
        <NCheckbox v-model:checked="editor.form.sudo">{{ t('accounts.sudo') }}</NCheckbox>
      </NSpace>

      <NButton type="primary" block attr-type="submit" :loading="editor.saving">
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="password.show"
    preset="card"
    :title="t('accounts.passwordOf', { name: password.name })"
    style="width: 92vw; max-width: 420px"
  >
    <NSpace vertical :size="14">
      <NInput
        v-model:value="password.value"
        type="password"
        show-password-on="click"
        :placeholder="t('accounts.password')"
      />
      <NButton type="primary" block :disabled="!password.value" @click="savePassword">
        {{ t('common.save') }}
      </NButton>
    </NSpace>
  </NModal>

  <NModal
    v-model:show="keys.show"
    preset="card"
    :title="t('accounts.keysOf', { name: keys.name })"
    style="width: 92vw; max-width: 860px"
  >
    <NSpace vertical :size="14">
      <NDataTable
        v-if="keys.items.length"
        :columns="keyColumns"
        :data="keys.items"
        :loading="keys.loading"
        :row-key="(row: SshKey) => row.fingerprint"
        size="small"
      />
      <NEmpty v-else :description="t('accounts.noKeys')" />

      <NInput
        v-model:value="keys.input"
        type="textarea"
        :rows="3"
        :placeholder="t('accounts.keyPlaceholder')"
      />
      <NText depth="3" style="font-size: 12px">{{ t('accounts.keyHelp') }}</NText>

      <NButton
        type="primary"
        block
        :loading="keys.adding"
        :disabled="!keys.input.trim()"
        @click="addKey"
      >
        {{ t('accounts.addKey') }}
      </NButton>
    </NSpace>
  </NModal>
</template>
