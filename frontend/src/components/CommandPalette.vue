<script setup lang="ts">
import { computed, onMounted, onUnmounted, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { NInput, NModal, NText } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { SUPPORTED_LOCALES, setLocale, type Locale } from '@/locales'

const { t, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

const show = ref(false)
const keyword = ref('')
const active = ref(0)
const input = ref<InstanceType<typeof NInput> | null>(null)

interface Command {
  /** label là nhãn hiển thị, đã dịch sẵn. */
  label: string
  /** group là nhóm hiển thị trong danh sách. */
  group: string
  /** run thực hiện lệnh; luôn bất đồng bộ để mọi lệnh xử lý như nhau. */
  run: () => Promise<void>
  /** keywords là các từ phụ giúp tìm ra lệnh, ví dụ tên viết không dấu. */
  keywords?: string
}

/** Các trang điều hướng được, lọc theo quyền của người dùng hiện tại. */
const navigationCommands = computed<Command[]>(() => {
  const routes: { name: string; adminOnly?: boolean; writeOnly?: boolean; keywords?: string }[] = [
    { name: 'dashboard', keywords: 'tong quan overview home' },
    { name: 'files', keywords: 'tep file quan ly tep' },
    { name: 'disk', keywords: 'dung luong o dia disk usage du day dia phan vung' },
    { name: 'apps', keywords: 'cho ung dung app store cai dat wordpress gitea n8n docker compose' },
    { name: 'websites', keywords: 'trang web site domain ten mien ssl chung chi nginx vhost' },
    { name: 'databases', keywords: 'co so du lieu database mysql mariadb postgres sql truy van' },
    { name: 'backups', keywords: 'sao luu backup khoi phuc restore s3 webdav lich' },
    { name: 'docker', keywords: 'container image volume' },
    { name: 'services', keywords: 'dich vu systemd service' },
    { name: 'processes', keywords: 'tien trinh process top htop cpu ram kill cong dang mo port listen' },
    { name: 'cron', keywords: 'tac vu dinh ky schedule job' },
    { name: 'alerts', keywords: 'canh bao alert thong bao telegram email webhook nguong' },
    { name: 'firewall', keywords: 'tuong lua port cong' },
    { name: 'terminal', writeOnly: true, keywords: 'shell console bash' },
    { name: 'system-logs', writeOnly: true, keywords: 'nhat ky he thong log syslog nginx error tail theo doi' },
    { name: 'plugins', keywords: 'plugin tien ich mo rong addon' },
    { name: 'nodes', adminOnly: true, keywords: 'node may chu da node agent tu xa remote cluster' },
    { name: 'users', adminOnly: true, keywords: 'nguoi dung user account' },
    { name: 'audit', adminOnly: true, keywords: 'nhat ky log audit' },
    { name: 'accounts', adminOnly: true, keywords: 'tai khoan may chu linux user ssh key khoa cong khai useradd sudo' },
    { name: 'settings', adminOnly: true, keywords: 'cai dat setting cau hinh cong port duong dan bi mat https ssl ip khoi dong lai' },
    { name: 'profile', keywords: 'tai khoan account 2fa mat khau' },
  ]

  return routes
    .filter((route) => {
      if (route.adminOnly) return auth.isAdmin
      if (route.writeOnly) return auth.canWrite
      return true
    })
    .map((route) => ({
      label: t(`nav.${route.name}`),
      group: t('palette.navigate'),
      keywords: route.keywords,
      run: async () => {
        await router.push({ name: route.name })
      },
    }))
})

/** Các hành động nhanh không phải điều hướng. */
const actionCommands = computed<Command[]>(() => {
  const commands: Command[] = [
    {
      label: theme.isDark ? t('profile.themes.light') : t('profile.themes.dark'),
      group: t('palette.actions'),
      keywords: 'theme giao dien sang toi dark light',
      run: async () => theme.setMode(theme.isDark ? 'light' : 'dark'),
    },
  ]

  for (const item of SUPPORTED_LOCALES) {
    if (item.value === locale.value) continue
    commands.push({
      label: `${t('profile.language')}: ${item.label}`,
      group: t('palette.actions'),
      keywords: 'language ngon ngu locale',
      run: async () => {
        setLocale(item.value as Locale)
        await auth.updatePreferences({ language: item.value }).catch(() => undefined)
      },
    })
  }

  commands.push({
    label: t('nav.logout'),
    group: t('palette.actions'),
    keywords: 'dang xuat logout thoat',
    run: async () => {
      await auth.logout()
      await router.push({ name: 'login' })
    },
  })

  return commands
})

/**
 * Chuẩn hóa chuỗi để tìm kiếm bỏ qua dấu tiếng Việt.
 *
 * Người dùng gõ nhanh thường không bỏ dấu; "tuong lua" phải tìm ra "Tường lửa".
 */
function normalize(value: string): string {
  return value
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/đ/g, 'd')
}

const results = computed(() => {
  const all = [...navigationCommands.value, ...actionCommands.value]
  const query = normalize(keyword.value.trim())
  if (!query) return all

  return all.filter(
    (command) =>
      normalize(command.label).includes(query) ||
      normalize(command.keywords ?? '').includes(query),
  )
})

// Danh sách đổi thì lựa chọn cũ không còn ý nghĩa.
watch(results, () => (active.value = 0))

function open(): void {
  show.value = true
  keyword.value = ''
  active.value = 0
  void nextTick(() => input.value?.focus())
}

async function runActive(): Promise<void> {
  const command = results.value[active.value]
  if (!command) return

  show.value = false
  await command.run()
}

function move(delta: number): void {
  const count = results.value.length
  if (count === 0) return
  // Cộng thêm count trước khi chia dư để chỉ số âm vẫn quay vòng đúng.
  active.value = (active.value + delta + count) % count
}

function onKeydown(event: KeyboardEvent): void {
  // Ctrl+K trên Windows/Linux, Cmd+K trên macOS.
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    show.value ? (show.value = false) : open()
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

defineExpose({ open })
</script>

<template>
  <NModal
    v-model:show="show"
    :mask-closable="true"
    transform-origin="center"
    class="palette-modal"
  >
    <div class="palette" @keydown.down.prevent="move(1)" @keydown.up.prevent="move(-1)">
      <NInput
        ref="input"
        v-model:value="keyword"
        size="large"
        :placeholder="t('palette.placeholder')"
        :bordered="false"
        class="palette-input"
        @keydown.enter.prevent="runActive"
        @keydown.esc="show = false"
      />

      <div v-if="results.length > 0" class="palette-list">
        <button
          v-for="(command, index) in results"
          :key="command.group + command.label"
          type="button"
          class="palette-item"
          :class="{ active: index === active }"
          @mouseenter="active = index"
          @click="runActive"
        >
          <span class="palette-label">{{ command.label }}</span>
          <NText depth="3" class="palette-group">{{ command.group }}</NText>
        </button>
      </div>

      <div v-else class="palette-empty">
        <NText depth="3">{{ t('palette.noResults') }}</NText>
      </div>

      <div class="palette-footer">
        <NText depth="3">{{ t('palette.hint') }}</NText>
      </div>
    </div>
  </NModal>
</template>

<style scoped>
.palette {
  width: min(90vw, 560px);
  overflow: hidden;
  background: var(--sp-bg-elevated);
  border: 1px solid var(--sp-border);
  border-radius: 10px;
  box-shadow: 0 18px 48px rgb(0 0 0 / 22%);
}

.palette-input {
  --n-height: 52px;
  border-bottom: 1px solid var(--sp-border);
}

.palette-list {
  max-height: min(50vh, 340px);
  overflow-y: auto;
  padding: 6px;
}

.palette-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 10px 12px;
  color: inherit;
  font: inherit;
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
}

.palette-item.active {
  background: rgb(24 160 88 / 12%);
}

.palette-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.palette-group {
  flex: none;
  margin-left: 12px;
  font-size: 12px;
}

.palette-empty {
  padding: 28px;
  text-align: center;
}

.palette-footer {
  padding: 8px 14px;
  font-size: 12px;
  border-top: 1px solid var(--sp-border);
}
</style>
