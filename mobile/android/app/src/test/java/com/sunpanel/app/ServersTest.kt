package com.sunpanel.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

class ServersTest {

    @Test
    fun `them http khi thieu giao thuc`() {
        assertEquals(
            "http://10.0.0.5:9527/qvzQfJuo56JQ/",
            Servers.normalizeUrl("10.0.0.5:9527/qvzQfJuo56JQ"),
        )
    }

    @Test
    fun `them gach cheo cuoi duong dan bi mat`() {
        assertEquals(
            "https://panel.example.com/abc123/",
            Servers.normalizeUrl("https://panel.example.com/abc123"),
        )
    }

    @Test
    fun `giu nguyen dia chi da dung`() {
        assertEquals(
            "https://panel.example.com/abc123/",
            Servers.normalizeUrl("  https://panel.example.com/abc123/  "),
        )
    }

    @Test
    fun `dia chi khong co duong dan van co gach cheo`() {
        assertEquals("http://127.0.0.1:9527/", Servers.normalizeUrl("127.0.0.1:9527"))
    }

    @Test
    fun `tu choi dia chi khong dung duoc`() {
        for (raw in listOf("", "   ", "ftp://panel.example.com", "file:///etc/passwd")) {
            try {
                val got = Servers.normalizeUrl(raw)
                fail("normalizeUrl(\"$raw\") = \"$got\", mong đợi lỗi")
            } catch (expected: InvalidAddress) {
                // đúng như mong đợi
            }
        }
    }

    @Test
    fun `luu sinh dinh danh khong trung`() {
        var list = Servers.save(emptyList(), Server(id = "", name = "Máy chủ nhà", url = "127.0.0.1:9527/qvzQfJuo56JQ"))
        list = Servers.save(list, Server(id = "", name = "VPS Sài Gòn", url = "https://sg.example.com/abc/"))

        assertEquals(2, list.size)
        assertEquals("http://127.0.0.1:9527/qvzQfJuo56JQ/", list[0].url)
        assertTrue(list[0].id != list[1].id)
    }

    @Test
    fun `ten trong thi lay dia chi`() {
        val list = Servers.save(emptyList(), Server(id = "", name = "   ", url = "a.example.com/abc"))
        assertEquals("http://a.example.com/abc/", list[0].name)
    }

    @Test
    fun `sua khong tao them muc moi`() {
        var list = Servers.save(emptyList(), Server(id = "", name = "A", url = "a.example.com"))
        list = Servers.save(list, Server(id = list[0].id, name = "A mới", url = "a.example.com"))

        assertEquals(1, list.size)
        assertEquals("A mới", list[0].name)
    }

    @Test
    fun `doi dia chi thi bo van tay chung chi cu`() {
        var list = Servers.save(emptyList(), Server(id = "", name = "A", url = "https://a.example.com/abc"))
        list = Servers.trustCertificate(list, list[0].id, "AABB")
        assertEquals("AABB", list[0].certFingerprint)

        // Đổi tên thôi thì vân tay còn nguyên: vẫn đúng máy đó.
        list = Servers.save(list, Server(id = list[0].id, name = "A khác", url = "https://a.example.com/abc"))
        assertEquals("AABB", list[0].certFingerprint)

        // Đổi sang máy khác thì vân tay cũ không còn nói lên điều gì.
        list = Servers.save(list, Server(id = list[0].id, name = "A khác", url = "https://b.example.com/abc"))
        assertEquals("", list[0].certFingerprint)
    }

    @Test
    fun `chi mot may chu duoc danh dau mo gan nhat`() {
        var list = Servers.save(emptyList(), Server(id = "", name = "A", url = "a.example.com"))
        list = Servers.save(list, Server(id = "", name = "B", url = "b.example.com"))

        assertNull(Servers.last(list))

        list = Servers.markLast(list, list[1].id)
        assertEquals(list[1].id, Servers.last(list)?.id)

        list = Servers.markLast(list, list[0].id)
        assertEquals(1, list.count { it.last })
        assertEquals(list[0].id, Servers.last(list)?.id)
    }

    @Test
    fun `xoa roi them lai dung dinh danh trong`() {
        var list = Servers.save(emptyList(), Server(id = "", name = "A", url = "a.example.com"))
        list = Servers.save(list, Server(id = "", name = "B", url = "b.example.com"))
        list = Servers.remove(list, list[0].id)

        assertEquals(1, list.size)
        list = Servers.save(list, Server(id = "", name = "C", url = "c.example.com"))
        assertEquals(2, list.size)
        assertEquals(2, list.map { it.id }.toSet().size)
    }

