<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NForm,
  NFormItem,
  NGi,
  NGrid,
  NEmpty,
  NInput,
  NInputNumber,
  NList,
  NListItem,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  NThing,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, apiKeyApi, authApi, type ApiKey, type Session } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { SUPPORTED_LOCALES, setLocale, translateError, type Locale } from '@/locales'
import { formatDateTime } from '@/utils/format'
import QrCode from '@/components/QrCode.vue'

const { t, locale } = useI18n()
const auth = useAuthStore()
const theme = useThemeStore()
const message = useMessage()
const router = useRouter()

const sessions = ref<Session[]>([])

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const changingPassword = ref(false)

const totpModal = ref(false)
const totpSecret = ref('')
const totpUrl = ref('')
const totpCode = ref('')
const totpBusy = ref(false)

const disableModal = ref(false)
const disablePassword = ref('')

const languageOptions = SUPPORTED_LOCALES.map((l) => ({ label: l.label, value: l.value }))
const themeOptions = computed(() =>
  (['auto', 'light', 'dark'] as const).map((v) => ({ label: t(`profile.themes.${v}`), value: v })),
)

const passwordsMatch = computed(
  () => confirmPassword.value === '' || newPassword.value === confirmPassword.value,
)
const canChangePassword = computed(
  () =>
    currentPassword.value !== '' &&
    newPassword.value !== '' &&
    newPassword.value === confirmPassword.value &&
    !changingPassword.value,
)

onMounted(loadSessions)

async function loadSessions(): Promise<void> {
  sessions.value = await authApi.sessions().catch(() => [])
}

