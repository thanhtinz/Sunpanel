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
import androidx.compose.ui.platform.LocalContext

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

                // Mở thẳng máy chủ dùng lần trước, nhưng chỉ một lần cho mỗi lần chạy:
                // sau khi người dùng bấm quay lại để về danh sách, xoay ngang màn hình
                // không được kéo họ trở vào panel.
                LaunchedEffect(Unit) {
                    if (!openedLast) {
                        openedLast = true
                        openId = store.last()?.id
                    }
                }

                val open = openId?.let { store.byId(it) }

                Surface(modifier = Modifier.fillMaxSize()) {
                    if (open == null) {
                        ServerListScreen(
                            servers = servers,
                            onOpen = { server ->
                                store.markLast(server.id)
                                openId = server.id
                            },
                            onSave = { id, name, url -> saveServer(store, id, name, url) },
                            onRemove = store::remove,
                        )
                    } else {
                        PanelScreen(
                            server = open,
                            onTrustCertificate = { fingerprint -> store.trustCertificate(open.id, fingerprint) },
                            onFileChooser = ::showFileChooser,
                            onExit = { openId = null },
                        )
                    }
                }
            }
        }
    }

    /** Lưu máy chủ, trả về câu lỗi đã dịch nếu địa chỉ không dùng được. */
    private fun saveServer(store: ServerStore, id: String, name: String, url: String): String? {
        return try {
            store.save(id, name, url)
            null
        } catch (e: InvalidAddress) {
            getString(
                when (e.reason) {
                    InvalidAddress.Reason.EMPTY -> R.string.err_url_empty
                    InvalidAddress.Reason.MALFORMED -> R.string.err_url_malformed
                    InvalidAddress.Reason.SCHEME -> R.string.err_url_scheme
                    InvalidAddress.Reason.HOST -> R.string.err_url_host
                }
            )
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
