<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import {
  NBreadcrumb,
  NBreadcrumbItem,
  NButton,
  NDataTable,
  NInput,
  NModal,
  NSpace,
  NSpin,
  NText,
  NUpload,
  useDialog,
  useMessage,
  type DataTableColumns,
  type UploadCustomRequestOptions,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { ApiError, nodeApi, type RemoteFile } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { translateError } from '@/locales'
import { formatBytes, formatDateTime } from '@/utils/format'
import CodeEditor from '@/components/CodeEditor.vue'

const props = defineProps<{ nodeId: number }>()

const { t } = useI18n()
const auth = useAuthStore()
const message = useMessage()
const dialog = useDialog()

const cwd = ref('/')
const entries = ref<RemoteFile[]>([])
const loading = ref(false)

const editor = ref({ show: false, path: '', name: '', content: '', saving: false })

watch(() => props.nodeId, () => void open('/'), { immediate: true })

function report(err: unknown): void {
  message.error(err instanceof ApiError ? translateError(err.code, err.params) : t('error.network'))
}

async function open(dir: string): Promise<void> {
  loading.value = true
  try {
    entries.value = await nodeApi.files.list(props.nodeId, dir)
    cwd.value = dir
  } catch (err) {
    report(err)
  } finally {
    loading.value = false
  }
}

/** Các mốc đường dẫn để bấm ngược lên thư mục cha. */
const crumbs = computed(() => {
  const parts = cwd.value.split('/').filter(Boolean)
  const out = [{ label: '/', path: '/' }]
  let accumulated = ''
  for (const part of parts) {
    accumulated += `/${part}`
    out.push({ label: part, path: accumulated })
  }
  return out
})

async function openEntry(entry: RemoteFile): Promise<void> {
  if (entry.isDir) {
    await open(entry.path)
    return
  }

  try {
    const { content } = await nodeApi.files.read(props.nodeId, entry.path)
    editor.value = {
      show: true,
      path: entry.path,
      name: entry.name,
      content,
      saving: false,
    }
  } catch (err) {
    report(err)
  }
}

async function save(): Promise<void> {
  editor.value.saving = true
  try {
    await nodeApi.files.write(props.nodeId, editor.value.path, editor.value.content)
    message.success(t('remoteFiles.saved'))
    editor.value.show = false
  } catch (err) {
    report(err)
  } finally {
    editor.value.saving = false
  }
}

async function download(entry: RemoteFile): Promise<void> {
  try {
    window.location.href = await nodeApi.files.downloadUrl(props.nodeId, entry.path)
  } catch (err) {
    report(err)
  }
}

function newFolder(): void {
  const name = ref('')
  dialog.create({
    title: t('remoteFiles.newFolder'),
    content: () =>
      h(NInput, {
        value: name.value,
        placeholder: t('remoteFiles.folderName'),
        'onUpdate:value': (value: string) => (name.value = value),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      if (!name.value.trim()) return
      try {
        await nodeApi.files.mkdir(props.nodeId, joinPath(cwd.value, name.value.trim()))
        await open(cwd.value)
      } catch (err) {
        report(err)
      }
    },
  })
}

function rename(entry: RemoteFile): void {
  const name = ref(entry.name)
  dialog.create({
    title: t('remoteFiles.rename'),
    content: () =>
      h(NInput, {
        value: name.value,
        'onUpdate:value': (value: string) => (name.value = value),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const target = name.value.trim()
      if (!target || target === entry.name) return
      try {
        await nodeApi.files.rename(props.nodeId, entry.path, joinPath(cwd.value, target))
        await open(cwd.value)
      } catch (err) {
        report(err)
      }
    },
  })
}

function chmod(entry: RemoteFile): void {
  const mode = ref('0644')
  dialog.create({
    title: t('remoteFiles.chmod'),
    content: () =>
      h(NInput, {
        value: mode.value,
        placeholder: '0644',
        'onUpdate:value': (value: string) => (mode.value = value),
      }),
    positiveText: t('common.save'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await nodeApi.files.chmod(props.nodeId, entry.path, mode.value.trim())
        await open(cwd.value)
      } catch (err) {
        report(err)
      }
    },
  })
}

function remove(entry: RemoteFile): void {
  dialog.warning({
    title: t('common.delete'),
    // Xóa qua SFTP không đi vào thùng rác nào cả; nói thẳng điều đó ra.
    content: t('remoteFiles.deleteConfirm', { name: entry.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await nodeApi.files.remove(props.nodeId, entry.path)
        message.success(t('remoteFiles.deleted'))
        await open(cwd.value)
      } catch (err) {
        report(err)
      }
    },
  })
}

/** Tải tệp lên thư mục đang mở. */
async function upload({ file, onFinish, onError }: UploadCustomRequestOptions): Promise<void> {
  if (!file.file) return
  try {
    await nodeApi.files.upload(props.nodeId, cwd.value, file.file)
    message.success(t('remoteFiles.uploaded', { name: file.name }))
    await open(cwd.value)
    onFinish()
  } catch (err) {
    report(err)
    onError()
  }
}

function joinPath(dir: string, name: string): string {
  return dir === '/' ? `/${name}` : `${dir}/${name}`
}

const columns = computed<DataTableColumns<RemoteFile>>(() => [
  {
    title: t('remoteFiles.name'),
    key: 'name',
    // Không dùng ellipsis của bảng: nó bọc nội dung trong một lớp bắt sự kiện
    // bấm, và cú bấm vào nút bên trong rơi vào đúng lớp đó thay vì vào nút.
    // Cắt chữ bằng kiểu ngay trên nút, kèm title để vẫn xem được tên đầy đủ.
    render: (row) =>
      h(
        NButton,
        {
          text: true,
          type: row.isDir ? 'primary' : 'default',
          title: row.path,
          style: 'max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap',
          onClick: () => openEntry(row),
        },
        { default: () => (row.isDir ? `${row.name}/` : row.name) },
      ),
  },
  {
    title: t('remoteFiles.size'),
    key: 'size',
    width: 110,
    render: (row) => (row.isDir ? '—' : formatBytes(row.size)),
  },
  {
    title: t('remoteFiles.mode'),
    key: 'mode',
    width: 130,
    render: (row) => h(NText, { class: 'sp-metric', depth: 3 }, { default: () => row.mode }),
  },
  {
    title: t('remoteFiles.modified'),
    key: 'modTime',
    width: 170,
    render: (row) =>
      h(
        NText,
        { depth: 3, style: 'font-size: 12px' },
        { default: () => formatDateTime(new Date(row.modTime).toISOString()) },
      ),
  },
  {
    title: '',
    key: 'actions',
    width: 240,
    render: (row) =>
      h(NSpace, { size: 4, wrap: false }, {
        default: () => [
          row.isDir
            ? null
            : h(
                NButton,
                { size: 'tiny', quaternary: true, onClick: () => download(row) },
                { default: () => t('remoteFiles.download') },
              ),
          auth.isAdmin
            ? h(
                NButton,
                { size: 'tiny', quaternary: true, onClick: () => rename(row) },
                { default: () => t('remoteFiles.rename') },
              )
            : null,
          auth.isAdmin
            ? h(
                NButton,
                { size: 'tiny', quaternary: true, onClick: () => chmod(row) },
                { default: () => t('remoteFiles.chmodShort') },
              )
            : null,
          auth.isAdmin
            ? h(
                NButton,
                { size: 'tiny', quaternary: true, type: 'error', onClick: () => remove(row) },
                { default: () => t('common.delete') },
              )
            : null,
        ],
      }),
  },
])
</script>

<template>
  <NSpace vertical :size="12">
    <NSpace align="center" justify="space-between">
      <NBreadcrumb>
        <NBreadcrumbItem
          v-for="crumb in crumbs"
          :key="crumb.path"
          style="cursor: pointer"
          @click="open(crumb.path)"
        >
          {{ crumb.label }}
        </NBreadcrumbItem>
      </NBreadcrumb>

      <NSpace :size="8">
        <NButton size="small" :loading="loading" @click="open(cwd)">
          {{ t('common.refresh') }}
        </NButton>
        <template v-if="auth.isAdmin">
          <NButton size="small" @click="newFolder">{{ t('remoteFiles.newFolder') }}</NButton>
          <NUpload :custom-request="upload" :show-file-list="false">
            <NButton size="small" type="primary">{{ t('remoteFiles.upload') }}</NButton>
          </NUpload>
        </template>
      </NSpace>
    </NSpace>

    <NSpin :show="loading">
      <NDataTable
        :columns="columns"
        :data="entries"
        :row-key="(row: RemoteFile) => row.path"
        size="small"
        max-height="420"
      />
    </NSpin>
  </NSpace>

  <NModal
    v-model:show="editor.show"
    preset="card"
    :title="editor.name"
    style="width: 94vw; max-width: 900px"
  >
    <NSpace vertical :size="12">
      <CodeEditor
        v-model="editor.content"
        :filename="editor.name"
        :readonly="!auth.isAdmin"
        @save="save"
      />
      <NButton v-if="auth.isAdmin" type="primary" :loading="editor.saving" @click="save">
        {{ t('common.save') }}
      </NButton>
    </NSpace>
  </NModal>
</template>
