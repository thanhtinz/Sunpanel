<script setup lang="ts">
import { computed, h, ref, type Component } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  NAvatar,
  NButton,
  NDropdown,
  NIcon,
  NLayout,
  NLayoutHeader,
  NLayoutSider,
  NMenu,
  NSpace,
  NText,
  type MenuOption,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { SUPPORTED_LOCALES, setLocale, type Locale } from '@/locales'
import IconGauge from '@/components/icons/IconGauge.vue'
import IconFolder from '@/components/icons/IconFolder.vue'
import IconUsers from '@/components/icons/IconUsers.vue'
import IconLogs from '@/components/icons/IconLogs.vue'
import IconMoon from '@/components/icons/IconMoon.vue'
import IconSun from '@/components/icons/IconSun.vue'
import IconGlobe from '@/components/icons/IconGlobe.vue'

const { t, locale } = useI18n()
const auth = useAuthStore()
const theme = useThemeStore()
const route = useRoute()
const router = useRouter()

const collapsed = ref(false)

function renderIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

function renderLink(name: string, label: string) {
  return () => h(RouterLink, { to: { name } }, { default: () => label })
}

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = [
    {
      label: renderLink('dashboard', t('nav.dashboard')),
      key: 'dashboard',
      icon: renderIcon(IconGauge),
    },
    {
      label: renderLink('files', t('nav.files')),
      key: 'files',
      icon: renderIcon(IconFolder),
    },
  ]

  // Người dùng không phải quản trị viên không nên thấy các mục họ không vào được.
  if (auth.isAdmin) {
    options.push(
      { label: renderLink('users', t('nav.users')), key: 'users', icon: renderIcon(IconUsers) },
      { label: renderLink('audit', t('nav.audit')), key: 'audit', icon: renderIcon(IconLogs) },
    )
  }

  return options
})

const userMenuOptions = computed(() => [
  { label: t('nav.profile'), key: 'profile' },
  { type: 'divider', key: 'divider' },
  { label: t('nav.logout'), key: 'logout' },
])

const languageOptions = computed(() =>
  SUPPORTED_LOCALES.map((l) => ({
    label: l.label,
    key: l.value,
    // Đánh dấu ngôn ngữ đang chọn để người dùng biết mình đang ở đâu.
    disabled: l.value === locale.value,
  })),
)

async function handleUserMenu(key: string): Promise<void> {
  if (key === 'logout') {
    await auth.logout()
    await router.push({ name: 'login' })
    return
  }
  await router.push({ name: key })
}

async function handleLanguage(key: string): Promise<void> {
  setLocale(key as Locale)
  // Ghi nhớ lựa chọn lên máy chủ để lần đăng nhập sau trên máy khác vẫn đúng ngôn ngữ.
  await auth.updatePreferences({ language: key }).catch(() => undefined)
}

function toggleTheme(): void {
  theme.setMode(theme.isDark ? 'light' : 'dark')
}
</script>

<template>
  <NLayout has-sider position="absolute">
    <NLayoutSider
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="232"
      :collapsed="collapsed"
      show-trigger
      @collapse="collapsed = true"
      @expand="collapsed = false"
    >
      <div class="brand">
        <div class="brand-mark">☀</div>
        <NText v-if="!collapsed" strong class="brand-name">{{ t('app.name') }}</NText>
      </div>

      <NMenu
        :options="menuOptions"
        :value="String(route.name ?? '')"
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="20"
      />
    </NLayoutSider>

    <NLayout>
      <NLayoutHeader bordered class="header">
        <NText strong class="page-title">{{ t(`nav.${String(route.name ?? 'dashboard')}`) }}</NText>

        <NSpace align="center" :size="8">
          <NDropdown :options="languageOptions" @select="handleLanguage">
            <NButton quaternary circle :title="t('profile.language')">
              <template #icon><NIcon><IconGlobe /></NIcon></template>
            </NButton>
          </NDropdown>

          <NButton quaternary circle :title="t('profile.theme')" @click="toggleTheme">
            <template #icon>
              <NIcon><IconMoon v-if="!theme.isDark" /><IconSun v-else /></NIcon>
            </template>
          </NButton>

          <NDropdown :options="userMenuOptions" @select="handleUserMenu">
            <NSpace align="center" :size="8" class="user-chip">
              <NAvatar round size="small" :style="{ background: '#f0a500' }">
                {{ auth.user?.username.charAt(0).toUpperCase() }}
              </NAvatar>
              <NText>{{ auth.user?.username }}</NText>
            </NSpace>
          </NDropdown>
        </NSpace>
      </NLayoutHeader>

      <NLayout :native-scrollbar="false" class="content">
        <div class="content-inner">
          <RouterView />
        </div>
      </NLayout>
    </NLayout>
  </NLayout>
</template>

<style scoped>
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 20px;
  overflow: hidden;
}

.brand-mark {
  flex: none;
  font-size: 22px;
  line-height: 1;
  color: #f0a500;
}

.brand-name {
  font-size: 17px;
  white-space: nowrap;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 20px;
}

.page-title {
  font-size: 16px;
}

.user-chip {
  cursor: pointer;
}

.content {
  height: calc(100vh - 56px);
}

/* Padding phải nằm ở lớp trong: đặt trên NLayout thì vùng cuộn vẫn rộng 100%
   cộng thêm padding, khiến nội dung tràn ngang và bị cắt ở mép phải. */
.content-inner {
  padding: 20px;
}
</style>
