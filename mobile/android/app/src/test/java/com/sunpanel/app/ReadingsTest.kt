package com.sunpanel.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ReadingsTest {

    @Test
    fun `doc thong tin may chu`() {
        val info = Readings.parseInfo(
            """
            SP_HOST=vps-sai-gon
            SP_KERNEL=6.8.0-45-generic
            SP_OS=Ubuntu 24.04.1 LTS
            SP_CPU=4
            """.trimIndent()
        )

        assertEquals("vps-sai-gon", info.hostname)
        assertEquals("6.8.0-45-generic", info.kernel)
        assertEquals("Ubuntu 24.04.1 LTS", info.os)
        assertEquals(4, info.cpuCores)
    }

    @Test
    fun `thieu dong nao thi de trong dong do`() {
        // Một bản Linux gọn nhẹ có thể không có nproc hay /etc/os-release; thiếu
        // một dòng không được làm hỏng cả phần đọc.
        val info = Readings.parseInfo("SP_HOST=máy-nhỏ\nSP_CPU=\n")

        assertEquals("máy-nhỏ", info.hostname)
        assertEquals(0, info.cpuCores)
        assertEquals("", info.os)
    }

    @Test
    fun `tinh muc dung cpu tu hieu hai lan doc`() {
        // Lần đầu bận 100, rảnh 900; lần sau bận 150, rảnh 950 → bận 50 trên 100.
        val metrics = Readings.parseMetrics(
            """
            SP_CPU1=100 900
            SP_CPU2=150 950
            SP_MEM=8000000 2000000
            SP_DISK=100000 25000
            SP_LOAD=1.25
            """.trimIndent()
        )

        assertEquals(50.0, metrics.cpu, 0.001)
        assertEquals(75.0, metrics.memory, 0.001)
        assertEquals(25.0, metrics.disk, 0.001)
        assertEquals(1.25, metrics.load1, 0.001)
    }

    @Test
    fun `may vua khoi dong lai khong cho ra so am`() {
        // Lần đọc sau nhỏ hơn lần đọc trước: bộ đếm vừa về 0 vì máy khởi động lại.
        val metrics = Readings.parseMetrics("SP_CPU1=900 900\nSP_CPU2=10 10\n")

        assertTrue("CPU âm: ${metrics.cpu}", metrics.cpu >= 0.0)
        assertTrue("CPU quá 100: ${metrics.cpu}", metrics.cpu <= 100.0)
    }

    @Test
    fun `khong co du lieu thi tra ve khong`() {
        val metrics = Readings.parseMetrics("")

        assertEquals(0.0, metrics.cpu, 0.001)
        assertEquals(0.0, metrics.memory, 0.001)
        assertEquals(0.0, metrics.disk, 0.001)
    }

    @Test
    fun `bo qua dong khong phai cua minh`() {
        // Máy chủ có thể in ra lời chào hoặc cảnh báo trước khi lệnh chạy.
        val metrics = Readings.parseMetrics(
            """
            Welcome to Ubuntu 24.04 LTS
            LANG=vi_VN.UTF-8
            SP_MEM=1000 250
            """.trimIndent()
        )

        assertEquals(75.0, metrics.memory, 0.001)
    }

    @Test
    fun `lenh doc thong so co du hai lan doc cpu`() {
        assertTrue(Readings.METRICS_COMMAND.contains("SP_CPU1="))
        assertTrue(Readings.METRICS_COMMAND.contains("SP_CPU2="))
        assertTrue(Readings.METRICS_COMMAND.contains("sleep 0.3"))
        // Dấu đô la phải xuống tới máy chủ nguyên vẹn, không bị Kotlin nội suy.
        assertTrue(Readings.METRICS_COMMAND.contains("$2+$3+$4+$6+$7+$8"))
        assertTrue(Readings.INFO_COMMAND.contains("$(hostname 2>/dev/null)"))
    }
}
