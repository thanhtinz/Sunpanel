<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NDataTable,
  NDivider,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  NText,
  NUpload,
  useMessage,
  type UploadFileInfo,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  ApiError,
  certApi,
  websiteApi,
  type CertSource,
  type Certificate,
  type WebServerStatus,
  type Website,
  type RedirectRule,
  type WebsiteType,
} from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const status = ref<WebServerStatus>({ server: '', available: true })
const sites = ref<Website[]>([])
const certificates = ref<Certificate[]>([])
const loading = ref(false)

const emptySite = () => ({
  id: 0,
  name: '',
  domains: '',
  type: 'static' as WebsiteType,
  root: '',
  proxyTarget: '',
  phpSocket: '/run/php/php-fpm.sock',
  port: 80,
  sslPort: 443,
  sslEnabled: false,
  forceHttps: false,
  certName: '',
  extraConfig: '',
  remark: '',
  authEnabled: false,
  authUser: '',
  authPassword: '',
  denyIps: '',
  redirects: [] as RedirectRule[],
})

const emptyCert = () => ({
  name: '',
  domains: '',
  source: 'self' as CertSource,
  email: '',
  staging: true,
  autoRenew: true,
  certPem: '',
  keyPem: '',
})

/** Đọc cột JSON quy tắc chuyển hướng; dữ liệu hỏng cho ra danh sách rỗng. */
function parseRedirects(raw: string): RedirectRule[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

const editor = ref({ show: false, saving: false, form: emptySite() })
const certEditor = ref({ show: false, saving: false, form: emptyCert() })
const preview = ref({ show: false, name: '', content: '', loading: false })

onMounted(load)

function report(err: unknown): void {
  if (!(err instanceof ApiError)) {
    message.error(t('error.network'))
    return
  }
  // Mã lỗi website và chứng chỉ có tiền tố riêng để không đụng khối chuỗi giao diện.
  const prefixes: [string, string][] = [
    ['website.', 'websiteErr.'],
    ['cert.', 'certErr.'],
  ]
  for (const [code, prefix] of prefixes) {
    if (err.code.startsWith(code)) {
      message.error(t(err.code.replace(code, prefix), err.params ?? {}))
      return
    }
  }
  message.error(translateError(err.code, err.params))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const [serverStatus, siteList, certList] = await Promise.all([
      websiteApi.status(),
      websiteApi.list(),
      certApi.list(),
    ])
    status.value = serverStatus
    sites.value = siteList
    certificates.value = certList
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/** Tách chuỗi tên miền người dùng gõ; chấp nhận cả dấu phẩy lẫn xuống dòng. */
function parseDomains(value: string): string[] {
  return value
    .split(/[\s,]+/)
    .map((domain) => domain.trim())
    .filter(Boolean)
}

const siteTypeOptions = computed(() => [
  { label: t('website.typeStatic'), value: 'static' },
  { label: t('website.typeProxy'), value: 'proxy' },
  { label: t('website.typePhp'), value: 'php' },
])

const certOptions = computed(() =>
  certificates.value.map((cert) => ({
    label: `${cert.name} (${cert.domains})`,
    value: cert.name,
  })),
)

const certSourceOptions = computed(() => [
  { label: t('website.sourceSelf'), value: 'self' },
  { label: t('website.sourceAcme'), value: 'acme' },
  { label: t('website.sourceUpload'), value: 'upload' },
])

const canSaveSite = computed(() => {
  const form = editor.value.form
  if (!form.name.trim() || parseDomains(form.domains).length === 0) return false
  if (form.type === 'proxy') return form.proxyTarget.trim().length > 0
  return form.root.trim().length > 0
})

const canSaveCert = computed(() => {
  const form = certEditor.value.form
  if (!form.name.trim() || parseDomains(form.domains).length === 0) return false
  if (form.source === 'acme') return form.email.includes('@')
  if (form.source === 'upload') return form.certPem.trim() !== '' && form.keyPem.trim() !== ''
  return true
})

function openCreate(): void {
  editor.value = { show: true, saving: false, form: emptySite() }
}

function openEdit(site: Website): void {
  editor.value = {
    show: true,
    saving: false,
    form: {
      id: site.id,
      name: site.name,
      domains: site.domains.split(',').join('\n'),
      type: site.type,
      root: site.root,
      proxyTarget: site.proxyTarget,
      phpSocket: site.phpSocket || '/run/php/php-fpm.sock',
      port: site.port,
      sslPort: site.sslPort,
      sslEnabled: site.sslEnabled,
      forceHttps: site.forceHttps,
      certName: site.certName,
      extraConfig: site.extraConfig,
      remark: site.remark,
      authEnabled: site.authEnabled,
      authUser: site.authUser,
      // Mật khẩu không bao giờ được gửi ngược về giao diện; để trống nghĩa là
      // giữ nguyên cái đã lưu.
      authPassword: '',
      denyIps: site.denyIps,
      redirects: parseRedirects(site.redirects),
    },
  }
}

async function saveSite(): Promise<void> {
  const form = editor.value.form
  editor.value.saving = true
  try {
    const payload = {
      name: form.name.trim(),
      domains: parseDomains(form.domains),
      type: form.type,
      root: form.root.trim(),
      proxyTarget: form.proxyTarget.trim(),
      phpSocket: form.type === 'php' ? form.phpSocket.trim() : '',
      port: form.port,
      sslPort: form.sslPort,
      sslEnabled: form.sslEnabled,
      forceHttps: form.forceHttps,
      certName: form.sslEnabled ? form.certName : '',
      extraConfig: form.extraConfig,
      enabled: true,
      remark: form.remark.trim(),
      authEnabled: form.authEnabled,
      authUser: form.authUser.trim(),
      authPassword: form.authPassword,
      denyIps: form.denyIps.split('\n').map((line) => line.trim()).filter(Boolean),
      redirects: form.redirects.filter((rule) => rule.from.trim() && rule.to.trim()),
    }
    if (form.id === 0) {
      await websiteApi.create(payload)
      message.success(t('website.created'))
    } else {
      await websiteApi.update(form.id, payload)
      message.success(t('website.updated'))
    }
    editor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

async function toggleSite(site: Website, enabled: boolean): Promise<void> {
  try {
    await websiteApi.setEnabled(site.id, enabled)
    await load()
  } catch (err) {
    report(err)
    await load()
  }
}

/**
 * Hộp thoại triển khai mã nguồn.
 *
 * Hai đường vào vì người dùng đến từ hai hoàn cảnh: mã nguồn đang nằm trên máy
 * họ (tải lên) hoặc vừa được tải thẳng về máy chủ bằng terminal (chỉ đường dẫn).
 */
const deploy = ref({
  show: false,
  site: null as Website | null,
  file: null as File | null,
  path: '',
  clean: false,
  keepWrapper: false,
  running: false,
})

function openDeploy(site: Website): void {
  deploy.value = {
    show: true,
    site,
    file: null,
    path: '',
    clean: false,
    keepWrapper: false,
    running: false,
  }
}

// NUpload ở đây chỉ dùng để chọn tệp; việc gửi do chính hàm bên dưới làm, vì nó
// còn phải gửi kèm các lựa chọn triển khai trong cùng một yêu cầu.
function pickSource(options: { fileList: UploadFileInfo[] }): void {
  deploy.value.file = options.fileList[0]?.file ?? null
}

const canDeploy = computed(() => Boolean(deploy.value.file) || deploy.value.path.trim() !== '')

async function submitDeploy(): Promise<void> {
  const site = deploy.value.site
  if (!site || !canDeploy.value) return

  deploy.value.running = true
  try {
    const result = await websiteApi.deploySource(site.id, {
      file: deploy.value.file ?? undefined,
      path: deploy.value.path.trim(),
      clean: deploy.value.clean,
      keepWrapper: deploy.value.keepWrapper,
    })
    deploy.value.show = false
    message.success(
      result.wrapper
        ? t('website.deployedStripped', { files: result.files, wrapper: result.wrapper })
        : t('website.deployed', { files: result.files }),
    )
  } catch (err) {
    report(err)
  } finally {
    deploy.value.running = false
  }
}

async function removeSite(site: Website): Promise<void> {
  try {
    await websiteApi.remove(site.id)
    message.success(t('website.deleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function showConfig(site: Website): Promise<void> {
  preview.value = { show: true, name: site.name, content: '', loading: true }
  try {
    preview.value.content = (await websiteApi.config(site.id)).content
  } catch (err) {
    report(err)
    preview.value.show = false
  } finally {
    preview.value.loading = false
  }
}

async function reload(): Promise<void> {
  try {
    await websiteApi.reload()
    message.success(t('website.reloaded'))
  } catch (err) {
    report(err)
  }
}

function openCertCreate(): void {
  certEditor.value = { show: true, saving: false, form: emptyCert() }
}

async function saveCert(): Promise<void> {
  const form = certEditor.value.form
  certEditor.value.saving = true
  try {
    await certApi.issue({
      name: form.name.trim(),
      domains: parseDomains(form.domains),
      source: form.source,
      email: form.email.trim(),
      staging: form.staging,
      autoRenew: form.autoRenew,
      certPem: form.certPem,
      keyPem: form.keyPem,
    })
    message.success(t('website.certCreated'))
    certEditor.value.show = false
    await load()
  } catch (err) {
    report(err)
  } finally {
    certEditor.value.saving = false
  }
}

async function renewCert(cert: Certificate): Promise<void> {
  try {
    await certApi.renew(cert.name)
    message.success(t('website.certRenewed'))
    await load()
  } catch (err) {
    report(err)
  }
}

async function removeCert(cert: Certificate): Promise<void> {
  try {
    await certApi.remove(cert.name)
    message.success(t('website.certDeleted'))
    await load()
  } catch (err) {
    report(err)
  }
}

const siteColumns = computed<DataTableColumns<Website>>(() => [
  { title: t('website.name'), key: 'name', width: 160 },
  {
    title: t('website.domains'),
    key: 'domains',
    // Cột co giãn không có chiều rộng tối thiểu sẽ bị ép về 0 khi bảng chật hơn
    // tổng các cột cố định, và tên miền — thứ quan trọng nhất — biến mất hẳn.
    minWidth: 200,
    ellipsis: { tooltip: true },
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () =>
            row.domains
              .split(',')
              .map((domain) =>
                h(
                  'a',
                  {
                    href: `${row.sslEnabled ? 'https' : 'http'}://${domain}`,
                    target: '_blank',
                    rel: 'noreferrer noopener',
                    class: 'domain-link',
                  },
                  domain,
                ),
              ),
        },
      ),
  },
  {
    title: t('website.type'),
    key: 'type',
    width: 110,
    render: (row) =>
      h(NTag, { size: 'small', bordered: false }, { default: () => t(`website.type${capitalize(row.type)}`) }),
  },
  {
    title: t('website.ssl'),
    key: 'sslEnabled',
    width: 150,
    render: (row) => {
      if (!row.sslEnabled) return h(NText, { depth: 3 }, { default: () => t('website.sslOff') })
      const cert = certificates.value.find((item) => item.name === row.certName)
      // Ngày còn lại quan trọng hơn việc "có bật SSL không": chứng chỉ hết hạn
      // thì website vẫn bật SSL nhưng trình duyệt chặn hẳn.
      const expiring = cert !== undefined && cert.daysRemaining < 30
      return h(
        NTag,
        { size: 'small', type: expiring ? 'warning' : 'success', bordered: false },
        {
          default: () =>
            cert ? t('website.daysLeft', { days: cert.daysRemaining }) : row.certName,
        },
      )
    },
  },
  {
    title: t('website.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) =>
      h(NSwitch, {
        value: row.enabled,
        size: 'small',
        disabled: !auth.canWrite,
        'onUpdate:value': (value: boolean) => toggleSite(row, value),
      }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 320,
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            h(
              NButton,
              { size: 'tiny', quaternary: true, onClick: () => showConfig(row) },
              { default: () => t('website.viewConfig') },
            ),
            auth.canWrite
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openEdit(row) },
                  { default: () => t('common.edit') },
                )
              : null,
            // Website chuyển tiếp không có thư mục gốc nên không có gì để triển khai.
            auth.canWrite && row.type !== 'proxy'
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => openDeploy(row) },
                  { default: () => t('website.deploySource') },
                )
              : null,
            auth.canWrite
              ? h(
                  NPopconfirm,
                  { onPositiveClick: () => removeSite(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('common.delete') },
                      ),
                    default: () => t('website.deleteConfirm', { name: row.name }),
                  },
                )
              : null,
          ],
        },
      ),
  },
])

