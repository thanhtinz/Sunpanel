package com.sunpanel.app

import java.net.URI
import java.net.URISyntaxException
import org.json.JSONArray
import org.json.JSONObject

/** Cách ứng dụng nối tới một máy chủ. */
enum class Kind {
    /** Mở giao diện panel đã cài trên máy chủ. */
    PANEL,
    /** Nối thẳng vào máy chủ qua SSH, không cần cài gì trên đó. */
    SSH;

    companion object {
        /** Đọc kiểu từ dữ liệu đã lưu; bản ghi cũ không có trường này. */
        fun of(text: String?): Kind = if (text.equals("ssh", ignoreCase = true)) SSH else PANEL
    }
}

/** Một máy chủ đã lưu trong ứng dụng. */
data class Server(
    val id: String,
    val name: String = "",
    val kind: Kind = Kind.PANEL,

    /** Địa chỉ panel kèm đường dẫn bí mật. Chỉ dùng cho [Kind.PANEL]. */
    val url: String = "",

    /** Các trường dưới đây chỉ dùng cho [Kind.SSH]. */
    val host: String = "",
    val port: Int = 22,
    val user: String = "",
    /** Nội dung khóa riêng dạng PEM, người dùng dán vào. */
    val privateKey: String = "",
    /** Mật khẩu, chỉ được ghi khi người dùng tự chọn nhớ. */
    val password: String = "",
    /** Vân tay khóa máy chủ SSH đã ghi nhận ở lần kết nối đầu. */
    val hostKey: String = "",

    val last: Boolean = false,
    /**
     * Vân tay SHA-256 của chứng chỉ TLS mà người dùng đã đồng ý tin, dạng hoa
     * không dấu phân cách. Chỉ dùng cho [Kind.PANEL].
     */
    val certFingerprint: String = "",
) {
    /** Dòng địa chỉ hiện dưới tên máy chủ. */
    fun label(): String = if (kind == Kind.SSH) "$user@$host:$port" else url
}

/** Lỗi thông tin máy chủ không dùng được, kèm mã để giao diện tự dịch. */
class InvalidAddress(val reason: Reason) : Exception(reason.name) {
    enum class Reason { EMPTY, MALFORMED, SCHEME, HOST, USER, PORT }
}

/**
 * Toàn bộ phần xử lý danh sách máy chủ, viết thuần Kotlin.
 *
 * Không đụng gì tới Android nên chạy thẳng được trong bài kiểm thử trên JVM —
 * đây là chỗ dễ sai nhất của ứng dụng (địa chỉ gõ tay, dữ liệu cũ trên máy) nên
 * phải kiểm được mà không cần máy ảo.
 */
object Servers {

    /** Kiểm và chuẩn hóa địa chỉ panel người dùng gõ vào. */
    fun normalizeUrl(raw: String): String {
        var text = raw.trim()
        if (text.isEmpty()) throw InvalidAddress(InvalidAddress.Reason.EMPTY)

        // Thiếu giao thức thì hiểu là http: panel chạy HTTP cho tới khi người dùng
        // gắn chứng chỉ, và bắt gõ đủ "http://" chỉ tạo thêm chỗ sai.
        if (!text.contains("://")) text = "http://$text"

        val parsed =
            try {
                URI(text)
            } catch (e: URISyntaxException) {
                throw InvalidAddress(InvalidAddress.Reason.MALFORMED)
            }

        val scheme = parsed.scheme?.lowercase()
        if (scheme != "http" && scheme != "https") throw InvalidAddress(InvalidAddress.Reason.SCHEME)

        val authority = parsed.authority
        if (authority.isNullOrEmpty()) throw InvalidAddress(InvalidAddress.Reason.HOST)

        // Đường dẫn bí mật phải kết thúc bằng dấu gạch chéo, nếu không thẻ base của
        // panel phân giải sai và giao diện nạp tài nguyên từ thư mục cha.
        var path = parsed.rawPath ?: ""
        if (!path.endsWith("/")) path += "/"

        return "$scheme://$authority$path"
    }

    /** Tách máy chủ trong địa chỉ đang mở, để biết trang nào thuộc panel nào. */
    fun sameOrigin(a: String, b: String): Boolean {
        return try {
            val left = URI(a)
            val right = URI(b)
            left.scheme.equals(right.scheme, ignoreCase = true) &&
                left.authority.equals(right.authority, ignoreCase = true)
        } catch (e: URISyntaxException) {
            false
        }
    }

    /** Sinh định danh chưa dùng trong danh sách. */
    fun nextId(list: List<Server>): String {
        var next = 1
        while (list.any { it.id == "s$next" }) next++
        return "s$next"
    }

