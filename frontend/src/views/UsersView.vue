<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NText,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, userApi, type Role, type User } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { SUPPORTED_LOCALES, translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const users = ref<User[]>([])
const loading = ref(false)

const createModal = ref(false)
const creating = ref(false)
const form = ref({ username: '', password: '', email: '', role: 'readonly' as Role, language: 'vi' })

const resetModal = ref(false)
const resetTarget = ref<User | null>(null)
const resetPassword = ref('')

const roleOptions = computed(() =>
  (['admin', 'operator', 'readonly'] as const).map((r) => ({
    label: t(`users.roles.${r}`),
    value: r,
  })),
)
const languageOptions = SUPPORTED_LOCALES.map((l) => ({ label: l.label, value: l.value }))

const roleTagType = { admin: 'error', operator: 'warning', readonly: 'default' } as const

onMounted(load)

async function load(): Promise<void> {
  loading.value = true
  try {
    users.value = await userApi.list()
  } catch (err) {
    reportError(err)
  } finally {
    loading.value = false
  }
}

function reportError(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function createUser(): Promise<void> {
  creating.value = true
  try {
    await userApi.create(form.value)
    createModal.value = false
    form.value = { username: '', password: '', email: '', role: 'readonly', language: 'vi' }
    message.success(t('common.success'))
    await load()
  } catch (err) {
    reportError(err)
  } finally {
    creating.value = false
  }
}

async function toggleActive(user: User): Promise<void> {
  try {
    await userApi.update(user.id, { active: !user.active })
    await load()
  } catch (err) {
    reportError(err)
  }
}

async function removeUser(user: User): Promise<void> {
  try {
    await userApi.remove(user.id)
    message.success(t('common.success'))
    await load()
  } catch (err) {
    reportError(err)
  }
}

function openReset(user: User): void {
  resetTarget.value = user
  resetPassword.value = ''
  resetModal.value = true
}

async function submitReset(): Promise<void> {
  if (!resetTarget.value) return

  try {
    await userApi.resetPassword(resetTarget.value.id, resetPassword.value)
    resetModal.value = false
    message.success(t('common.success'))
  } catch (err) {
    reportError(err)
  }
}

const columns = computed<DataTableColumns<User>>(() => [
  { title: t('users.username'), key: 'username' },
  { title: t('users.email'), key: 'email', render: (row) => row.email || '—' },
  {
    title: t('users.role'),
    key: 'role',
    render: (row) =>
      h(NTag, { size: 'small', type: roleTagType[row.role] }, { default: () => t(`users.roles.${row.role}`) }),
  },
  {
    title: t('users.status'),
    key: 'active',
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: row.active ? 'success' : 'default' },
        { default: () => (row.active ? t('users.active') : t('users.inactive')) },
      ),
  },
  {
    title: t('users.twoFactor'),
    key: 'totpEnabled',
    render: (row) =>
      h(
        NText,
        { depth: row.totpEnabled ? 1 : 3 },
        { default: () => (row.totpEnabled ? t('common.enabled') : t('common.disabled')) },
      ),
  },
  {
    title: t('users.lastLogin'),
    key: 'lastLoginAt',
    render: (row) => formatDateTime(row.lastLoginAt),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(
            NButton,
            { size: 'tiny', quaternary: true, onClick: () => openReset(row) },
            { default: () => t('users.resetPassword') },
          ),
          h(
            NButton,
            { size: 'tiny', quaternary: true, onClick: () => toggleActive(row) },
            { default: () => (row.active ? t('users.inactive') : t('users.active')) },
          ),
          // Không hiện nút xóa cho chính mình — máy chủ cũng từ chối, nhưng để
          // người dùng bấm rồi mới báo lỗi là thiết kế tồi.
          row.id === auth.user?.id
            ? null
            : h(
                NPopconfirm,
                { onPositiveClick: () => removeUser(row) },
                {
                  trigger: () =>
                    h(
                      NButton,
                      { size: 'tiny', quaternary: true, type: 'error' },
                      { default: () => t('common.delete') },
                    ),
                  default: () => t('users.deleteConfirm', { name: row.username }),
                },
              ),
        ],
      }),
  },
])
</script>

<template>
  <NCard :title="t('users.title')" size="small">
    <template #header-extra>
      <NButton type="primary" size="small" @click="createModal = true">
        {{ t('common.create') }}
      </NButton>
    </template>

    <NDataTable
      :columns="columns"
      :data="users"
      :loading="loading"
      :row-key="(row: User) => row.id"
      size="small"
    />
  </NCard>

  <NModal
    v-model:show="createModal"
    preset="card"
    :title="t('users.createTitle')"
    style="max-width: 420px"
  >
    <NForm @submit.prevent="createUser">
      <NFormItem :label="t('users.username')">
        <NInput v-model:value="form.username" autocomplete="off" />
      </NFormItem>
      <NFormItem :label="t('login.password')">
        <NInput
          v-model:value="form.password"
          type="password"
          show-password-on="mousedown"
          autocomplete="new-password"
        />
      </NFormItem>
      <NFormItem :label="t('users.email')">
        <NInput v-model:value="form.email" autocomplete="off" />
      </NFormItem>
      <NFormItem :label="t('users.role')">
        <NSelect v-model:value="form.role" :options="roleOptions" />
      </NFormItem>
      <NFormItem :label="t('profile.language')">
        <NSelect v-model:value="form.language" :options="languageOptions" />
      </NFormItem>
      <NText depth="3" style="font-size: 12px">{{ t(`users.roleHints.${form.role}`) }}</NText>

      <NButton
        type="primary"
        block
        attr-type="submit"
        style="margin-top: 16px"
        :loading="creating"
        :disabled="!form.username || !form.password"
      >
        {{ t('common.create') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="resetModal"
    preset="card"
    :title="t('users.resetPassword')"
    style="max-width: 380px"
  >
    <NSpace vertical :size="14">
      <NText depth="3">{{ resetTarget?.username }}</NText>
      <NFormItem :label="t('users.newPassword')" :show-feedback="false">
        <NInput v-model:value="resetPassword" type="password" show-password-on="mousedown" />
      </NFormItem>
      <NButton type="primary" block :disabled="!resetPassword" @click="submitReset">
        {{ t('common.confirm') }}
      </NButton>
    </NSpace>
  </NModal>
</template>
