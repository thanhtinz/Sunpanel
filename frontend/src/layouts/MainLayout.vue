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
  useMessage,
  NModal,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { install, installPrompt, platform, standalone } from '@/pwa'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { SUPPORTED_LOCALES, setLocale, type Locale } from '@/locales'
import { useBreakpoint } from '@/composables/useBreakpoint'
import IconGauge from '@/components/icons/IconGauge.vue'
import IconFolder from '@/components/icons/IconFolder.vue'
import IconTerminal from '@/components/icons/IconTerminal.vue'
import IconServices from '@/components/icons/IconServices.vue'
import IconPulse from '@/components/icons/IconPulse.vue'
import IconSliders from '@/components/icons/IconSliders.vue'
import IconKey from '@/components/icons/IconKey.vue'
import IconDisk from '@/components/icons/IconDisk.vue'
import IconClock from '@/components/icons/IconClock.vue'
import IconBan from '@/components/icons/IconBan.vue'
import IconChecklist from '@/components/icons/IconChecklist.vue'
import IconShield from '@/components/icons/IconShield.vue'
import IconDocker from '@/components/icons/IconDocker.vue'
import IconSite from '@/components/icons/IconSite.vue'
import IconApps from '@/components/icons/IconApps.vue'
import IconDatabase from '@/components/icons/IconDatabase.vue'
import IconBackup from '@/components/icons/IconBackup.vue'
import IconBell from '@/components/icons/IconBell.vue'
import IconNodes from '@/components/icons/IconNodes.vue'
import IconPlugin from '@/components/icons/IconPlugin.vue'
import IconMenu from '@/components/icons/IconMenu.vue'
import IconSearch from '@/components/icons/IconSearch.vue'
import CommandPalette from '@/components/CommandPalette.vue'
import IconUsers from '@/components/icons/IconUsers.vue'
import IconLogs from '@/components/icons/IconLogs.vue'
import IconMoon from '@/components/icons/IconMoon.vue'
import IconSun from '@/components/icons/IconSun.vue'
import BrandLogo from '@/components/BrandLogo.vue'
import IconGlobe from '@/components/icons/IconGlobe.vue'

const { t, locale } = useI18n()
const message = useMessage()
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

/** Nhãn nhóm trong menu.
 *
 * Mười lăm mục xếp phẳng là một bức tường chữ; chia nhóm cho biết mục nào thuộc
 * loại việc nào, nên tìm bằng mắt nhanh hơn hẳn. */
function renderGroupLabel(label: string) {
  return () => h('span', { class: 'nav-group' }, label)
}

const menuOptions = computed<MenuOption[]>(() => {
  const item = (name: string, icon: Component): MenuOption => ({
    label: renderLink(name, t(`nav.${name}`)),
    key: name,
    icon: renderIcon(icon),
  })

  const options: MenuOption[] = [
    item('dashboard', IconGauge),

    {
      type: 'group',
      key: 'group-workload',
      label: renderGroupLabel(t('nav.groupWorkload')),
      children: [
        item('apps', IconApps),
        item('websites', IconSite),
        item('databases', IconDatabase),
        item('docker', IconDocker),
      ],
    },

    {
      type: 'group',
      key: 'group-system',
      label: renderGroupLabel(t('nav.groupSystem')),
      children: [
        item('files', IconFolder),
        item('disk', IconDisk),
        item('services', IconServices),
        item('processes', IconPulse),
        item('cron', IconClock),
        item('firewall', IconShield),
        ...(auth.canWrite ? [item('system-logs', IconLogs), item('terminal', IconTerminal)] : []),
      ],
    },

    {
      type: 'group',
      key: 'group-operations',
      label: renderGroupLabel(t('nav.groupOperations')),
      children: [item('backups', IconBackup), item('uptime', IconPulse), item('alerts', IconBell)],
    },
  ]

  // Người dùng không phải quản trị viên không nên thấy các mục họ không vào được.
  if (auth.isAdmin) {
    options.push({
      type: 'group',
      key: 'group-admin',
      label: renderGroupLabel(t('nav.groupAdmin')),
      children: [
        item('nodes', IconNodes),
        item('plugins', IconPlugin),
        item('users', IconUsers),
        item('accounts', IconKey),
        item('audit', IconLogs),
        item('health', IconChecklist),
        item('security', IconBan),
        item('settings', IconSliders),
      ],
    })
  }

  return options
})

/** Trang con của plugin vẫn phải làm sáng mục Plugin ở menu. */
const activeMenuKey = computed(() => {
  const name = String(route.name ?? '')
  return name === 'plugin-detail' ? 'plugins' : name
})

/** Một dòng nói trang này dùng để làm gì.
 *
 * Panel có nhiều trang mà tên gọi không tự giải thích ("Node", "Plugin"), và
 * dòng này là chỗ rẻ nhất để trả lời trước khi người dùng phải đoán. */
