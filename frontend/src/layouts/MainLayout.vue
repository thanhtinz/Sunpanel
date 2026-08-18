<script setup lang="ts">
import { computed, h, ref, watch, type Component } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  NAvatar,
  NButton,
  NDrawer,
  NDrawerContent,
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
import { useBreakpoint } from '@/composables/useBreakpoint'
import IconGauge from '@/components/icons/IconGauge.vue'
import IconFolder from '@/components/icons/IconFolder.vue'
import IconTerminal from '@/components/icons/IconTerminal.vue'
import IconServices from '@/components/icons/IconServices.vue'
import IconClock from '@/components/icons/IconClock.vue'
import IconShield from '@/components/icons/IconShield.vue'
import IconDocker from '@/components/icons/IconDocker.vue'
import IconSite from '@/components/icons/IconSite.vue'
import IconApps from '@/components/icons/IconApps.vue'
import IconDatabase from '@/components/icons/IconDatabase.vue'
import IconBackup from '@/components/icons/IconBackup.vue'
import IconMenu from '@/components/icons/IconMenu.vue'
import IconSearch from '@/components/icons/IconSearch.vue'
import CommandPalette from '@/components/CommandPalette.vue'
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

const { isMobile } = useBreakpoint()
const palette = ref<InstanceType<typeof CommandPalette> | null>(null)
const collapsed = ref(false)
/** Trên di động, thanh điều hướng là ngăn kéo phủ lên nội dung thay vì cột cố định. */
const drawerOpen = ref(false)

// Chuyển trang thì đóng ngăn kéo, nếu không nó che mất trang vừa mở.
watch(() => route.fullPath, () => (drawerOpen.value = false))

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
    {
      label: renderLink('apps', t('nav.apps')),
      key: 'apps',
      icon: renderIcon(IconApps),
    },
    {
      label: renderLink('websites', t('nav.websites')),
      key: 'websites',
      icon: renderIcon(IconSite),
    },
    {
      label: renderLink('databases', t('nav.databases')),
      key: 'databases',
      icon: renderIcon(IconDatabase),
    },
    {
      label: renderLink('backups', t('nav.backups')),
      key: 'backups',
      icon: renderIcon(IconBackup),
    },
    {
      label: renderLink('docker', t('nav.docker')),
      key: 'docker',
      icon: renderIcon(IconDocker),
    },
    {
      label: renderLink('services', t('nav.services')),
      key: 'services',
      icon: renderIcon(IconServices),
    },
    {
      label: renderLink('cron', t('nav.cron')),
      key: 'cron',
      icon: renderIcon(IconClock),
    },
    {
      label: renderLink('firewall', t('nav.firewall')),
      key: 'firewall',
      icon: renderIcon(IconShield),
    },
  ]

  if (auth.canWrite) {
    options.push({
      label: renderLink('terminal', t('nav.terminal')),
      key: 'terminal',
      icon: renderIcon(IconTerminal),
    })
  }

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
      v-if="!isMobile"
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
        <NSpace align="center" :size="10" class="header-left">
          <NButton
            v-if="isMobile"
            quaternary
            circle
            :aria-label="t('nav.menu')"
            @click="drawerOpen = true"
          >
            <template #icon><NIcon><IconMenu /></NIcon></template>
          </NButton>
          <NText strong class="page-title">
            {{ t(`nav.${String(route.name ?? 'dashboard')}`) }}
          </NText>
        </NSpace>

        <NSpace align="center" :size="8" class="header-right">
          <NButton quaternary circle :title="t('palette.open')" @click="palette?.open()">
            <template #icon><NIcon><IconSearch /></NIcon></template>
          </NButton>

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
              <NText v-if="!isMobile">{{ auth.user?.username }}</NText>
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

  <CommandPalette ref="palette" />

  <NDrawer v-model:show="drawerOpen" :width="260" placement="left">
    <NDrawerContent :native-scrollbar="false" body-content-style="padding: 0">
      <div class="brand">
        <div class="brand-mark">☀</div>
        <NText strong class="brand-name">{{ t('app.name') }}</NText>
      </div>
      <NMenu :options="menuOptions" :value="String(route.name ?? '')" />
    </NDrawerContent>
  </NDrawer>
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
  gap: 12px;
  height: 56px;
  padding: 0 20px;
}

/* min-width: 0 là bắt buộc để phần tử con co lại được; thiếu nó, tiêu đề dài sẽ
   đẩy giãn khối cha và đè lên nhóm nút bên phải. */
.header-left {
  min-width: 0;
  flex: 1;
}

.header-right {
  flex: none;
}

.page-title {
  overflow: hidden;
  font-size: 16px;
  text-overflow: ellipsis;
  white-space: nowrap;
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

@media (max-width: 900px) {
  .header {
    padding: 0 12px;
  }

  .content-inner {
    padding: 12px;
  }
}
</style>
