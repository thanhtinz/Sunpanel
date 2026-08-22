package com.sunpanel.app

import android.annotation.SuppressLint
import android.app.DownloadManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.net.http.SslError
import android.os.Environment
import android.webkit.CookieManager
import android.webkit.SslErrorHandler
import android.webkit.URLUtil
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.DialogProperties

/** Một chứng chỉ đang chờ người dùng quyết định có tin hay không. */
private class PendingCertificate(val handler: SslErrorHandler, val fingerprint: String, val error: SslError)

/**
 * Màn hình mở panel.
 *
 * Không có thanh công cụ nào: màn hình điện thoại vốn đã chật, mà panel đã có
 * thanh điều hướng riêng. Nút quay lại của hệ thống đi lùi trong panel, hết lịch
 * sử thì về danh sách máy chủ.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun PanelScreen(
    server: Server,
    onTrustCertificate: (String) -> Unit,
    onFileChooser: (ValueCallback<Array<Uri>>, WebChromeClient.FileChooserParams) -> Boolean,
    onExit: () -> Unit,
) {
    val context = LocalContext.current
    var progress by remember { mutableStateOf(0) }
    var pending by remember { mutableStateOf<PendingCertificate?>(null) }
    var webView by remember { mutableStateOf<WebView?>(null) }

    BackHandler {
        val view = webView
        if (view != null && view.canGoBack()) view.goBack() else onExit()
    }

    // WebView giữ tiến trình nền và kết nối WebSocket của bảng giám sát; rời màn
    // hình mà không dọn là chúng chạy tiếp cho tới khi hệ thống thu hồi ứng dụng.
    DisposableEffect(Unit) {
        onDispose {
            webView?.apply {
                stopLoading()
                loadUrl("about:blank")
                destroy()
            }
            webView = null
        }
    }

    Box(modifier = Modifier.fillMaxSize()) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                WebView(ctx).apply {
                    settings.javaScriptEnabled = true
                    // Panel giữ phiên đăng nhập và tùy chọn giao diện trong bộ nhớ
                    // cục bộ của trình duyệt; tắt cái này là mỗi lần mở lại phải
                    // đăng nhập lại từ đầu.
                    settings.domStorageEnabled = true
                    settings.useWideViewPort = true
                    settings.loadWithOverviewMode = true
                    settings.setSupportZoom(false)
                    settings.mediaPlaybackRequiresUserGesture = false
                    CookieManager.getInstance().setAcceptThirdPartyCookies(this, true)

                    webViewClient =
                        object : WebViewClient() {
                            override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
                                val fingerprint = Certificates.fingerprint(error.certificate)
                                if (fingerprint.isNotEmpty() && fingerprint == server.certFingerprint) {
                                    handler.proceed()
                                    return
                                }
                                pending = PendingCertificate(handler, fingerprint, error)
                            }

                            override fun shouldOverrideUrlLoading(
                                view: WebView,
                                request: WebResourceRequest,
                            ): Boolean {
                                val target = request.url.toString()
                                if (Servers.sameOrigin(target, server.url)) return false

                                // Liên kết ra ngoài panel (tài liệu, trang web vừa
                                // tạo) mở bằng trình duyệt: giữ chúng trong ứng dụng
                                // này thì người dùng mắc kẹt, không có thanh địa chỉ
                                // để đi tiếp hay quay ra.
                                return openExternally(context, target)
                            }
                        }

                    webChromeClient =
                        object : WebChromeClient() {
                            override fun onProgressChanged(view: WebView, newProgress: Int) {
                                progress = newProgress
                            }

                            override fun onShowFileChooser(
                                view: WebView,
                                callback: ValueCallback<Array<Uri>>,
                                params: FileChooserParams,
                            ): Boolean = onFileChooser(callback, params)
                        }

                    setDownloadListener { url, userAgent, contentDisposition, mimeType, _ ->
                        startDownload(context, url, userAgent, contentDisposition, mimeType)
                    }

                    loadUrl(server.url)
                    webView = this
                }
            },
        )

        if (progress in 1..99) {
            LinearProgressIndicator(
                progress = { progress / 100f },
                modifier = Modifier.fillMaxWidth().align(Alignment.TopCenter),
            )
        }
    }

    pending?.let { certificate ->
        AlertDialog(
            onDismissRequest = {},
            properties = DialogProperties(dismissOnBackPress = false, dismissOnClickOutside = false),
            title = { Text(stringResource(R.string.cert_title)) },
            text = {
                Text(
                    stringResource(
                        R.string.cert_body,
                        certificate.error.url ?: server.url,
                        Certificates.readable(certificate.fingerprint),
                    ),
                    style = MaterialTheme.typography.bodyMedium,
                )
            },
            confirmButton = {
                TextButton(
                    enabled = certificate.fingerprint.isNotEmpty(),
                    onClick = {
                        onTrustCertificate(certificate.fingerprint)
                        certificate.handler.proceed()
                        pending = null
                    },
                ) {
                    Text(stringResource(R.string.cert_trust))
                }
            },
            dismissButton = {
                TextButton(
                    onClick = {
                        certificate.handler.cancel()
                        pending = null
                        onExit()
                    }
                ) {
                    Text(stringResource(R.string.cert_refuse))
                }
            },
        )
    }
}

/** Mở một địa chỉ bằng ứng dụng khác, trả về true nếu đã xử lý xong. */
private fun openExternally(context: Context, url: String): Boolean {
    return try {
        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
        true
    } catch (e: Exception) {
        // Không có ứng dụng nào nhận: để WebView tự xoay xở còn hơn không có gì
        // xảy ra khi bấm.
        false
    }
}

/** Giao tệp panel gửi về cho trình tải xuống của hệ thống. */
private fun startDownload(
    context: Context,
    url: String,
    userAgent: String?,
    contentDisposition: String?,
    mimeType: String?,
) {
    // Trình tải xuống của hệ thống chỉ hiểu http và https; tệp sinh ngay trong
    // trình duyệt (blob:) thì nó không với tới được.
    if (!URLUtil.isNetworkUrl(url)) return

    val name = URLUtil.guessFileName(url, contentDisposition, mimeType)
    val request =
        DownloadManager.Request(Uri.parse(url)).apply {
            setMimeType(mimeType)
            addRequestHeader("Cookie", CookieManager.getInstance().getCookie(url) ?: "")
            userAgent?.let { addRequestHeader("User-Agent", it) }
            setTitle(name)
            setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
            setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
        }

    val manager = context.getSystemService(Context.DOWNLOAD_SERVICE) as DownloadManager
    manager.enqueue(request)
}