const certColumns = computed<DataTableColumns<Certificate>>(() => [
  { title: t('website.name'), key: 'name', width: 160 },
  { title: t('website.domains'), key: 'domains', minWidth: 200, ellipsis: { tooltip: true } },
  {
    title: t('website.source'),
    key: 'source',
    width: 130,
    render: (row) =>
      h(
        NSpace,
        { size: 4, align: 'center' },
        {
          default: () => [
            h(
              NTag,
              { size: 'tiny', bordered: false },
              { default: () => t(`website.source${capitalize(row.source)}`) },
            ),
            row.staging
              ? h(NTag, { size: 'tiny', type: 'warning', bordered: false }, { default: () => t('website.staging') })
              : null,
          ],
        },
      ),
  },
  {
    title: t('website.expires'),
    key: 'expiresAt',
    width: 220,
    render: (row) => {
      if (row.missing) {
        return h(NTag, { size: 'small', type: 'error', bordered: false }, { default: () => t('website.certMissing') })
      }
      const type = row.daysRemaining < 0 ? 'error' : row.daysRemaining < 30 ? 'warning' : 'success'
      return h(
        NSpace,
        { size: 6, align: 'center' },
        {
          default: () => [
            h(NTag, { size: 'tiny', type, bordered: false }, { default: () => t('website.daysLeft', { days: row.daysRemaining }) }),
            h(NText, { depth: 3, style: 'font-size:12px' }, { default: () => formatDateTime(row.expiresAt) }),
          ],
        },
      )
    },
  },
  {
    title: t('website.autoRenew'),
    key: 'autoRenew',
    width: 100,
    render: (row) =>
      row.autoRenew
        ? h(NTag, { size: 'tiny', type: 'success', bordered: false }, { default: () => t('common.yes') })
        : h(NText, { depth: 3 }, { default: () => t('common.no') }),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 180,
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            auth.canWrite && row.source !== 'upload'
              ? h(
                  NButton,
                  { size: 'tiny', quaternary: true, onClick: () => renewCert(row) },
                  { default: () => t('website.renew') },
                )
              : null,
            auth.canWrite
              ? h(
                  NPopconfirm,
                  { onPositiveClick: () => removeCert(row) },
                  {
                    trigger: () =>
                      h(
                        NButton,
                        { size: 'tiny', quaternary: true, type: 'error' },
                        { default: () => t('common.delete') },
                      ),
                    default: () => t('website.certDeleteConfirm', { name: row.name }),
                  },
                )
              : null,
          ],
        },
      ),
  },
])

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}
</script>