    /** Thêm mới hoặc cập nhật một máy chủ, trả về danh sách mới. */
    fun save(list: List<Server>, draft: Server): List<Server> {
        val clean = normalize(draft)

        if (clean.id.isEmpty()) {
            return list + clean.copy(id = nextId(list))
        }
        return list.map { server ->
            if (server.id != clean.id) {
                server
            } else {
                // Đổi địa chỉ là đổi máy: vân tay cũ không còn nói lên điều gì về
                // máy mới, giữ lại là tự bịt mắt mình.
                val sameMachine =
                    if (clean.kind == Kind.SSH) server.host == clean.host && server.port == clean.port
                    else sameOrigin(server.url, clean.url)
                clean.copy(
                    last = server.last,
                    certFingerprint = if (sameMachine) server.certFingerprint else "",
                    hostKey = if (sameMachine) server.hostKey else "",
                )
            }
        }
    }

    /** Kiểm và dọn một bản ghi trước khi lưu. */
    fun normalize(draft: Server): Server {
        if (draft.kind == Kind.SSH) {
            val host = draft.host.trim()
            val user = draft.user.trim()
            if (host.isEmpty()) throw InvalidAddress(InvalidAddress.Reason.HOST)
            if (host.any { it.isWhitespace() } || host.contains('/')) {
                throw InvalidAddress(InvalidAddress.Reason.MALFORMED)
            }
            if (user.isEmpty()) throw InvalidAddress(InvalidAddress.Reason.USER)

            val port = if (draft.port == 0) 22 else draft.port
            if (port !in 1..65535) throw InvalidAddress(InvalidAddress.Reason.PORT)

            return draft.copy(
                name = draft.name.trim().ifEmpty { "$user@$host" },
                host = host,
                user = user,
                port = port,
                url = "",
                certFingerprint = "",
            )
        }

        val address = normalizeUrl(draft.url)
        return draft.copy(
            kind = Kind.PANEL,
            name = draft.name.trim().ifEmpty { address },
            url = address,
            host = "",
            user = "",
            privateKey = "",
            password = "",
            hostKey = "",
        )
    }

    /** Ghi nhớ khóa máy chủ SSH sau lần kết nối đầu tiên. */
    fun trustHostKey(list: List<Server>, id: String, fingerprint: String): List<Server> =
        list.map { if (it.id == id) it.copy(hostKey = fingerprint) else it }

    /** Xóa một máy chủ khỏi danh sách. */
    fun remove(list: List<Server>, id: String): List<Server> = list.filter { it.id != id }

    /** Đánh dấu máy chủ vừa mở để lần chạy sau tự vào thẳng. */
    fun markLast(list: List<Server>, id: String): List<Server> = list.map { it.copy(last = it.id == id) }

    /** Ghi nhớ chứng chỉ mà người dùng đã đồng ý tin cho một máy chủ. */
    fun trustCertificate(list: List<Server>, id: String, fingerprint: String): List<Server> =
        list.map { if (it.id == id) it.copy(certFingerprint = fingerprint) else it }

    /** Máy chủ mở gần nhất. */
    fun last(list: List<Server>): Server? = list.firstOrNull { it.last }

    /** Chuyển danh sách thành JSON để lưu. */
    fun encode(list: List<Server>): String {
        val array = JSONArray()
        for (server in list) {
            val item = JSONObject()
            item.put("id", server.id)
            item.put("name", server.name)
            item.put("url", server.url)
            item.put("kind", if (server.kind == Kind.SSH) "ssh" else "panel")
            item.put("host", server.host)
            item.put("port", server.port)
            item.put("user", server.user)
            item.put("key", server.privateKey)
            item.put("password", server.password)
            item.put("hostKey", server.hostKey)
            item.put("last", server.last)
            item.put("cert", server.certFingerprint)
            array.put(item)
        }
        return array.toString()
    }

    /**
     * Đọc danh sách từ JSON đã lưu.
     *
     * Dữ liệu hỏng không được làm ứng dụng không mở lên được: bắt đầu lại với danh
     * sách rỗng còn hơn treo ngay màn hình đầu tiên. Từng mục hỏng cũng vậy — mất
     * một máy chủ nhẹ hơn mất cả danh sách.
     */
    fun decode(text: String?): List<Server> {
        if (text.isNullOrBlank()) return emptyList()

        val array =
            try {
                JSONArray(text)
            } catch (e: Exception) {
                return emptyList()
            }

        val list = mutableListOf<Server>()
        for (i in 0 until array.length()) {
            val item = array.optJSONObject(i) ?: continue
            val id = item.optString("id")
            if (id.isEmpty()) continue

            // Bản ghi lưu từ phiên bản trước chỉ có panel nên không ghi trường kind.
            val kind = Kind.of(item.optString("kind"))
            val url = item.optString("url")
            val host = item.optString("host")
            if (kind == Kind.PANEL && url.isEmpty()) continue
            if (kind == Kind.SSH && host.isEmpty()) continue

            list.add(
                Server(
                    id = id,
                    kind = kind,
                    name = item.optString("name").ifEmpty { if (kind == Kind.SSH) host else url },
                    url = url,
                    host = host,
                    port = item.optInt("port", 22).let { if (it <= 0) 22 else it },
                    user = item.optString("user"),
                    privateKey = item.optString("key"),
                    password = item.optString("password"),
                    hostKey = item.optString("hostKey"),
                    last = item.optBoolean("last", false),
                    certFingerprint = item.optString("cert"),
                )
            )
        }
        return list
    }
}
