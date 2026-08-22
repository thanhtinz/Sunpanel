package com.sunpanel.app

import android.annotation.SuppressLint
import android.util.Base64
import android.webkit.JavascriptInterface
import android.webkit.WebView
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import java.util.concurrent.Executors
import kotlin.concurrent.thread

/**
 * Terminal của một phiên SSH.
 *
 * Phần vẽ terminal là xterm.js chạy trong WebView chứ không phải một bộ giả lập
 * viết tay: một terminal thật phải hiểu đủ mã điều khiển để `vim` và `htop` vẽ
 * đúng, và đó là hàng chục nghìn dòng không có lý do gì để viết lại.
 */
@OptIn(ExperimentalMaterial3Api::class)
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun TerminalScreen(server: Server, session: SshSession, onExit: () -> Unit) {
    var metrics by remember { mutableStateOf(Metrics()) }
    var info by remember { mutableStateOf(HostInfo()) }
    val bridge = remember { TerminalBridge(session) }

    BackHandler { onExit() }

    DisposableEffect(session) {
        // Đọc thông tin máy một lần, rồi đo tài nguyên theo nhịp. Mỗi lần đo là
        // một phiên trên máy chủ nên năm giây một lần là đủ để thấy thay đổi mà
        // không làm phiền máy.
        val reader = thread(isDaemon = true) {
            runCatching { info = Readings.parseInfo(session.run(Readings.INFO_COMMAND)) }
            while (!Thread.currentThread().isInterrupted) {
                runCatching { metrics = Readings.parseMetrics(session.run(Readings.METRICS_COMMAND)) }
                    .onFailure { return@thread }
                try {
                    Thread.sleep(5_000)
                } catch (e: InterruptedException) {
                    return@thread
                }
            }
        }
        onDispose {
            reader.interrupt()
            bridge.close()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(server.name, style = MaterialTheme.typography.titleSmall)
                        Text(
                            listOf(server.label(), info.os).filter { it.isNotEmpty() }.joinToString(" · "),
                            style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                },
                actions = { MetricsRow(metrics) },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surface),
            )
        }
    ) { padding ->
        AndroidView(
            modifier = Modifier.fillMaxSize().padding(padding),
            factory = { context ->
                WebView(context).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    // Trang terminal nằm trong ứng dụng, không phải trên mạng.
                    settings.allowFileAccess = false
                    settings.allowContentAccess = false
                    setBackgroundColor(0xFF0B1220.toInt())

                    bridge.attach(this)
                    addJavascriptInterface(bridge, "Cau")
                    loadUrl("file:///android_asset/terminal.html")
                }
            },
        )
    }
}

/** Ba con số tài nguyên trên thanh tiêu đề. */
@Composable
private fun MetricsRow(metrics: Metrics) {
    Row(
        modifier = Modifier.padding(end = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MetricBar("CPU", metrics.cpu)
        MetricBar("RAM", metrics.memory)
        MetricBar("Đĩa", metrics.disk)
    }
}

@Composable
private fun MetricBar(label: String, percent: Double) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text("$label ${percent.toInt()}%", style = MaterialTheme.typography.labelSmall)
        Spacer(modifier = Modifier.height(3.dp))
        Box(modifier = Modifier.width(38.dp)) {
            LinearProgressIndicator(
                progress = { (percent / 100.0).toFloat() },
                modifier = Modifier.fillMaxWidth().height(4.dp),
                // Đỏ khi gần đầy: đó là lúc con số này đáng để ngước lên nhìn.
                color = if (percent >= 85) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant,
            )
        }
    }
}

/**
 * Cầu nối giữa trang terminal và phiên SSH.
 *
 * Mọi thứ đi qua đây đều là base64: cầu nối JavaScript chỉ chuyển được chuỗi, mà
 * đầu ra của máy chủ là byte bất kỳ — bao gồm cả byte không hợp lệ trong UTF-8
 * khi một ký tự tiếng Việt bị cắt làm đôi giữa hai lần đọc.
 */
class TerminalBridge(private val session: SshSession) {

    private var view: WebView? = null
    private var shell: SshSession.Shell? = null
    private val writer = Executors.newSingleThreadExecutor()

    fun attach(webView: WebView) {
        view = webView
    }

    /** Trang gọi khi xterm đã đo xong kích thước và sẵn sàng nhận dữ liệu. */
    @JavascriptInterface
    fun sanSang(cols: Int, rows: Int) {
        if (shell != null) return

        val opened =
            runCatching { session.openShell(cols.coerceAtLeast(20), rows.coerceAtLeast(4)) }
                .getOrElse { error ->
                    ketThuc(error.message ?: "không mở được phiên")
                    return
                }
        shell = opened

        thread(isDaemon = true) {
            val buffer = ByteArray(8192)
            while (true) {
                val n = runCatching { opened.input.read(buffer) }.getOrDefault(-1)
                if (n <= 0) break
                val base64 = Base64.encodeToString(buffer.copyOf(n), Base64.NO_WRAP)
                goiJS("window.nhan('$base64')")
            }
            ketThuc("")
        }
    }

    /** Trang gọi mỗi khi người dùng gõ hoặc bấm một phím phụ. */
    @JavascriptInterface
    fun go(base64: String) {
        val data = runCatching { Base64.decode(base64, Base64.DEFAULT) }.getOrNull() ?: return
        val stream = shell?.output ?: return
        // Ghi ở luồng riêng: cầu nối JavaScript chạy trên luồng của WebView, và
        // một lần ghi bị nghẽn mạng ở đó là cả giao diện đứng hình.
        writer.execute {
            runCatching {
                stream.write(data)
                stream.flush()
            }
        }
    }

    /** Trang gọi khi cửa sổ đổi kích thước, kể cả lúc bàn phím ảo bật lên. */
    @JavascriptInterface
    fun doiCo(cols: Int, rows: Int) {
        if (cols <= 0 || rows <= 0) return
        runCatching { shell?.resize(cols, rows) }
    }

    fun close() {
        shell?.close()
        shell = null
        writer.shutdownNow()
    }

    private fun ketThuc(message: String) {
        val escaped = message.replace("\\", "\\\\").replace("'", "\\'").replace("\n", " ")
        goiJS("window.ketThuc('$escaped')")
    }

    private fun goiJS(script: String) {
        val webView = view ?: return
        webView.post { webView.evaluateJavascript(script, null) }
    }
}
