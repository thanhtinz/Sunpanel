<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NModal,
  NSpace,
  NSwitch,
  NSelect,
  NText,
  useMessage,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, settingsApi, type PanelSettingsInfo } from '@/api'
import { translateError } from '@/locales'

const { t } = useI18n()
const message = useMessage()

const form = ref<PanelSettingsInfo | null>(null)
const loading = ref(false)
const saving = ref(false)

// Danh sách IP hiển thị dạng mỗi dòng một mục: dán từ tài liệu hay từ tường lửa
// ra đều là từng dòng, còn dấu phẩy thì phải tự sửa lại bằng tay.
const allowedIps = ref('')
const trustedProxies = ref('')

/** Hộp thoại khởi động lại, kèm địa chỉ panel sau khi cấu hình mới có hiệu lực. */
const restart = ref({ show: false, url: '', counting: false })

const logLevels = ['debug', 'info', 'warn', 'error'].map((value) => ({ label: value, value }))

onMounted(load)

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const data = await settingsApi.get()
    form.value = data
    allowedIps.value = data.allowedIps.join('\n')
    trustedProxies.value = data.trustedProxies.join('\n')
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/**
 * Địa chỉ panel sau khi lưu, dựng từ chính thanh địa chỉ của trình duyệt.
 *
 * Máy chủ chỉ biết nó lắng nghe trên 0.0.0.0 — một địa chỉ không gõ được vào
 * trình duyệt. Tên miền người dùng đang dùng để vào panel mới là thứ đúng.
 */
const newUrl = computed(() => {
  if (!form.value) return ''
  const scheme = form.value.tlsEnabled ? 'https' : 'http'
  return `${scheme}://${location.hostname}:${form.value.port}/${form.value.entryPath}/`
})

async function generateEntryPath(): Promise<void> {
  if (!form.value) return
  try {
    form.value.entryPath = (await settingsApi.entryPath()).entryPath
  } catch (err) {
    report(err)
  }
}

async function save(): Promise<void> {
  if (!form.value) return

  saving.value = true
  try {
    const payload = {
      ...form.value,
      allowedIps: splitLines(allowedIps.value),
      trustedProxies: splitLines(trustedProxies.value),
    }
    const result = await settingsApi.update(payload)
    form.value = result
    allowedIps.value = result.allowedIps.join('\n')
    trustedProxies.value = result.trustedProxies.join('\n')
    message.success(t('settings.saved'))
  } catch (err) {
    report(err)
  } finally {
    saving.value = false
  }
}

function splitLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
}

function openRestart(): void {
  restart.value = { show: true, url: newUrl.value, counting: false }
}

/**
 * Khởi động lại rồi tự đưa người dùng sang địa chỉ mới.
 *
 * Panel tắt ngay sau khi trả lời, nên trang hiện tại chắc chắn mất kết nối. Chờ
 * vài giây cho tiến trình mới lên rồi chuyển thẳng sang địa chỉ mới, thay vì để
 * người dùng nhìn một trang lỗi và tự đoán chuyện gì đã xảy ra.
 */
async function confirmRestart(): Promise<void> {
  restart.value.counting = true
  try {
    await settingsApi.restart()
  } catch (err) {
    report(err)
    restart.value.counting = false
    return
  }

  setTimeout(() => {
    location.href = restart.value.url
  }, 4000)
}
</script>