function reportError(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function changePassword(): Promise<void> {
  if (!canChangePassword.value) return

  changingPassword.value = true
  try {
    await authApi.changePassword(currentPassword.value, newPassword.value)
    message.success(t('profile.passwordChanged'))
    // Đổi mật khẩu thu hồi mọi phiên kể cả phiên hiện tại, nên phải đăng nhập lại.
    await auth.logout()
    await router.push({ name: 'login' })
  } catch (err) {
    reportError(err)
  } finally {
    changingPassword.value = false
  }
}

async function beginTotp(): Promise<void> {
  try {
    const setup = await authApi.beginTotp()
    totpSecret.value = setup.secret
    totpUrl.value = setup.url
    totpCode.value = ''
    totpModal.value = true
  } catch (err) {
    reportError(err)
  }
}

async function confirmTotp(): Promise<void> {
  totpBusy.value = true
  try {
    await authApi.confirmTotp(totpCode.value)
    await auth.loadUser()
    totpModal.value = false
    message.success(t('profile.2faEnabled'))
  } catch (err) {
    reportError(err)
  } finally {
    totpBusy.value = false
  }
}

async function disableTotp(): Promise<void> {
  totpBusy.value = true
  try {
    await authApi.disableTotp(disablePassword.value)
    await auth.loadUser()
    disableModal.value = false
    disablePassword.value = ''
    message.success(t('profile.2faDisabled'))
  } catch (err) {
    reportError(err)
  } finally {
    totpBusy.value = false
  }
}

async function revokeSession(id: number): Promise<void> {
  try {
    await authApi.revokeSession(id)
    message.success(t('profile.revoked'))
    await loadSessions()
  } catch (err) {
    reportError(err)
  }
}

async function changeLanguage(value: Locale): Promise<void> {
  setLocale(value)
  await auth.updatePreferences({ language: value }).catch(() => undefined)
}

async function changeTheme(value: 'auto' | 'light' | 'dark'): Promise<void> {
  theme.setMode(value)
  await auth.updatePreferences({ theme: value }).catch(() => undefined)
}

async function copySecret(): Promise<void> {
  await navigator.clipboard.writeText(totpSecret.value)
  message.success(t('common.copied'))
}

// ---- Khóa API ----

const apiKeys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const keyForm = ref({ show: false, saving: false, name: '', expiresInDays: 0 })
/** Khóa vừa tạo; chỉ hiện đúng một lần vì máy chủ chỉ lưu băm của nó. */
const newKey = ref({ show: false, value: '' })

onMounted(loadKeys)

async function loadKeys(): Promise<void> {
  keysLoading.value = true
  try {
    apiKeys.value = await apiKeyApi.list()
  } catch (err) {
    reportError(err)
  } finally {
    keysLoading.value = false
  }
}

async function createKey(): Promise<void> {
  keyForm.value.saving = true
  try {
    const created = await apiKeyApi.create({
      name: keyForm.value.name.trim(),
      expiresInDays: keyForm.value.expiresInDays,
    })
    keyForm.value.show = false
    keyForm.value.name = ''
    newKey.value = { show: true, value: created.key }
    await loadKeys()
  } catch (err) {
    reportError(err)
  } finally {
    keyForm.value.saving = false
  }
}

async function toggleKey(key: ApiKey, enabled: boolean): Promise<void> {
  try {
    await apiKeyApi.setEnabled(key.id, enabled)
    await loadKeys()
  } catch (err) {
    reportError(err)
    await loadKeys()
  }
}

async function removeKey(key: ApiKey): Promise<void> {
  try {
    await apiKeyApi.remove(key.id)
    message.success(t('apikey.deleted'))
    await loadKeys()
  } catch (err) {
    reportError(err)
  }
}

async function copyKey(): Promise<void> {
  try {
    await navigator.clipboard.writeText(newKey.value.value)
    message.success(t('common.copied'))
  } catch {
    message.error(t('apikey.copyFailed'))
  }
}

const keyColumns = computed<DataTableColumns<ApiKey>>(() => [
  { title: t('apikey.name'), key: 'name', minWidth: 150 },
  {
    title: t('apikey.prefix'),
    key: 'prefix',
    width: 160,
    render: (row) => h('code', { style: 'font-size:12px' }, row.prefix + '…'),
  },
  {
    title: t('apikey.lastUsed'),
    key: 'lastUsedAt',
    minWidth: 180,
    render: (row) =>
      row.lastUsedAt
        ? `${formatDateTime(row.lastUsedAt)}${row.lastIp ? ' · ' + row.lastIp : ''}`
        : h(NText, { depth: 3 }, { default: () => t('common.never') }),
  },
  {
    title: t('apikey.expires'),
    key: 'expiresAt',
    width: 160,
    render: (row) =>
      row.expiresAt
        ? formatDateTime(row.expiresAt)
        : h(NText, { depth: 3 }, { default: () => t('apikey.noExpiry') }),
  },
  {
    title: t('apikey.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) =>
      h(NSwitch, {
        value: row.enabled,
        size: 'small',
        'onUpdate:value': (value: boolean) => toggleKey(row, value),
      }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 90,
    render: (row) =>
      h(
        NPopconfirm,
        { onPositiveClick: () => removeKey(row) },
        {
          trigger: () =>
            h(
              NButton,
              { size: 'tiny', quaternary: true, type: 'error' },
              { default: () => t('apikey.revoke') },
            ),
          default: () => t('apikey.revokeConfirm', { name: row.name }),
        },
      ),
  },
])
</script>

<template>
  <NSpace vertical :size="16">
    <NAlert v-if="auth.user?.mustChangePassword" type="warning" :bordered="false">
      {{ t('profile.mustChangePassword') }}
    </NAlert>

    <NGrid :cols="'1 1100:2'" :x-gap="16" :y-gap="16">
      <NGi>
        <NCard :title="t('profile.title')" size="small">
          <NDescriptions :column="1" label-placement="left" size="small">
            <NDescriptionsItem :label="t('users.username')">
              {{ auth.user?.username }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('users.role')">
              <NTag size="small" type="info">
                {{ t(`users.roles.${auth.user?.role ?? 'readonly'}`) }}
              </NTag>
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('users.email')">
              {{ auth.user?.email || '—' }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="t('users.lastLogin')">
              {{ formatDateTime(auth.user?.lastLoginAt) }}
            </NDescriptionsItem>
          </NDescriptions>
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('profile.preferences')" size="small">
          <NForm label-placement="left" :label-width="110">
            <NFormItem :label="t('profile.language')">
              <NSelect
                :value="locale"
                :options="languageOptions"
                @update:value="changeLanguage"
              />
            </NFormItem>
            <NFormItem :label="t('profile.theme')">
              <NSelect :value="theme.mode" :options="themeOptions" @update:value="changeTheme" />
            </NFormItem>
          </NForm>
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('profile.changePassword')" size="small">
          <NForm @submit.prevent="changePassword">
            <NFormItem :label="t('profile.currentPassword')">
              <NInput
                v-model:value="currentPassword"
                type="password"
                show-password-on="mousedown"
                autocomplete="current-password"
              />
            </NFormItem>
            <NFormItem :label="t('profile.newPassword')">
              <NInput
                v-model:value="newPassword"
                type="password"
                show-password-on="mousedown"
                autocomplete="new-password"
              />
            </NFormItem>
            <NFormItem
              :label="t('profile.confirmPassword')"
              :validation-status="passwordsMatch ? undefined : 'error'"
              :feedback="passwordsMatch ? undefined : t('profile.passwordNotMatch')"
            >
              <NInput
                v-model:value="confirmPassword"
                type="password"
                show-password-on="mousedown"
                autocomplete="new-password"
              />
            </NFormItem>
            <NButton
              type="primary"
              attr-type="submit"
              :loading="changingPassword"
              :disabled="!canChangePassword"
            >
              {{ t('profile.changePassword') }}
            </NButton>
          </NForm>
        </NCard>
      </NGi>

      <NGi>
        <NCard :title="t('profile.twoFactor')" size="small">
          <NSpace vertical :size="14">
            <NTag :type="auth.user?.totpEnabled ? 'success' : 'default'" round size="small">
              {{ auth.user?.totpEnabled ? t('profile.twoFactorOn') : t('profile.twoFactorOff') }}
            </NTag>

            <NButton v-if="!auth.user?.totpEnabled" type="primary" @click="beginTotp">
              {{ t('profile.enable2fa') }}
            </NButton>
            <NButton v-else type="error" secondary @click="disableModal = true">
              {{ t('profile.disable2fa') }}
            </NButton>
          </NSpace>
        </NCard>
      </NGi>
    </NGrid>

    <NCard :title="t('profile.sessions')" size="small">
      <NList>
        <NListItem v-for="session in sessions" :key="session.id">
          <NThing>
            <template #header>
              <NText class="sp-metric">{{ session.ip || '—' }}</NText>
            </template>
            <template #description>
              <NText depth="3" style="font-size: 12px">
                {{ session.userAgent || t('common.unknown') }}
              </NText>
            </template>
            <NText depth="3" style="font-size: 12px">
              {{ t('profile.lastUsed') }}: {{ formatDateTime(session.lastUsed) }}
            </NText>
          </NThing>
          <template #suffix>
            <NPopconfirm @positive-click="revokeSession(session.id)">
              <template #trigger>
                <NButton size="small" quaternary type="error">{{ t('profile.revoke') }}</NButton>
              </template>
              {{ t('common.confirm') }}
            </NPopconfirm>
          </template>
        </NListItem>
      </NList>
    </NCard>

    <NModal
      v-model:show="totpModal"
      preset="card"
      :title="t('profile.enable2fa')"
      style="max-width: 420px"
    >
      <NSpace vertical :size="14">
        <NText depth="3">{{ t('profile.scanQr') }}</NText>

        <!-- Mã QR được vẽ tại chỗ bằng canvas thay vì gọi dịch vụ sinh QR bên
             ngoài: khóa 2FA tuyệt đối không được rời khỏi máy này. -->
        <QrCode :value="totpUrl" />

        <NText depth="3" style="font-size: 12px">{{ t('profile.manualKey') }}</NText>
        <NInput :value="totpSecret" readonly class="sp-metric">
          <template #suffix>
            <NButton text size="tiny" @click="copySecret">{{ t('common.copy') }}</NButton>
          </template>
        </NInput>

        <NFormItem :label="t('profile.enterCode')" :show-feedback="false">
          <NInput v-model:value="totpCode" :maxlength="6" inputmode="numeric" />
        </NFormItem>

        <NButton
          type="primary"
          block
          :loading="totpBusy"
          :disabled="totpCode.length !== 6"
          @click="confirmTotp"
        >
          {{ t('common.confirm') }}
        </NButton>
      </NSpace>
    </NModal>

    <NModal
      v-model:show="disableModal"
      preset="card"
      :title="t('profile.disable2fa')"
      style="max-width: 380px"
    >
      <NSpace vertical :size="14">
        <NFormItem :label="t('profile.currentPassword')" :show-feedback="false">
          <NInput v-model:value="disablePassword" type="password" show-password-on="mousedown" />
        </NFormItem>
        <NButton
          type="error"
          block
          :loading="totpBusy"
          :disabled="disablePassword === ''"
          @click="disableTotp"
        >
          {{ t('profile.disable2fa') }}
        </NButton>
      </NSpace>
    </NModal>
    <NCard :title="t('apikey.title')" size="small">
      <template #header-extra>
        <NButton size="small" type="primary" @click="keyForm.show = true">
          {{ t('apikey.create') }}
        </NButton>
      </template>

      <NAlert type="info" :bordered="false" style="margin-bottom: 12px">
        {{ t('apikey.hint') }}
      </NAlert>

      <NDataTable
        :columns="keyColumns"
        :data="apiKeys"
        :loading="keysLoading"
        :row-key="(row: ApiKey) => row.id"
        size="small"
      >
        <template #empty>
          <NEmpty :description="t('apikey.none')" />
        </template>
      </NDataTable>
    </NCard>

    <NModal
      v-model:show="keyForm.show"
      preset="card"
      :title="t('apikey.create')"
      style="width: 92vw; max-width: 440px"
    >
      <NForm @submit.prevent="createKey">
        <NFormItem :label="t('apikey.name')" :feedback="t('apikey.nameHelp')">
          <NInput v-model:value="keyForm.name" autofocus />
        </NFormItem>
        <NFormItem :label="t('apikey.expiresInDays')" :feedback="t('apikey.expiresHelp')">
          <NInputNumber v-model:value="keyForm.expiresInDays" :min="0" style="width: 100%" />
        </NFormItem>
        <NButton
          type="primary"
          block
          attr-type="submit"
          :loading="keyForm.saving"
          :disabled="!keyForm.name.trim()"
        >
          {{ t('common.create') }}
        </NButton>
      </NForm>
    </NModal>

    <NModal
      v-model:show="newKey.show"
      preset="card"
      :title="t('apikey.created')"
      style="width: 92vw; max-width: 560px"
    >
      <NAlert type="warning" :bordered="false" style="margin-bottom: 14px">
        {{ t('apikey.showOnce') }}
      </NAlert>
      <NInput :value="newKey.value" readonly class="sp-metric" />
      <NButton type="primary" block style="margin-top: 14px" @click="copyKey">
        {{ t('common.copy') }}
      </NButton>
    </NModal>
  </NSpace>
</template>
