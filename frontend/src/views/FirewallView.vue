<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NEmpty,
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
import {
  ApiError,
  firewallApi,
  type FirewallAction,
  type FirewallProtocol,
  type FirewallRule,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const status = ref({ backend: 'none', available: false, enabled: false })
const rules = ref<FirewallRule[]>([])
const loading = ref(false)
const busy = ref(false)

const form = ref({
  show: false,
  action: 'allow' as FirewallAction,
  protocol: 'tcp' as FirewallProtocol,
  port: '',
  source: '',
  description: '',
})

const actionOptions = computed(() =>
  (['allow', 'deny', 'reject'] as const).map((v) => ({ label: t(`firewall.${v}`), value: v })),
)
const protocolOptions = computed(() =>
  (['tcp', 'udp', 'any'] as const).map((v) => ({ label: t(`firewall.${v}`), value: v })),
)

const actionTag: Record<FirewallAction, 'success' | 'error' | 'warning'> = {
  allow: 'success',
  deny: 'error',
  reject: 'warning',
}

onMounted(load)

/** Mã lỗi tường lửa có tiền tố "firewall." trùng với khối chuỗi giao diện, nên
 * bảng dịch dùng "firewallErr." để hai bên không đè lên nhau. */
function translateFirewallError(err: unknown): string {
  if (err instanceof ApiError && err.code.startsWith('firewall.')) {
    return t(err.code.replace('firewall.', 'firewallErr.'), err.params)
  }
  return err instanceof ApiError ? translateError(err.code, err.params) : t('error.network')
}

function report(err: unknown): void {
  message.error(translateFirewallError(err))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    status.value = await firewallApi.status()
    // Không có công cụ tường lửa thì việc hỏi danh sách quy tắc chỉ tạo ra một
    // thông báo lỗi thừa; cảnh báo trên trang đã nói rõ tình trạng rồi.
    rules.value = status.value.available ? await firewallApi.rules() : []
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

async function toggleFirewall(): Promise<void> {
  busy.value = true
  try {
    const next = !status.value.enabled
    await firewallApi.setEnabled(next)
    message.success(next ? t('firewall.enabled') : t('firewall.disabled'))
    await load()
  } catch (err) {
    report(err)
  } finally {
    busy.value = false
  }
}

async function addRule(): Promise<void> {
  busy.value = true
  try {
    await firewallApi.addRule({
      action: form.value.action,
      protocol: form.value.protocol,
      port: form.value.port.trim(),
      source: form.value.source.trim(),
      description: form.value.description.trim(),
    })
    message.success(t('firewall.added'))
    form.value.show = false
    form.value.port = ''
    form.value.source = ''
    form.value.description = ''
    await load()
  } catch (err) {
    report(err)
  } finally {
    busy.value = false
  }
}

async function deleteRule(id: string): Promise<void> {
  try {
    await firewallApi.deleteRule(id)
    message.success(t('firewall.removed'))
    await load()
  } catch (err) {
    report(err)
  }
}

const columns = computed<DataTableColumns<FirewallRule>>(() => [
  {
    title: t('firewall.action'),
    key: 'action',
    width: 120,
    render: (row) =>
      h(NTag, { size: 'small', type: actionTag[row.action] },
        { default: () => t(`firewall.${row.action}`) }),
  },
  {
    title: t('firewall.port'),
    key: 'port',
    width: 150,
    render: (row) =>
      row.port
        ? h('span', { class: 'sp-metric' }, row.port)
        : h(NText, { depth: 3 }, { default: () => t('firewall.anyPort') }),
  },
  {
    title: t('firewall.protocol'),
    key: 'protocol',
    width: 120,
    render: (row) => t(`firewall.${row.protocol}`),
  },
  {
    title: t('firewall.source'),
    key: 'source',
    render: (row) =>
      row.source
        ? h('span', { class: 'sp-metric' }, row.source)
        : h(NText, { depth: 3 }, { default: () => t('firewall.anySource') }),
  },
  {
    title: 'IP',
    key: 'ipv6',
    width: 80,
    // ufw sinh một dòng IPv6 song song cho mỗi quy tắc IPv4; đánh dấu rõ để danh
    // sách không trông như bị lặp.
    render: (row) => h(NText, { depth: 3 }, { default: () => (row.ipv6 ? 'IPv6' : 'IPv4') }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 90,
    render: (row) =>
      auth.canWrite
        ? h(NPopconfirm, { onPositiveClick: () => deleteRule(row.id) },
            {
              trigger: () =>
                h(NButton, { size: 'tiny', quaternary: true, type: 'error' },
                  { default: () => t('common.delete') }),
              default: () => t('firewall.deleteConfirm'),
            })
        : null,
  },
])
</script>

<template>
  <NCard :title="t('firewall.title')" size="small">
    <template #header-extra>
      <NSpace align="center" :size="10">
        <NTag :type="status.enabled ? 'success' : 'default'" size="small" round>
          {{ status.enabled ? t('firewall.on') : t('firewall.off') }}
        </NTag>
        <NText v-if="status.available" depth="3" style="font-size: 12px">
          {{ t('firewall.backend') }}: {{ status.backend }}
        </NText>

        <template v-if="auth.canWrite && status.available">
          <NButton size="small" :loading="busy" @click="toggleFirewall">
            {{ status.enabled ? t('firewall.disable') : t('firewall.enable') }}
          </NButton>
          <NButton size="small" type="primary" @click="form.show = true">
            {{ t('firewall.addRule') }}
          </NButton>
        </template>
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
      </NSpace>
    </template>

    <NAlert v-if="!status.available" type="warning" :bordered="false">
      {{ t('firewall.unavailableHint') }}
    </NAlert>

    <template v-else>
      <NAlert v-if="!status.enabled" type="info" :bordered="false" class="notice">
        {{ t('firewall.enableWarning') }}
      </NAlert>

      <NDataTable
        v-if="rules.length > 0"
        :columns="columns"
        :data="rules"
        :loading="loading"
        :row-key="(row: FirewallRule) => row.id + (row.ipv6 ? '-v6' : '')"
        size="small"
      />
      <NEmpty v-else :description="t('firewall.noRules')" style="padding: 40px 0" />
    </template>
  </NCard>

  <NModal
    v-model:show="form.show"
    preset="card"
    :title="t('firewall.addRule')"
    style="max-width: 460px"
  >
    <NForm @submit.prevent="addRule">
      <NFormItem :label="t('firewall.action')">
        <NSelect v-model:value="form.action" :options="actionOptions" />
      </NFormItem>
      <NFormItem :label="t('firewall.protocol')">
        <NSelect v-model:value="form.protocol" :options="protocolOptions" />
      </NFormItem>
      <NFormItem :label="t('firewall.port')" :feedback="t('firewall.portHelp')">
        <NInput v-model:value="form.port" placeholder="80" class="sp-metric" />
      </NFormItem>
      <NFormItem :label="t('firewall.source')" :feedback="t('firewall.sourceHelp')">
        <NInput v-model:value="form.source" placeholder="192.168.1.0/24" class="sp-metric" />
      </NFormItem>
      <NFormItem :label="t('firewall.description')">
        <NInput v-model:value="form.description" />
      </NFormItem>

      <NText v-if="form.action !== 'allow'" depth="3" class="warn">
        {{ t('firewall.protectedHint') }}
      </NText>

      <NButton type="primary" block attr-type="submit" :loading="busy" style="margin-top: 12px">
        {{ t('firewall.addRule') }}
      </NButton>
    </NForm>
  </NModal>
</template>

<style scoped>
.notice {
  margin-bottom: 12px;
}

.warn {
  display: block;
  font-size: 12px;
  line-height: 1.5;
}
</style>
