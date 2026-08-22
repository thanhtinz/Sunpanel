package com.sunpanel.app

import com.jcraft.jsch.ChannelExec
import com.jcraft.jsch.ChannelSftp
import com.jcraft.jsch.ChannelShell
import com.jcraft.jsch.HostKey
import com.jcraft.jsch.HostKeyRepository
import com.jcraft.jsch.JSch
import com.jcraft.jsch.JSchException
import com.jcraft.jsch.Session
import com.jcraft.jsch.UserInfo
import java.io.InputStream
import java.io.OutputStream
import java.security.MessageDigest
import java.util.Base64

/** Lỗi kết nối SSH, kèm mã để giao diện tự dịch. */
class SshError(val reason: Reason, val detail: String = "") : Exception(reason.name) {
    enum class Reason {
        /** Khóa máy chủ khác lần kết nối trước. */
        HOST_KEY_CHANGED,
        /** Sai tài khoản, mật khẩu hoặc khóa. */
        AUTH_FAILED,
        /** Không mở được kết nối tới máy chủ. */
        UNREACHABLE,
        /** Mọi thứ khác. */
        OTHER,
    }
}

/**
 * Một kết nối SSH đang mở.
 *
 * Giữ lại giữa các màn hình: mở terminal rồi bấm sang tệp không nên phải đăng
 * nhập lại, và mỗi lần bắt tay SSH là một vòng đi về qua mạng.
 */
class SshSession private constructor(private val session: Session, val hostKey: String) {

    /** Một phiên shell đang chạy trên máy chủ. */
    class Shell(
        private val channel: ChannelShell,
        val input: InputStream,
        val output: OutputStream,
    ) {
        /** Báo kích thước cửa sổ mới cho máy chủ. */
        fun resize(cols: Int, rows: Int) {
            // Kích thước tính bằng điểm ảnh chỉ để cho đủ tham số; máy chủ ngắt
            // dòng theo số cột và số dòng.
            channel.setPtySize(cols, rows, cols * 8, rows * 16)
        }

        fun close() {
            runCatching { channel.disconnect() }
        }
    }

    /** Mở một shell có PTY. */
    fun openShell(cols: Int, rows: Int): Shell {
        val channel = session.openChannel("shell") as ChannelShell
        channel.setPtyType("xterm-256color", cols, rows, cols * 8, rows * 16)
        val output = channel.outputStream
        val input = channel.inputStream
        channel.connect(CONNECT_TIMEOUT_MS)
        return Shell(channel, input, output)
    }

    /** Chạy một lệnh và trả về đầu ra. */
    fun run(command: String): String {
        val channel = session.openChannel("exec") as ChannelExec
        try {
            channel.setCommand(command)
            // Bỏ luồng lỗi: mọi lệnh đọc thông tin đều tự nuốt lỗi, phần vắng mặt
            // chỉ đơn giản là để trống.
            channel.setErrStream(null)
            val input = channel.inputStream
            channel.connect(CONNECT_TIMEOUT_MS)
            return input.readBytes().toString(Charsets.UTF_8)
        } finally {
            runCatching { channel.disconnect() }
        }
    }

    /** Liệt kê một thư mục qua SFTP. */
    fun list(dir: String): Pair<String, List<RemoteFile>> {
        val channel = session.openChannel("sftp") as ChannelSftp
        try {
            channel.connect(CONNECT_TIMEOUT_MS)
            val path = if (dir.isEmpty() || dir == ".") channel.pwd() else channel.realpath(dir)

            val entries = mutableListOf<RemoteFile>()
            for (item in channel.ls(path)) {
                val entry = item as ChannelSftp.LsEntry
                // "." và ".." do máy chủ trả về; giao diện tự dựng lối đi lên.
                if (entry.filename == "." || entry.filename == "..") continue
                entries.add(
                    RemoteFile(
                        name = entry.filename,
                        size = entry.attrs.size,
                        isDir = entry.attrs.isDir,
                        isLink = entry.attrs.isLink,
                    )
                )
            }
            entries.sortWith(compareBy({ !it.isDir }, { it.name.lowercase() }))
            return path to entries
        } finally {
            runCatching { channel.disconnect() }
        }
    }

    fun close() {
        runCatching { session.disconnect() }
    }

