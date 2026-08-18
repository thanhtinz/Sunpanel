<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as monaco from 'monaco-editor/editor/editor.api'
import 'monaco-editor/basic-languages/monaco.contribution'
import editorWorker from 'monaco-editor/editor/editor.worker?worker'
import { useThemeStore } from '@/stores/theme'

const props = defineProps<{ modelValue: string; filename?: string; readonly?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [string]; save: [] }>()

const theme = useThemeStore()
const container = ref<HTMLElement | null>(null)
const editor = shallowRef<monaco.editor.IStandaloneCodeEditor | null>(null)

// Nạp editor.api (lõi) chứ không phải gói "monaco-editor" đầy đủ.
//
// Gói đầy đủ kéo theo dịch vụ ngôn ngữ của TypeScript, CSS, HTML và JSON — riêng
// worker của TypeScript đã 7 MB, và tất cả đều bị nhúng vào binary của panel.
// Trình sửa tệp cấu hình chỉ cần tô màu cú pháp, việc đó do basic-languages lo
// bằng bộ tokenizer thuần, không cần worker riêng nào.
//
// Đường dẫn theo exports map của monaco-editor 0.56: "monaco-editor/<x>" trỏ tới
// "esm/vs/<x>.js", nên không viết trực tiếp "monaco-editor/esm/vs/...".
self.MonacoEnvironment = { getWorker: () => new editorWorker() }

/** Đoán ngôn ngữ tô màu từ phần mở rộng của tệp. */
function languageOf(filename?: string): string {
  const ext = filename?.split('.').pop()?.toLowerCase() ?? ''
  const map: Record<string, string> = {
    js: 'javascript', mjs: 'javascript', ts: 'typescript', json: 'json',
    html: 'html', htm: 'html', css: 'css', scss: 'scss',
    go: 'go', py: 'python', rb: 'ruby', php: 'php', rs: 'rust',
    sh: 'shell', bash: 'shell', zsh: 'shell',
    yml: 'yaml', yaml: 'yaml', toml: 'ini', ini: 'ini', conf: 'ini',
    sql: 'sql', md: 'markdown', xml: 'xml', vue: 'html',
  }
  return map[ext] ?? 'plaintext'
}

onMounted(() => {
  if (!container.value) return

  editor.value = monaco.editor.create(container.value, {
    value: props.modelValue,
    language: languageOf(props.filename),
    theme: theme.isDark ? 'vs-dark' : 'vs',
    readOnly: props.readonly,
    automaticLayout: true,
    minimap: { enabled: false },
    fontSize: 13,
    // Font mặc định của monospace trên nhiều bản Linux là DejaVu Sans Mono, và
    // font đó dựng sai dấu tiếng Việt: "để thử" hiện thành "đê`thư`" vì dấu móc
    // và dấu hỏi bị đẩy ra khoảng trắng bên cạnh. Liberation Mono, Courier New,
    // Consolas và Menlo đều dựng đúng, nên ưu tiên chúng trước khi rơi về
    // monospace chung.
    fontFamily:
      '"Cascadia Mono", Consolas, Menlo, "Liberation Mono", "Courier New", monospace',
    tabSize: 2,
    scrollBeyondLastLine: false,
    renderWhitespace: 'selection',
    // Tắt cảnh báo ký tự Unicode "dễ nhầm với ASCII". Cảnh báo đó dành cho việc
    // săn ký tự đồng hình trong mã nguồn công khai; ở đây nó chỉ bôi khung lên
    // các bình luận tiếng Việt hoàn toàn bình thường.
    unicodeHighlight: {
      ambiguousCharacters: false,
      invisibleCharacters: false,
      nonBasicASCII: false,
    },
  })

  editor.value.onDidChangeModelContent(() => {
    emit('update:modelValue', editor.value?.getValue() ?? '')
  })

  // Ctrl/Cmd+S là phản xạ của mọi người khi sửa tệp; chặn để trình duyệt không
  // mở hộp thoại lưu trang.
  editor.value.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => emit('save'))
})

onBeforeUnmount(() => {
  editor.value?.dispose()
  editor.value = null
})

watch(
  () => props.modelValue,
  (value) => {
    // Chỉ ghi đè khi nội dung thực sự khác, nếu không con trỏ sẽ nhảy về đầu tệp
    // mỗi lần người dùng gõ.
    if (editor.value && editor.value.getValue() !== value) editor.value.setValue(value)
  },
)

watch(
  () => props.filename,
  (name) => {
    const model = editor.value?.getModel()
    if (model) monaco.editor.setModelLanguage(model, languageOf(name))
  },
)

watch(
  () => theme.isDark,
  (dark) => monaco.editor.setTheme(dark ? 'vs-dark' : 'vs'),
)
</script>

<template>
  <div ref="container" class="editor" />
</template>

<style scoped>
.editor {
  width: 100%;
  height: 100%;
  min-height: 320px;
}
</style>
