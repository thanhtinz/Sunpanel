import type { ApiResponse } from './types'

/**
 * Lỗi từ API, mang theo mã dịch thay vì câu chữ cố định.
 *
 * Giao diện luôn hiển thị lỗi bằng cách dịch `code`, nhờ vậy cùng một lỗi hiện
 * ra bằng đúng ngôn ngữ người dùng đang chọn.
 */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
    readonly params: Record<string, unknown> = {},
  ) {
    super(code)
    this.name = 'ApiError'
  }
}

/** Hàm lấy access token hiện tại; do store xác thực cung cấp. */
type TokenProvider = () => string | null
/** Hàm làm mới token khi gặp 401; trả về true nếu làm mới thành công. */
type RefreshHandler = () => Promise<boolean>

let getToken: TokenProvider = () => null
let refreshToken: RefreshHandler = async () => false

/** Gắn store xác thực vào client. Gọi một lần lúc khởi tạo ứng dụng. */
export function configureClient(provider: TokenProvider, refresher: RefreshHandler): void {
  getToken = provider
  refreshToken = refresher
}

/**
 * baseUrl suy ra từ thẻ <base> mà máy chủ chèn vào, nên giao diện tự động gọi
 * đúng API kể cả khi panel nằm sau một đường dẫn bí mật.
 */
export function baseUrl(): string {
  const base = document.querySelector('base')?.getAttribute('href') ?? '/'
  return base.endsWith('/') ? base.slice(0, -1) : base
}

/** Dựng URL WebSocket tương ứng với đường dẫn API. */
export function wsUrl(path: string, token: string): string {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${location.host}${baseUrl()}${path}?token=${encodeURIComponent(token)}`
}

interface RequestOptions {
  method?: string
  body?: unknown
  /** Bỏ qua bước tự làm mới token — dùng cho chính endpoint làm mới. */
  skipRefresh?: boolean
}

/**
 * request gọi API và bóc lớp vỏ phản hồi.
 *
 * Khi gặp 401, hàm tự thử làm mới token đúng một lần rồi gọi lại. Nếu vẫn hỏng
 * thì ném lỗi để lớp trên đưa người dùng về trang đăng nhập.
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const send = async (): Promise<Response> => {
    // Tải tệp lên đi cùng FormData: trình duyệt phải tự đặt Content-Type vì nó
    // còn phải kèm chuỗi phân cách của multipart, thứ mã ở đây không biết trước.
    const multipart = options.body instanceof FormData
    const headers: Record<string, string> = multipart ? {} : { 'Content-Type': 'application/json' }
    const token = getToken()
    if (token) headers.Authorization = `Bearer ${token}`

    let body: BodyInit | undefined
    if (multipart) body = options.body as FormData
    else if (options.body !== undefined) body = JSON.stringify(options.body)

    return fetch(`${baseUrl()}${path}`, {
      method: options.method ?? 'GET',
      headers,
      body,
    })
  }

  let response = await send()

  if (response.status === 401 && !options.skipRefresh) {
    if (await refreshToken()) {
      response = await send()
    }
  }

  let payload: ApiResponse<T>
  try {
    payload = (await response.json()) as ApiResponse<T>
  } catch {
    // Máy chủ trả về thứ không phải JSON (proxy lỗi, sập giữa chừng...).
    throw new ApiError('error.network', response.status)
  }

  if (!response.ok || !payload.success) {
    throw new ApiError(payload.code ?? 'error.internal', response.status, payload.params ?? {})
  }

  return payload.data as T
}