    @Test
    fun `ghi roi doc lai giu nguyen du lieu`() {
        var list = Servers.save(emptyList(), Server(id = "", name = "Máy chủ nhà", url = "127.0.0.1:9527/qvzQfJuo56JQ"))
        list = Servers.save(list, Server(id = "", name = "VPS Sài Gòn", url = "https://sg.example.com/abc/"))
        list = Servers.markLast(list, list[1].id)
        list = Servers.trustCertificate(list, list[1].id, "DEADBEEF")

        assertEquals(list, Servers.decode(Servers.encode(list)))
    }

    @Test
    fun `luu may chu ssh`() {
        val list = Servers.save(emptyList(), Server(id = "", name = "", kind = Kind.SSH, host = "203.0.113.10", user = "root", port = 0))

        assertEquals(1, list.size)
        assertEquals(22, list[0].port)
        assertEquals("root@203.0.113.10", list[0].name)
        assertEquals("root@203.0.113.10:22", list[0].label())
    }

    @Test
    fun `tu choi may chu ssh thieu thong tin`() {
        val hong = listOf(
            Server(id = "", kind = Kind.SSH, user = "root"),
            Server(id = "", kind = Kind.SSH, host = "10.0.0.1"),
            Server(id = "", kind = Kind.SSH, host = "10.0.0.1 rm -rf", user = "root"),
            Server(id = "", kind = Kind.SSH, host = "10.0.0.1", user = "root", port = 70000),
        )
        for (draft in hong) {
            try {
                Servers.save(emptyList(), draft)
                fail("mong đợi lỗi với $draft")
            } catch (expected: InvalidAddress) {
                // đúng như mong đợi
            }
        }
    }

    @Test
    fun `doi may chu thi bo khoa cu`() {
        var list = Servers.save(emptyList(), Server(id = "", kind = Kind.SSH, host = "203.0.113.10", user = "root"))
        list = Servers.trustHostKey(list, list[0].id, "SHA256:abc")
        assertEquals("SHA256:abc", list[0].hostKey)

        // Đổi mỗi tên thì vẫn là máy đó.
        list = Servers.save(list, list[0].copy(name = "Máy chính"))
        assertEquals("SHA256:abc", list[0].hostKey)

        // Đổi sang máy khác thì khóa cũ không còn nói lên điều gì.
        list = Servers.save(list, list[0].copy(host = "198.51.100.7"))
        assertEquals("", list[0].hostKey)
    }

    @Test
    fun `ghi roi doc lai giu nguyen may chu ssh`() {
        var list = Servers.save(emptyList(), Server(id = "", kind = Kind.SSH, host = "10.0.0.9", user = "quantri", port = 2222, password = "bí mật"))
        list = Servers.trustHostKey(list, list[0].id, "SHA256:xyz")

        assertEquals(list, Servers.decode(Servers.encode(list)))
    }

    @Test
    fun `ban ghi cu khong co kieu hieu la panel`() {
        val list = Servers.decode("""[{"id":"s1","name":"Máy chủ nhà","url":"http://127.0.0.1:9527/abc/","last":true}]""")

        assertEquals(1, list.size)
        assertEquals(Kind.PANEL, list[0].kind)
        assertEquals("s1", Servers.last(list)?.id)
    }

    @Test
    fun `du lieu hong tra ve danh sach rong`() {
        assertTrue(Servers.decode(null).isEmpty())
        assertTrue(Servers.decode("").isEmpty())
        assertTrue(Servers.decode("{ đây không phải mảng").isEmpty())
    }

    @Test
    fun `bo qua muc thieu dia chi thay vi mat ca danh sach`() {
        val list = Servers.decode("""[{"id":"s1"},{"id":"s2","url":"http://b.example.com/"}]""")
        assertEquals(1, list.size)
        assertEquals("s2", list[0].id)
    }

    @Test
    fun `bo qua may chu ssh thieu dia chi`() {
        val list = Servers.decode("""[{"id":"s1","kind":"ssh","user":"root"},{"id":"s2","kind":"ssh","host":"10.0.0.1","user":"root"}]""")
        assertEquals(1, list.size)
        assertEquals("s2", list[0].id)
        assertEquals(Kind.SSH, list[0].kind)
    }

    @Test
    fun `cung mot may khi chi khac duong dan`() {
        assertTrue(Servers.sameOrigin("https://a.example.com/abc/", "https://a.example.com/khac/"))
        assertFalse(Servers.sameOrigin("https://a.example.com/abc/", "http://a.example.com/abc/"))
        assertFalse(Servers.sameOrigin("https://a.example.com/abc/", "https://a.example.com:8443/abc/"))
        assertFalse(Servers.sameOrigin("https://a.example.com/", "không phải địa chỉ"))
    }
}
