<script setup lang="ts">
import { computed } from 'vue'
import {
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  NNotificationProvider,
  dateEnUS,
  dateViVN,
  enUS,
  viVN,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'

const theme = useThemeStore()
const { locale } = useI18n()

// Naive UI có bộ ngôn ngữ riêng cho các thành phần dựng sẵn (bộ chọn ngày,
// phân trang…), phải đồng bộ với ngôn ngữ của ứng dụng.
const naiveLocale = computed(() => (locale.value === 'en' ? enUS : viVN))
const naiveDateLocale = computed(() => (locale.value === 'en' ? dateEnUS : dateViVN))
</script>

<template>
  <NConfigProvider
    :theme="theme.naiveTheme"
    :locale="naiveLocale"
    :date-locale="naiveDateLocale"
    inline-theme-disabled
  >
    <NLoadingBarProvider>
      <NDialogProvider>
        <NNotificationProvider>
          <NMessageProvider>
            <RouterView />
          </NMessageProvider>
        </NNotificationProvider>
      </NDialogProvider>
    </NLoadingBarProvider>
  </NConfigProvider>
</template>
