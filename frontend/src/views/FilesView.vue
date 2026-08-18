<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NBreadcrumb,
  NBreadcrumbItem,
  NButton,
  NCard,
  NDataTable,
  NDropdown,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NTag,
  NText,
  NUpload,
  useMessage,
  type DataTableColumns,
  type UploadFileInfo,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, fileApi, type ArchiveFormat, type FileInfo } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'
import CodeEditor from '@/components/CodeEditor.vue'
import IconFolder from '@/components/icons/IconFolder.vue'
import IconFile from '@/components/icons/IconFile.vue'

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()

const currentPath = ref('/')
const items = ref<FileInfo[]>([])
const loading = ref(false)
const checkedPaths = ref<string[]>([])

// Hộp thoại nhập một dòng dùng chung cho tạo thư mục, tạo tệp, đổi tên, phân quyền.
const prompt = ref({
  show: false,
  title: '',
  label: '',
  value: '',
  action: (() => Promise.resolve()) as (value: string) => Promise<void>,
})

const editor = ref({ show: false, path: '', content: '', saving: false })

const archive = ref({ show: false, name: 'luu-tru.zip', format: 'zip' as ArchiveFormat })

const uploadUrl = fileApi.uploadUrl()
const uploadHeaders = computed(() => ({ Authorization: `Bearer ${auth.accessToken}` }))

const breadcrumbs = computed(() => {
  const parts = currentPath.value.split('/').filter(Boolean)
  const crumbs = [{ label: t('files.root'), path: '/' }]
  let accumulated = ''
  for (const part of parts) {
    accumulated += '/' + part
    crumbs.push({ label: part, path: accumulated })
  }
  return crumbs
})

