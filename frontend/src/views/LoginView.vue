<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NAlert,
  NButton,
  NCard,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSpace,
  NText,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api'
import BrandLogo from '@/components/BrandLogo.vue'
import { useAuthStore } from '@/stores/auth'
import { SUPPORTED_LOCALES, setLocale, translateError, type Locale } from '@/locales'

const { t, locale } = useI18n()
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const totpCode = ref('')
const totpInput = ref<InstanceType<typeof NInput> | null>(null)

const loading = ref(false)
const errorMessage = ref('')
/** Chỉ hiện ô nhập mã 2FA sau khi máy chủ cho biết tài khoản này có bật. */
const needsTotp = ref(false)

const languageOptions = SUPPORTED_LOCALES.map((l) => ({ label: l.label, value: l.value }))

const canSubmit = computed(
  () => username.value.trim() !== '' && password.value !== '' && !loading.value,
)

async function submit(): Promise<void> {
  if (!canSubmit.value) return

  loading.value = true
  errorMessage.value = ''

  try {
    await auth.login(username.value.trim(), password.value, totpCode.value || undefined)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : undefined
    await router.push(redirect ?? { name: 'dashboard' })
  } catch (err) {
    if (err instanceof ApiError && err.code === 'auth.totp_required') {
      // Mật khẩu đã đúng, chỉ còn thiếu bước hai: mở ô nhập mã và đưa con trỏ vào.
      needsTotp.value = true
      errorMessage.value = ''
      await nextTick()
      totpInput.value?.focus()
    } else if (err instanceof ApiError) {
      errorMessage.value = translateError(err.code, err.params)
      totpCode.value = ''
    } else {
      errorMessage.value = t('error.network')
    }
  } finally {
    loading.value = false
  }
}

function changeLanguage(value: Locale): void {
  setLocale(value)
}
</script>

<template>
  <div class="login-page">
    <NCard class="login-card" size="large">
      <div class="brand">
        <BrandLogo :size="44" />
        <div>
          <div class="brand-name"><span class="brand-sun">Sun</span>Panel</div>
          <NText depth="3" class="brand-tagline">{{ t('app.tagline') }}</NText>
        </div>
      </div>

      <NAlert v-if="errorMessage" type="error" class="login-alert" :bordered="false">
        {{ errorMessage }}
      </NAlert>

      <NForm @submit.prevent="submit">
        <NFormItem :label="t('login.username')" path="username">
          <NInput
            v-model:value="username"
            autofocus
            autocomplete="username"
            :placeholder="t('login.username')"
            @keydown.enter.prevent="submit"
          />
        </NFormItem>

        <NFormItem :label="t('login.password')" path="password">
          <NInput
            v-model:value="password"
            type="password"
            show-password-on="mousedown"
            autocomplete="current-password"
            :placeholder="t('login.password')"
            @keydown.enter.prevent="submit"
          />
        </NFormItem>

        <NFormItem v-if="needsTotp" :label="t('login.totpCode')" path="totpCode">
          <NInput
            ref="totpInput"
            v-model:value="totpCode"
            :placeholder="t('login.totpHint')"
            :maxlength="6"
            inputmode="numeric"
            autocomplete="one-time-code"
            @keydown.enter.prevent="submit"
          />
        </NFormItem>

        <NButton
          type="primary"
          block
          size="large"
          attr-type="submit"
          :loading="loading"
          :disabled="!canSubmit"
        >
          {{ loading ? t('login.submitting') : t('login.submit') }}
        </NButton>
      </NForm>

      <NSpace justify="center" class="login-footer">
        <NSelect
          :value="locale"
          :options="languageOptions"
          size="small"
          class="language-select"
          @update:value="changeLanguage"
        />
      </NSpace>
    </NCard>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
  /* Nền chuyển sắc nhẹ giữ được tương phản ở cả chế độ sáng lẫn tối. */
  background:
    radial-gradient(1200px 600px at 50% -10%, rgba(240, 165, 0, 0.16), transparent 60%),
    var(--sp-bg);
}

.login-card {
  width: 100%;
  max-width: 400px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.brand-name {
  font-size: 20px;
  font-weight: 600;
}

.brand-sun {
  color: var(--sp-sun);
}

.brand-tagline {
  font-size: 13px;
}

.login-alert {
  margin-bottom: 16px;
}

.login-footer {
  margin-top: 20px;
}

.language-select {
  width: 140px;
}
</style>
