<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NSpace, NTabPane, NTabs, NTag, NText } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { request } from '@/api/client'
import TerminalPane from '@/components/TerminalPane.vue'

const { t } = useI18n()

type Status = 'connecting' | 'connected' | 'disconnected'

interface Tab {
  id: number
  status: Status
}

const available = ref(true)
const tabs = ref<Tab[]>([{ id: 1, status: 'connecting' }])
const activeTab = ref('1')
const panes = ref<Record<number, InstanceType<typeof TerminalPane> | null>>({})

let nextId = 2

onMounted(async () => {
  try {
    const status = await request<{ available: boolean }>('/api/v1/terminal/status')
    available.value = status.available
  } catch {
    available.value = false
  }
})

function addTab(): void {
  const tab = { id: nextId++, status: 'connecting' as Status }
  tabs.value.push(tab)
  activeTab.value = String(tab.id)
}

function closeTab(name: string | number): void {
  const id = Number(name)
  tabs.value = tabs.value.filter((tab) => tab.id !== id)
  delete panes.value[id]

  // Luôn giữ ít nhất một tab để trang không thành khoảng trống vô nghĩa.
  if (tabs.value.length === 0) {
    addTab()
    return
  }
  if (activeTab.value === name) {
    activeTab.value = String(tabs.value[tabs.value.length - 1]?.id ?? '')
  }
}

function setStatus(id: number, status: Status): void {
  const tab = tabs.value.find((item) => item.id === id)
  if (tab) tab.status = status
}

function currentTab(): Tab | undefined {
  return tabs.value.find((tab) => String(tab.id) === activeTab.value)
}

function statusType(status: Status): 'success' | 'warning' | 'error' {
  if (status === 'connected') return 'success'
  return status === 'connecting' ? 'warning' : 'error'
}
</script>

<template>
  <!-- Thẻ không đặt tiêu đề: thanh tiêu đề của panel đã nói đây là trang nào,
       nhắc lại lần nữa chỉ tốn một dòng ngay chỗ cần cho nội dung. -->
  <NCard size="small" class="terminal-card">
    <!-- Naive UI bỏ luôn thanh tiêu đề khi thẻ không có tiêu đề, kéo theo cả
         nhóm nút bên phải. Khe tiêu đề rỗng giữ thanh đó lại cho nhóm nút. -->
    <template #header><span /></template>

    <template #header-extra>
      <NSpace align="center" :size="8">
        <NTag v-if="currentTab()" :type="statusType(currentTab()!.status)" size="small" round>
          {{ t(`terminal.${currentTab()!.status}`) }}
        </NTag>
        <NButton
          v-if="currentTab()?.status === 'disconnected'"
          size="small"
          type="primary"
          @click="panes[currentTab()!.id]?.reconnect()"
        >
          {{ t('terminal.reconnect') }}
        </NButton>
        <NButton size="small" @click="panes[currentTab()?.id ?? 0]?.clear()">
          {{ t('terminal.clear') }}
        </NButton>
      </NSpace>
    </template>

    <NAlert v-if="!available" type="warning" :bordered="false">
      {{ t('terminal.not_supported') }}
    </NAlert>

    <template v-else>
      <NText depth="3" class="hint">{{ t('terminal.hint') }}</NText>

      <NTabs
        v-model:value="activeTab"
        type="card"
        addable
        closable
        size="small"
        @add="addTab"
        @close="closeTab"
      >
        <NTabPane
          v-for="tab in tabs"
          :key="tab.id"
          :name="String(tab.id)"
          :tab="`#${tab.id}`"
          display-directive="show"
        >
          <div class="pane-shell">
            <TerminalPane
              :ref="(el) => (panes[tab.id] = el as InstanceType<typeof TerminalPane>)"
              @status="(s) => setStatus(tab.id, s)"
            />
          </div>
        </NTabPane>
      </NTabs>
    </template>
  </NCard>
</template>

<style scoped>
.hint {
  display: block;
  margin-bottom: 10px;
  font-size: 12px;
}

.pane-shell {
  height: calc(100vh - 250px);
  min-height: 340px;
}
</style>