    companion object {
        private const val CONNECT_TIMEOUT_MS = 20_000

        /**
         * Mở kết nối tới một máy chủ.
         *
         * [password] là thứ người dùng vừa nhập; với khóa riêng thì đó là mật khẩu
         * mở khóa chứ không phải mật khẩu đăng nhập.
         */
        fun open(server: Server, password: String): SshSession {
            val jsch = JSch()
            if (server.privateKey.isNotBlank()) {
                jsch.addIdentity(
                    server.name.ifEmpty { server.host },
                    server.privateKey.toByteArray(Charsets.UTF_8),
                    null,
                    password.toByteArray(Charsets.UTF_8),
                )
            }

            val seen = StringBuilder()
            jsch.hostKeyRepository = TofuHostKeys(server.hostKey, seen)

            val session = jsch.getSession(server.user, server.host, server.port)
            if (server.privateKey.isBlank()) {
                // Bản nhận String đã bị bỏ: nó giữ mật khẩu trong một chuỗi bất
                // biến nằm lại trong bộ nhớ cho tới khi bộ dọn rác đi qua.
                session.setPassword(password.toByteArray(Charsets.UTF_8))
            }
            // Máy chủ có thể hỏi thêm bằng keyboard-interactive thay vì nhận mật
            // khẩu thẳng; trả lời bằng chính mật khẩu đã nhập.
            session.userInfo = PasswordOnly(password)
            session.setConfig("PreferredAuthentications", "publickey,keyboard-interactive,password")

            try {
                session.connect(CONNECT_TIMEOUT_MS)
            } catch (e: JSchException) {
                throw translate(e, seen.toString())
            }
            return SshSession(session, seen.toString())
        }

        /** Đổi lỗi của JSch thành lỗi giao diện dịch được. */
        private fun translate(e: JSchException, seen: String): SshError {
            val message = e.message.orEmpty()
            return when {
                message.contains(TofuHostKeys.CHANGED_MARKER) ->
                    SshError(SshError.Reason.HOST_KEY_CHANGED, seen)
                message.contains("Auth fail", ignoreCase = true) ||
                    message.contains("Auth cancel", ignoreCase = true) ->
                    SshError(SshError.Reason.AUTH_FAILED, message)
                e.cause != null || message.contains("timeout", ignoreCase = true) ->
                    SshError(SshError.Reason.UNREACHABLE, message)
                else -> SshError(SshError.Reason.OTHER, message)
            }
        }

        /** Vân tay SHA-256 của một khóa máy chủ, cùng dạng panel dùng. */
        fun fingerprint(key: ByteArray): String {
            val digest = MessageDigest.getInstance("SHA-256").digest(key)
            // OpenSSH bỏ dấu "=" đệm ở cuối; giữ đúng dạng đó để vân tay hiện ra
            // ở đây so được thẳng với vân tay panel và ssh-keygen in ra.
            return "SHA256:" + Base64.getEncoder().withoutPadding().encodeToString(digest)
        }
    }
}

/** Một mục trong thư mục trên máy chủ. */
data class RemoteFile(val name: String, val size: Long, val isDir: Boolean, val isLink: Boolean)

/**
 * Kho khóa máy chủ theo cách tin từ lần gặp đầu tiên.
 *
 * Lần đầu thì nhận khóa và ghi lại vân tay; từ lần sau khóa phải khớp. Nhận bừa
 * mọi khóa là mở cửa cho người đứng giữa, mà bắt người dùng tự chép khóa vào
 * trước khi kết nối lần đầu thì không ai làm.
 */
private class TofuHostKeys(private val known: String, private val seen: StringBuilder) : HostKeyRepository {

    override fun check(host: String?, key: ByteArray?): Int {
        val fingerprint = if (key == null) "" else SshSession.fingerprint(key)
        seen.setLength(0)
        seen.append(fingerprint)

        if (known.isEmpty()) return HostKeyRepository.OK
        return if (known == fingerprint) HostKeyRepository.OK else HostKeyRepository.CHANGED
    }

    // JSch tự gọi add sau khi check trả về OK; không có gì để làm vì bên gọi mới
    // là nơi biết ghi vân tay vào đâu.
    override fun add(hostkey: HostKey?, ui: UserInfo?) = Unit

    override fun remove(host: String?, type: String?) = Unit

    override fun remove(host: String?, type: String?, key: ByteArray?) = Unit

    override fun getKnownHostsRepositoryID(): String = "sunpanel"

    override fun getHostKey(): Array<HostKey> = emptyArray()

    override fun getHostKey(host: String?, type: String?): Array<HostKey> = emptyArray()

    companion object {
        /** Chuỗi JSch nhét vào lỗi khi kho khóa trả về CHANGED. */
        const val CHANGED_MARKER = "HostKey has been changed"
    }
}

/**
 * Trả lời mọi câu hỏi xác thực bằng đúng một mật khẩu.
 *
 * Không hỏi lại người dùng giữa chừng: mật khẩu đã được hỏi trước khi kết nối,
 * và một hộp thoại bật lên từ luồng nền thì không có chỗ nào để hiện.
 */
private class PasswordOnly(private val password: String) : UserInfo {
    override fun getPassphrase(): String = password

    override fun getPassword(): String = password

    override fun promptPassword(message: String?): Boolean = true

    override fun promptPassphrase(message: String?): Boolean = true

    // Khóa máy chủ do TofuHostKeys quyết định, không phải bằng một câu hỏi ở đây.
    override fun promptYesNo(message: String?): Boolean = false

    override fun showMessage(message: String?) = Unit
}