const pageKey = computed(() => String(route.name ?? 'dashboard'))
const pageTitle = computed(() => t(`nav.${pageKey.value}`))
const pageLead = computed(() => {
  const key = `lead.${pageKey.value}`
  const text = t(key)
  // vue-i18n trả về chính khóa khi chưa có bản dịch; đừng hiện khóa ra màn hình.
  return text === key ? '' : text
})

const userMenuOptions = computed(() => [
  { label: t('nav.profile'), key: 'profile' },
  // Đã chạy dạng ứng dụng rồi thì không mời cài nữa.
  ...(standalone.value ? [] : [{ label: t('app.install'), key: 'install' }]),
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
  if (key === 'install') {
    await installApp()
    return
  }
  await router.push({ name: key })
}

const installHelp = ref(false)

/**
 * Cài panel thành ứng dụng.
 *
 * Có sự kiện mời cài thì gọi thẳng hộp thoại của trình duyệt; không có thì chỉ
 * còn cách hướng dẫn — và đó là trường hợp của mọi iPhone.
 */
async function installApp(): Promise<void> {
  if (installPrompt.value) {
    if (await install()) message.success(t('app.installed'))
    return
  }
  installHelp.value = true
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
      <div class="sider-inner">
        <RouterLink :to="{ name: 'dashboard' }" class="brand">
          <BrandLogo :size="26" />
          <span v-if="!collapsed" class="brand-name">
            <span class="brand-sun">Sun</span>Panel
          </span>
        </RouterLink>

        <div class="sider-nav">
          <NMenu
            :options="menuOptions"
            :value="activeMenuKey"
            :collapsed="collapsed"
            :collapsed-width="64"
            :collapsed-icon-size="20"
            :indent="18"
          />
        </div>

      </div>
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
          <div class="page-heading">
            <div class="page-title">{{ pageTitle }}</div>
            <div v-if="pageLead && !isMobile" class="page-lead">{{ pageLead }}</div>
          </div>
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
          <RouterView v-slot="{ Component }">
            <Transition name="sp-fade" mode="out-in">
              <component :is="Component" />
            </Transition>
          </RouterView>
        </div>
      </NLayout>
    </NLayout>
  </NLayout>

  <CommandPalette ref="palette" />

  <NModal
    v-model:show="installHelp"
    preset="card"
    :title="t('app.install')"
    style="width: 92vw; max-width: 460px"
  >
    <NSpace vertical :size="10">
      <NText>{{ t(`app.installHint.${platform()}`) }}</NText>
      <NText depth="3" style="font-size: 12px">{{ t('app.installNote') }}</NText>
    </NSpace>
  </NModal>

  <NDrawer v-model:show="drawerOpen" :width="260" placement="left">
    <NDrawerContent :native-scrollbar="false" body-content-style="padding: 0">
      <div class="sider-inner">
        <div class="brand">
          <BrandLogo :size="26" />
          <span class="brand-name">
            <span class="brand-sun">Sun</span>Panel
          </span>
        </div>
        <div class="sider-nav">
          <NMenu :options="menuOptions" :value="activeMenuKey" :indent="18" />
        </div>
      </div>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
/* Cột menu chia ba phần theo chiều dọc: thương hiệu, danh sách cuộn được, và
   thanh trạng thái luôn dính đáy. Thiếu bố cục này thì thanh trạng thái trôi
   theo danh sách và biến mất khi menu dài. */
.sider-inner {
  display: flex;
  height: 100%;
  flex-direction: column;
}

.sider-nav {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 8px 8px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 18px;
  overflow: hidden;
  color: inherit;
  text-decoration: none;
}

.brand-sun {
  color: var(--sp-sun);
}

.brand-name {
  font-size: 16px;
  font-weight: 650;
  letter-spacing: -0.01em;
  white-space: nowrap;
  color: var(--sp-text);
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  height: 60px;
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

.page-heading {
  min-width: 0;
}

.page-title {
  overflow: hidden;
  font-size: 16px;
  font-weight: 650;
  line-height: 1.25;
  letter-spacing: -0.01em;
  color: var(--sp-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-lead {
  overflow: hidden;
  font-size: 12px;
  line-height: 1.35;
  color: var(--sp-text-muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-chip {
  padding: 3px 8px 3px 3px;
  border-radius: 999px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.user-chip:hover {
  background: var(--sp-surface-sunken);
}

.content {
  height: calc(100vh - 60px);
}

/* Padding phải nằm ở lớp trong: đặt trên NLayout thì vùng cuộn vẫn rộng 100%
   cộng thêm padding, khiến nội dung tràn ngang và bị cắt ở mép phải. */
.content-inner {
  padding: 20px;
}

@media (max-width: 900px) {
  .header {
    height: 56px;
    padding: 0 12px;
  }

  .content {
    height: calc(100vh - 56px);
  }

  .content-inner {
    padding: 12px;
  }
}
</style>
