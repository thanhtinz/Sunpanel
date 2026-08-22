package com.sunpanel.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class MainActivity : ComponentActivity() {

    /** Lời gọi lại của WebView đang chờ người dùng chọn tệp để tải lên. */
    private var pendingFiles: ValueCallback<Array<Uri>>? = null

    private val chooseFiles =
        registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
            val callback = pendingFiles ?: return@registerForActivityResult
            pendingFiles = null
            callback.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(result.resultCode, result.data))
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            SunPanelTheme {
                val context = LocalContext.current
                val store = remember { ServerStore(context) }
                val servers by store.servers

                var openId by rememberSaveable { mutableStateOf<String?>(null) }
                var openedLast by rememberSaveable { mutableStateOf(false) }
                var sshSession by remember { mutableStateOf<SshSession?>(null) }
                var dangNoi by remember { mutableStateOf<String?>(null) }
                var loiNoi by remember { mutableStateOf<String?>(null) }
                val scope = rememberCoroutineScope()

                // Mở thẳng máy chủ dùng lần trước, nhưng chỉ một lần cho mỗi lần chạy:
                // sau khi người dùng bấm quay lại để về danh sách, xoay ngang màn hình
                // không được kéo họ trở vào panel.
                LaunchedEffect(Unit) {
                    if (!openedLast) {
                        openedLast = true
                        // Chỉ panel mới tự mở lại: kết nối SSH cần mật khẩu, mà
                        // hỏi ngay khi vừa bật ứng dụng thì không khác gì bắt
                        // đăng nhập.
                        openId = store.last()?.takeIf { it.kind == Kind.PANEL }?.id
                    }
                }

                val open = openId?.let { store.byId(it) }
                val session = sshSession

                Surface(modifier = Modifier.fillMaxSize()) {
                    when {
                        open != null && open.kind == Kind.SSH && session != null ->
                            TerminalScreen(
                                server = open,
                                session = session,
                                onExit = {
                                    session.close()
                                    sshSession = null
                                    openId = null
                                },
                            )

                        open != null && open.kind == Kind.PANEL ->
                            PanelScreen(
                                server = open,
                                onTrustCertificate = { fingerprint -> store.trustCertificate(open.id, fingerprint) },
                                onFileChooser = ::showFileChooser,
                                onExit = { openId = null },
                            )

                        else ->
                            ServerListScreen(
                                servers = servers,
                                dangNoi = dangNoi,
                                loi = loiNoi,
                                onOpen = { server, matKhau ->
                                    loiNoi = null
                                    if (server.kind == Kind.PANEL) {
                                        store.markLast(server.id)
                                        openId = server.id
                                    } else {
                                        dangNoi = server.id
                                        scope.launch {
                                            // Bắt tay SSH đi qua mạng, có thể mất
                                            // vài giây; chạy ở luồng nền để giao
                                            // diện không đứng hình.
                                            val ketQua = withContext(Dispatchers.IO) {
                                                runCatching { SshSession.open(server, matKhau) }
                                            }
                                            dangNoi = null
                                            ketQua
                                                .onSuccess { moi ->
                                                    store.trustHostKey(server.id, moi.hostKey)
                                                    store.markLast(server.id)
                                                    sshSession = moi
                                                    openId = server.id
                                                }
                                                .onFailure { loiNoi = describeSshError(it) }
                                        }
                                    }
                                },
                                onSave = { draft -> saveServer(store, draft) },
                                onRemove = store::remove,
                            )
                    }
                }
            }
        }
    }

    /** Lưu máy chủ, trả về câu lỗi đã dịch nếu thông tin không dùng được. */
    private fun saveServer(store: ServerStore, draft: Server): String? {
        return try {
            store.save(draft)
            null
        } catch (e: InvalidAddress) {
            getString(
                when (e.reason) {
                    InvalidAddress.Reason.EMPTY -> R.string.err_url_empty
                    InvalidAddress.Reason.MALFORMED -> R.string.err_url_malformed
                    InvalidAddress.Reason.SCHEME -> R.string.err_url_scheme
                    InvalidAddress.Reason.HOST -> R.string.err_url_host
                    InvalidAddress.Reason.USER -> R.string.err_url_user
                    InvalidAddress.Reason.PORT -> R.string.err_url_port
                }
            )
        }
    }

    /** Đổi lỗi SSH thành câu nói rõ chuyện gì xảy ra và làm gì tiếp. */
    private fun describeSshError(error: Throwable): String {
        val ssh = error as? SshError ?: return error.message ?: error.toString()
        return when (ssh.reason) {
            SshError.Reason.HOST_KEY_CHANGED -> getString(R.string.err_ssh_host_key)
            SshError.Reason.AUTH_FAILED -> getString(R.string.err_ssh_auth)
            SshError.Reason.UNREACHABLE -> getString(R.string.err_ssh_unreachable)
            SshError.Reason.OTHER -> ssh.detail.ifEmpty { getString(R.string.err_ssh_unreachable) }
        }
    }

    /** Mở bộ chọn tệp của hệ thống cho ô tải lên trong trình quản lý tệp của panel. */
    private fun showFileChooser(
        callback: ValueCallback<Array<Uri>>,
        params: WebChromeClient.FileChooserParams,
    ): Boolean {
        // Chỉ một lượt chọn được chờ: lượt cũ chưa trả lời thì WebView treo ô tải lên.
        pendingFiles?.onReceiveValue(null)
        pendingFiles = callback

        return try {
            chooseFiles.launch(params.createIntent().addCategory(Intent.CATEGORY_OPENABLE))
            true
        } catch (e: Exception) {
            pendingFiles = null
            callback.onReceiveValue(null)
            false
        }
    }
}
