package com.sunpanel.app

import android.net.http.SslCertificate
import java.security.MessageDigest

/**
 * Vân tay chứng chỉ, dùng cho cách tin theo lần gặp đầu tiên.
 *
 * Rất nhiều panel chạy chứng chỉ tự ký, mà WebView thì chặn thẳng. Bỏ qua toàn
 * bộ lỗi chứng chỉ là mở cửa cho người đứng giữa; hỏi một lần rồi nhớ vân tay
 * giữ được cảnh báo cho lần chứng chỉ thật sự đổi — đúng cách panel đối xử với
 * khóa máy chủ SSH.
 */
object Certificates {

    /** Vân tay SHA-256 dạng hoa, không dấu phân cách. Rỗng nếu không đọc được. */
    fun fingerprint(certificate: SslCertificate?): String {
        val encoded = certificate?.let { SslCertificate.saveState(it).getByteArray("x509-certificate") }
        if (encoded == null || encoded.isEmpty()) return ""

        val digest = MessageDigest.getInstance("SHA-256").digest(encoded)
        val out = StringBuilder(digest.size * 2)
        for (byte in digest) out.append("%02X".format(byte))
        return out.toString()
    }

    /** Cắt vân tay thành từng cặp để người dùng đối chiếu được bằng mắt. */
    fun readable(fingerprint: String): String = fingerprint.chunked(2).joinToString(":")
}
