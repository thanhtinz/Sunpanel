package com.sunpanel.app

import java.net.URI
import java.net.URISyntaxException
import org.json.JSONArray
import org.json.JSONObject

/** Một panel đã lưu trong ứng dụng. */
data class Server(
    val id: String,
    val name: String,
    val url: String,
    val last: Boolean = false,
    /**
     * Vân tay SHA-256 của chứng chỉ mà người dùng đã đồng ý tin, dạng hoa không
     * dấu phân cách. Rỗng nghĩa là chưa từng chấp nhận chứng chỉ tự ký nào.
     */
    val certFingerprint: String = "",
)

/** Lỗi địa chỉ panel không dùng được, kèm mã để giao diện tự dịch. */
class InvalidAddress(val reason: Reason) : Exception(reason.name) {
    enum class Reason { EMPTY, MALFORMED, SCHEME, HOST }
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
    fun save(list: List<Server>, id: String, name: String, url: String): List<Server> {
        val address = normalizeUrl(url)
        val label = name.trim().ifEmpty { address }

        if (id.isEmpty()) {
            return list + Server(id = nextId(list), name = label, url = address)
        }
        return list.map { server ->
            if (server.id != id) {
                server
            } else {
                // Đổi địa chỉ là đổi máy: vân tay chứng chỉ cũ không còn nói lên
                // điều gì về máy mới, giữ lại là tự bịt mắt mình.
                val keepFingerprint = if (sameOrigin(server.url, address)) server.certFingerprint else ""
                server.copy(name = label, url = address, certFingerprint = keepFingerprint)
            }
        }
    }

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
            val url = item.optString("url")
            if (id.isEmpty() || url.isEmpty()) continue
            list.add(
                Server(
                    id = id,
                    name = item.optString("name").ifEmpty { url },
                    url = url,
                    last = item.optBoolean("last", false),
                    certFingerprint = item.optString("cert"),
                )
            )
        }
        return list
    }
}