onMounted(() => void load('/'))

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function load(path: string): Promise<void> {
  loading.value = true
  try {
    const result = await fileApi.list(path)
    currentPath.value = result.path
    items.value = result.items
    // Lựa chọn cũ không còn ý nghĩa ở thư mục mới.
    checkedPaths.value = []
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/** Chạy một thao tác rồi nạp lại thư mục, gom phần xử lý lỗi về một chỗ. */
async function run(operation: () => Promise<unknown>, successMessage: string): Promise<void> {
  try {
    await operation()
    message.success(successMessage)
    await load(currentPath.value)
  } catch (err) {
    report(err)
  }
}

function joinPath(dir: string, name: string): string {
  return dir === '/' ? `/${name}` : `${dir}/${name}`
}

async function openItem(item: FileInfo): Promise<void> {
  if (item.isDir) {
    await load(item.path)
    return
  }

  try {
    const content = await fileApi.read(item.path)
    editor.value = { show: true, path: content.path, content: content.content, saving: false }
  } catch (err) {
    report(err)
  }
}

async function saveEditor(): Promise<void> {
  editor.value.saving = true
  try {
    await fileApi.write(editor.value.path, editor.value.content)
    message.success(t('files.saved'))
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

async function download(item: FileInfo): Promise<void> {
  try {
    // Vé chỉ sống 60 giây nên phải xin ngay trước khi điều hướng.
    const url = await fileApi.downloadUrl(item.path)
    const link = document.createElement('a')
    link.href = url
    link.download = item.name
    link.click()
  } catch (err) {
    report(err)
  }
}

function openPrompt(
  title: string,
  label: string,
  value: string,
  action: (value: string) => Promise<void>,
): void {
  prompt.value = { show: true, title, label, value, action }
}

async function submitPrompt(): Promise<void> {
  const value = prompt.value.value.trim()
  if (!value) return

  const action = prompt.value.action
  prompt.value.show = false
  await action(value)
}

function promptNewFolder(): void {
  openPrompt(t('files.newFolder'), t('files.folderName'), '', (name) =>
    run(() => fileApi.mkdir(joinPath(currentPath.value, name)), t('files.created')),
  )
}

function promptNewFile(): void {
  openPrompt(t('files.newFile'), t('files.fileName'), '', (name) =>
    run(() => fileApi.write(joinPath(currentPath.value, name), ''), t('files.created')),
  )
}

function promptRename(item: FileInfo): void {
  openPrompt(t('files.rename'), t('files.newName'), item.name, (name) =>
    run(() => fileApi.move(item.path, joinPath(currentPath.value, name)), t('files.renamed')),
  )
}

function promptChmod(item: FileInfo): void {
  openPrompt(t('files.permissions'), t('files.mode'), item.mode, (mode) =>
    run(() => fileApi.chmod(item.path, mode), t('files.permissionsChanged')),
  )
}

function promptExtract(item: FileInfo): void {
  // Mặc định giải nén vào thư mục cùng tên với tệp nén, bỏ phần mở rộng.
  const suggested = item.name.replace(/\.(zip|tar\.gz|tgz)$/i, '')
  openPrompt(t('files.extract'), t('files.extractTo'), suggested, (target) =>
    run(() => fileApi.extract(item.path, joinPath(currentPath.value, target)), t('files.extracted')),
  )
}

async function submitCompress(): Promise<void> {
  const { name, format } = archive.value
  archive.value.show = false
  await run(
    () => fileApi.compress(checkedPaths.value, joinPath(currentPath.value, name), format),
    t('files.compressed'),
  )
}

async function removeSelected(): Promise<void> {
  await run(() => fileApi.remove(checkedPaths.value), t('files.deleted'))
}

function onUploadFinish({ file }: { file: UploadFileInfo }): UploadFileInfo {
  message.success(t('files.uploaded', { count: 1 }))
  void load(currentPath.value)
  return file
}

function onUploadError(): void {
  message.error(t('error.internal'))
}

const columns = computed<DataTableColumns<FileInfo>>(() => [
  { type: 'selection' },
  {
    title: t('files.name'),
    key: 'name',
    render: (row) =>
      h(
        NButton,
        { text: true, onClick: () => openItem(row) },
        {
          icon: () => h(NIcon, null, { default: () => h(row.isDir ? IconFolder : IconFile) }),
          default: () =>
            row.isLink
              ? [row.name, h(NText, { depth: 3, style: 'font-size:12px;margin-left:6px' },
                  { default: () => t('files.symlinkTo', { target: row.linkTarget ?? '?' }) })]
              : row.name,
        },
      ),
  },
  {
    title: t('files.size'),
    key: 'size',
    width: 110,
    render: (row) => (row.isDir ? '—' : h('span', { class: 'sp-metric' }, formatBytes(row.size))),
  },
  {
    title: t('files.mode'),
    key: 'mode',
    width: 90,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => row.mode }),
  },
  { title: t('files.owner'), key: 'owner', width: 130, render: (row) => row.owner ?? '—' },
  {
    title: t('files.modified'),
    key: 'modTime',
    width: 180,
    render: (row) => formatDateTime(row.modTime),
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 90,
    render: (row) =>
      h(
        NDropdown,
        {
          trigger: 'click',
          options: rowActions(row),
          onSelect: (key: string) => handleRowAction(key, row),
        },
        {
          default: () => h(NButton, { size: 'tiny', quaternary: true }, { default: () => '⋯' }),
        },
      ),
  },
])

function rowActions(row: FileInfo) {
  const options: { label: string; key: string; disabled?: boolean }[] = []
  if (!row.isDir) options.push({ label: t('files.download'), key: 'download' })

  // Tài khoản chỉ xem không được thấy các thao tác họ không thực hiện được.
  if (auth.canWrite) {
    options.push(
      { label: t('files.rename'), key: 'rename' },
      { label: t('files.permissions'), key: 'chmod' },
    )
    if (/\.(zip|tar\.gz|tgz)$/i.test(row.name)) {
      options.push({ label: t('files.extract'), key: 'extract' })
    }
  }
  return options
}

async function handleRowAction(key: string, row: FileInfo): Promise<void> {
  switch (key) {
    case 'download':
      await download(row)
      break
    case 'rename':
      promptRename(row)
      break
    case 'chmod':
      promptChmod(row)
      break
    case 'extract':
      promptExtract(row)
      break
  }
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
        <NButton size="small" @click="load(currentPath)">{{ t('common.refresh') }}</NButton>

        <template v-if="auth.canWrite">
          <NButton size="small" @click="promptNewFolder">{{ t('files.newFolder') }}</NButton>
          <NButton size="small" @click="promptNewFile">{{ t('files.newFile') }}</NButton>

          <NUpload
            :action="uploadUrl"
            :headers="uploadHeaders"
            :data="{ path: currentPath }"
            name="file"
            :show-file-list="false"
            multiple
            @finish="onUploadFinish"
            @error="onUploadError"
          >
            <NButton size="small" type="primary">{{ t('files.upload') }}</NButton>
          </NUpload>
        </template>
      </NSpace>
    </template>

    <NSpace vertical :size="12">
      <NSpace align="center" justify="space-between">
        <NBreadcrumb>
          <NBreadcrumbItem
            v-for="crumb in breadcrumbs"
            :key="crumb.path"
            @click="load(crumb.path)"
          >
            {{ crumb.label }}
          </NBreadcrumbItem>
        </NBreadcrumb>

        <NSpace v-if="checkedPaths.length > 0 && auth.canWrite" align="center" :size="8">
          <NText depth="3">{{ t('files.selected', { count: checkedPaths.length }) }}</NText>
          <NButton size="small" @click="archive.show = true">{{ t('files.compress') }}</NButton>
          <NPopconfirm @positive-click="removeSelected">
            <template #trigger>
              <NButton size="small" type="error" quaternary>{{ t('common.delete') }}</NButton>
            </template>
            {{ t('files.deleteConfirm', { count: checkedPaths.length }) }}
          </NPopconfirm>
        </NSpace>
      </NSpace>

      <NDataTable
        v-model:checked-row-keys="checkedPaths"
        :columns="columns"
        :data="items"
        :loading="loading"
        :row-key="(row: FileInfo) => row.path"
        size="small"
        max-height="calc(100vh - 300px)"
        virtual-scroll
      />
    </NSpace>
  </NCard>

  <NModal
    v-model:show="prompt.show"
    preset="card"
    :title="prompt.title"
    style="max-width: 400px"
  >
    <NSpace vertical :size="14">
      <NInput
        v-model:value="prompt.value"
        :placeholder="prompt.label"
        autofocus
        @keydown.enter="submitPrompt"
      />
      <NButton type="primary" block :disabled="!prompt.value.trim()" @click="submitPrompt">
        {{ t('common.confirm') }}
      </NButton>
    </NSpace>
  </NModal>

  <NModal
    v-model:show="archive.show"
    preset="card"
    :title="t('files.compress')"
    style="max-width: 400px"
  >
    <NSpace vertical :size="14">
      <NInput v-model:value="archive.name" :placeholder="t('files.archiveName')" />
      <NSelect
        v-model:value="archive.format"
        :options="[
          { label: 'ZIP', value: 'zip' },
          { label: 'TAR.GZ', value: 'tar.gz' },
        ]"
      />
      <NButton type="primary" block @click="submitCompress">{{ t('common.confirm') }}</NButton>
    </NSpace>
  </NModal>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="`${t('files.editing')} — ${editor.path}`"
    style="width: 92vw; max-width: 1200px"
  >
    <template #header-extra>
      <NButton
        v-if="auth.canWrite"
        type="primary"
        size="small"
        :loading="editor.saving"
        @click="saveEditor"
      >
        {{ t('common.save') }}
      </NButton>
    </template>

    <div class="editor-shell">
      <CodeEditor
        v-model="editor.content"
        :filename="editor.path"
        :readonly="!auth.canWrite"
        @save="saveEditor"
      />
    </div>
  </NModal>
</template>

<style scoped>
.editor-shell {
  height: 65vh;
}
</style>