<template>
  <!-- Thẻ không đặt tiêu đề: thanh tiêu đề của panel đã nói đây là trang nào,
       nhắc lại lần nữa chỉ tốn một dòng ngay chỗ cần cho nội dung. -->
  <NCard size="small">
    <!-- Naive UI bỏ luôn thanh tiêu đề khi thẻ không có tiêu đề, kéo theo cả
         nhóm nút bên phải. Khe tiêu đề rỗng giữ thanh đó lại cho nhóm nút. -->
    <template #header><span /></template>

    <template #header-extra>
      <NSpace :size="8">
        <NButton size="small" :loading="loading" @click="load">{{ t('common.refresh') }}</NButton>
        <NButton v-if="auth.canWrite" size="small" @click="reload">
          {{ t('website.reload') }}
        </NButton>
        <NButton v-if="auth.canWrite" size="small" type="primary" @click="openCreate">
          {{ t('website.create') }}
        </NButton>
      </NSpace>
    </template>

    <NAlert
      v-if="!status.available"
      type="warning"
      :bordered="false"
      style="margin-bottom: 12px"
    >
      {{ t('website.serverUnavailable', { server: status.server }) }}
    </NAlert>

    <NTabs type="line" animated>
      <NTabPane name="sites" :tab="t('website.sites')">
        <NDataTable
          :columns="siteColumns"
          :data="sites"
          :loading="loading"
          :row-key="(row: Website) => row.id"
          size="small"
        />
      </NTabPane>

      <NTabPane name="certs" :tab="t('website.certificates')">
        <NSpace vertical :size="12">
          <NButton v-if="auth.canWrite" size="small" type="primary" @click="openCertCreate">
            {{ t('website.certCreate') }}
          </NButton>
          <NDataTable
            :columns="certColumns"
            :data="certificates"
            :loading="loading"
            :row-key="(row: Certificate) => row.id"
            size="small"
          />
        </NSpace>
      </NTabPane>
    </NTabs>
  </NCard>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="editor.form.id === 0 ? t('website.create') : t('website.edit')"
    style="width: 92vw; max-width: 680px"
  >
    <NForm @submit.prevent="saveSite">
      <NFormItem :label="t('website.name')" :feedback="t('website.nameHelp')">
        <NInput v-model:value="editor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('website.domains')" :feedback="t('website.domainsHelp')">
        <NInput v-model:value="editor.form.domains" type="textarea" :rows="2" />
      </NFormItem>

      <NFormItem :label="t('website.type')">
        <NSelect v-model:value="editor.form.type" :options="siteTypeOptions" />
      </NFormItem>

      <NFormItem
        v-if="editor.form.type === 'proxy'"
        :label="t('website.proxyTarget')"
        :feedback="t('website.proxyTargetHelp')"
      >
        <NInput v-model:value="editor.form.proxyTarget" placeholder="http://127.0.0.1:3000" />
      </NFormItem>

      <NFormItem v-else :label="t('website.root')" :feedback="t('website.rootHelp')">
        <NInput v-model:value="editor.form.root" placeholder="/www/wwwroot/example.com" />
      </NFormItem>

      <NFormItem v-if="editor.form.type === 'php'" :label="t('website.phpSocket')">
        <NInput v-model:value="editor.form.phpSocket" />
      </NFormItem>

      <NSpace :size="12">
        <NFormItem :label="t('website.port')">
          <NInputNumber v-model:value="editor.form.port" :min="1" :max="65535" />
        </NFormItem>
        <NFormItem :label="t('website.sslPort')">
          <NInputNumber v-model:value="editor.form.sslPort" :min="1" :max="65535" />
        </NFormItem>
      </NSpace>

      <NFormItem :label="t('website.ssl')">
        <NSwitch v-model:value="editor.form.sslEnabled" />
      </NFormItem>

      <template v-if="editor.form.sslEnabled">
        <NFormItem :label="t('website.certificate')" :feedback="t('website.certificateHelp')">
          <NSelect
            v-model:value="editor.form.certName"
            :options="certOptions"
            :placeholder="t('website.certSelect')"
          />
        </NFormItem>

        <NFormItem :label="t('website.forceHttps')" :feedback="t('website.forceHttpsHelp')">
          <NSwitch v-model:value="editor.form.forceHttps" />
        </NFormItem>
      </template>

      <NDivider style="margin: 4px 0 14px">{{ t('website.protection') }}</NDivider>

      <NFormItem :label="t('website.auth')" :feedback="t('website.authHelp')">
        <NSwitch v-model:value="editor.form.authEnabled" />
      </NFormItem>

      <template v-if="editor.form.authEnabled">
        <NFormItem :label="t('website.authUser')">
          <NInput v-model:value="editor.form.authUser" placeholder="khach" />
        </NFormItem>

        <NFormItem :label="t('website.authPassword')" :feedback="t('website.authPasswordHelp')">
          <NInput
            v-model:value="editor.form.authPassword"
            type="password"
            show-password-on="click"
          />
        </NFormItem>
      </template>

      <NFormItem :label="t('website.denyIps')" :feedback="t('website.denyIpsHelp')">
        <NInput v-model:value="editor.form.denyIps" type="textarea" :rows="2" />
      </NFormItem>

      <NFormItem :label="t('website.redirects')" :feedback="t('website.redirectsHelp')">
        <NSpace vertical :size="8" style="width: 100%">
          <NSpace
            v-for="(rule, index) in editor.form.redirects"
            :key="index"
            :size="6"
            :wrap="false"
            align="center"
          >
            <NInput v-model:value="rule.from" placeholder="/duong-dan-cu" style="width: 190px" />
            <NInput v-model:value="rule.to" placeholder="https://example.com/moi" />
            <NSpace align="center" :size="4" :wrap="false">
              <NSwitch v-model:value="rule.permanent" size="small" />
              <NText depth="3" style="font-size: 12px">301</NText>
            </NSpace>
            <NButton quaternary type="error" size="small" @click="editor.form.redirects.splice(index, 1)">
              ×
            </NButton>
          </NSpace>

          <NButton
            size="small"
            dashed
            @click="editor.form.redirects.push({ from: '', to: '', permanent: false })"
          >
            {{ t('website.addRedirect') }}
          </NButton>
        </NSpace>
      </NFormItem>

      <NFormItem :label="t('website.extraConfig')" :feedback="t('website.extraConfigHelp')">
        <NInput
          v-model:value="editor.form.extraConfig"
          type="textarea"
          :rows="3"
          class="sp-metric"
        />
      </NFormItem>

      <NFormItem :label="t('website.remark')">
        <NInput v-model:value="editor.form.remark" />
      </NFormItem>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="editor.saving"
        :disabled="!canSaveSite"
      >
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="deploy.show"
    preset="card"
    :title="t('website.deploySourceOf', { name: deploy.site?.name ?? '' })"
    style="width: 92vw; max-width: 560px"
  >
    <NSpace vertical :size="16">
      <NText depth="3" style="font-size: 13px">{{ t('website.deployHint') }}</NText>

      <NUpload
        :default-upload="false"
        :max="1"
        @change="pickSource"
      >
        <NButton>{{ t('website.chooseArchive') }}</NButton>
      </NUpload>

      <NInput
        v-model:value="deploy.path"
        :placeholder="t('website.serverPath')"
        :disabled="Boolean(deploy.file)"
      />

      <NCheckbox v-model:checked="deploy.clean">{{ t('website.deployClean') }}</NCheckbox>
      <NCheckbox v-model:checked="deploy.keepWrapper">{{ t('website.keepWrapper') }}</NCheckbox>

      <NButton
        type="primary"
        block
        :loading="deploy.running"
        :disabled="!canDeploy"
        @click="submitDeploy"
      >
        {{ t('website.deploy') }}
      </NButton>
    </NSpace>
  </NModal>

  <NModal
    v-model:show="certEditor.show"
    preset="card"
    :title="t('website.certCreate')"
    style="width: 92vw; max-width: 640px"
  >
    <NForm @submit.prevent="saveCert">
      <NFormItem :label="t('website.name')" :feedback="t('website.nameHelp')">
        <NInput v-model:value="certEditor.form.name" autofocus />
      </NFormItem>

      <NFormItem :label="t('website.domains')" :feedback="t('website.domainsHelp')">
        <NInput v-model:value="certEditor.form.domains" type="textarea" :rows="2" />
      </NFormItem>

      <NFormItem :label="t('website.source')">
        <NSelect v-model:value="certEditor.form.source" :options="certSourceOptions" />
      </NFormItem>

      <template v-if="certEditor.form.source === 'acme'">
        <NAlert type="info" :bordered="false" style="margin-bottom: 12px">
          {{ t('website.acmeHint') }}
        </NAlert>

        <NFormItem :label="t('website.email')" :feedback="t('website.emailHelp')">
          <NInput v-model:value="certEditor.form.email" placeholder="admin@example.com" />
        </NFormItem>

        <NFormItem :label="t('website.staging')" :feedback="t('website.stagingHelp')">
          <NSwitch v-model:value="certEditor.form.staging" />
        </NFormItem>

        <NFormItem :label="t('website.autoRenew')">
          <NSwitch v-model:value="certEditor.form.autoRenew" />
        </NFormItem>
      </template>

      <template v-else-if="certEditor.form.source === 'upload'">
        <NFormItem :label="t('website.certPem')">
          <NInput
            v-model:value="certEditor.form.certPem"
            type="textarea"
            :rows="4"
            class="sp-metric"
            placeholder="-----BEGIN CERTIFICATE-----"
          />
        </NFormItem>

        <NFormItem :label="t('website.keyPem')">
          <NInput
            v-model:value="certEditor.form.keyPem"
            type="textarea"
            :rows="4"
            class="sp-metric"
            placeholder="-----BEGIN PRIVATE KEY-----"
          />
        </NFormItem>
      </template>

      <NAlert v-else type="warning" :bordered="false" style="margin-bottom: 12px">
        {{ t('website.selfSignedHint') }}
      </NAlert>

      <NButton
        type="primary"
        block
        attr-type="submit"
        :loading="certEditor.saving"
        :disabled="!canSaveCert"
      >
        {{ t('common.save') }}
      </NButton>
    </NForm>
  </NModal>

  <NModal
    v-model:show="preview.show"
    preset="card"
    :title="t('website.configOf', { name: preview.name })"
    style="width: 94vw; max-width: 900px"
  >
    <pre class="config-preview">{{ preview.content }}</pre>
  </NModal>
</template>

<style scoped>
.domain-link {
  color: inherit;
  text-decoration: none;
  border-bottom: 1px dashed currentColor;
  opacity: 0.85;
}

.domain-link:hover {
  opacity: 1;
}

.config-preview {
  max-height: 65vh;
  font-family: var(--sp-font-mono);
  margin: 0;
  padding: 12px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  border-radius: 6px;
  background: var(--sp-code-bg, rgba(128, 128, 128, 0.08));
}
</style>