<template>
  <NSpace vertical :size="16">
    <NAlert v-if="form?.pendingRestart" type="warning" :bordered="false">
      <NSpace align="center" :size="12">
        <span>{{ t('settings.pendingRestart') }}</span>
        <NButton size="small" type="warning" :disabled="!form?.restartSupported" @click="openRestart">
          {{ t('settings.restart') }}
        </NButton>
        <NText v-if="!form?.restartSupported" depth="3">{{ t('settings.restartManual') }}</NText>
      </NSpace>
    </NAlert>

    <NCard size="small" :title="t('settings.access')">
      <NForm v-if="form" label-placement="top">
        <NGrid :cols="24" :x-gap="16" responsive="screen" item-responsive>
          <NGridItem span="24 m:8">
            <NFormItem :label="t('settings.host')" :feedback="t('settings.hostHelp')">
              <NInput v-model:value="form.host" placeholder="0.0.0.0" />
            </NFormItem>
          </NGridItem>

          <NGridItem span="24 m:8">
            <NFormItem :label="t('settings.port')">
              <NInputNumber v-model:value="form.port" :min="1" :max="65535" style="width: 100%" />
            </NFormItem>
          </NGridItem>

          <NGridItem span="24 m:8">
            <NFormItem :label="t('settings.entryPath')" :feedback="t('settings.entryPathHelp')">
              <NSpace :size="6" :wrap="false" style="width: 100%">
                <NInput v-model:value="form.entryPath" />
                <NButton @click="generateEntryPath">{{ t('settings.generate') }}</NButton>
              </NSpace>
            </NFormItem>
          </NGridItem>
        </NGrid>

        <NFormItem :label="t('settings.tls')" :feedback="t('settings.tlsHelp')">
          <NSwitch v-model:value="form.tlsEnabled" />
        </NFormItem>

        <NGrid v-if="form.tlsEnabled" :cols="24" :x-gap="16" responsive="screen" item-responsive>
          <NGridItem span="24 m:12">
            <NFormItem :label="t('settings.tlsCert')">
              <NInput v-model:value="form.tlsCertFile" placeholder="/opt/sunpanel/certs/panel.crt" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:12">
            <NFormItem :label="t('settings.tlsKey')">
              <NInput v-model:value="form.tlsKeyFile" placeholder="/opt/sunpanel/certs/panel.key" />
            </NFormItem>
          </NGridItem>
        </NGrid>

        <NText depth="3" style="font-size: 13px">
          {{ t('settings.currentUrl') }}: <code>{{ newUrl }}</code>
        </NText>
      </NForm>
    </NCard>

    <NCard size="small" :title="t('settings.security')">
      <NForm v-if="form" label-placement="top">
        <NGrid :cols="24" :x-gap="16" responsive="screen" item-responsive>
          <NGridItem span="24 m:6">
            <NFormItem :label="t('settings.accessTtl')" :feedback="t('settings.durationHelp')">
              <NInput v-model:value="form.accessTokenTtl" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:6">
            <NFormItem :label="t('settings.refreshTtl')">
              <NInput v-model:value="form.refreshTokenTtl" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:6">
            <NFormItem :label="t('settings.maxAttempts')">
              <NInputNumber v-model:value="form.maxLoginAttempts" :min="1" style="width: 100%" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:6">
            <NFormItem :label="t('settings.lockout')">
              <NInput v-model:value="form.lockoutDuration" />
            </NFormItem>
          </NGridItem>
        </NGrid>

        <NGrid :cols="24" :x-gap="16" responsive="screen" item-responsive>
          <NGridItem span="24 m:12">
            <NFormItem :label="t('settings.allowedIps')" :feedback="t('settings.allowedIpsHelp')">
              <NInput v-model:value="allowedIps" type="textarea" :rows="4" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:12">
            <NFormItem :label="t('settings.trustedProxies')" :feedback="t('settings.trustedProxiesHelp')">
              <NInput v-model:value="trustedProxies" type="textarea" :rows="4" />
            </NFormItem>
          </NGridItem>
        </NGrid>
      </NForm>
    </NCard>

    <NCard size="small" :title="t('settings.system')">
      <NForm v-if="form" label-placement="top">
        <NGrid :cols="24" :x-gap="16" responsive="screen" item-responsive>
          <NGridItem span="24 m:8">
            <NFormItem :label="t('settings.monitorInterval')" :feedback="t('settings.durationHelp')">
              <NInput v-model:value="form.monitorInterval" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:8">
            <NFormItem :label="t('settings.monitorRetention')">
              <NInput v-model:value="form.monitorRetention" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="24 m:8">
            <NFormItem :label="t('settings.logLevel')">
              <NSelect v-model:value="form.logLevel" :options="logLevels" />
            </NFormItem>
          </NGridItem>
        </NGrid>

        <NSpace vertical :size="4">
          <NText depth="3" style="font-size: 13px">
            {{ t('settings.configPath') }}: <code>{{ form.configPath }}</code>
          </NText>
          <NText depth="3" style="font-size: 13px">
            {{ t('settings.dataDir') }}: <code>{{ form.dataDir }}</code> ·
            {{ t('settings.fileRoot') }}: <code>{{ form.fileRoot }}</code>
          </NText>
          <NText depth="3" style="font-size: 13px">{{ t('settings.readOnlyHelp') }}</NText>
        </NSpace>
      </NForm>
    </NCard>

    <NSpace justify="end">
      <NButton :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
      <NButton type="primary" :loading="saving" :disabled="!form" @click="save">
        {{ t('common.save') }}
      </NButton>
    </NSpace>
  </NSpace>

  <NModal
    v-model:show="restart.show"
    preset="card"
    :title="t('settings.restart')"
    style="width: 92vw; max-width: 480px"
  >
    <NSpace vertical :size="14">
      <NText>{{ t('settings.restartConfirm') }}</NText>
      <NText depth="3" style="font-size: 13px">
        {{ t('settings.newUrl') }}: <code>{{ restart.url }}</code>
      </NText>
      <NAlert v-if="restart.counting" type="info" :bordered="false">
        {{ t('settings.restarting') }}
      </NAlert>
      <NButton type="warning" block :loading="restart.counting" @click="confirmRestart">
        {{ t('settings.restart') }}
      </NButton>
    </NSpace>
  </NModal>
</template>
