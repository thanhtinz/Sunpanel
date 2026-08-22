package com.sunpanel.app

/** Thông tin máy chủ đọc một lần lúc mới kết nối. */
data class HostInfo(
    val hostname: String = "",
    val os: String = "",
    val kernel: String = "",
    val cpuCores: Int = 0,
)

/** Mức dùng tài nguyên ngay lúc đo, tính theo phần trăm. */
data class Metrics(
    val cpu: Double = 0.0,
    val memory: Double = 0.0,
    val disk: Double = 0.0,
    val load1: Double = 0.0,
)

/**
 * Đọc thông tin và mức dùng tài nguyên của máy chủ từ xa.
 *
 * Câu lệnh giữ nguyên như bản panel dùng (`pkg/sshx/info.go` và `metrics.go`) để
 * cùng một máy chủ cho ra cùng một con số dù xem bằng panel, ứng dụng máy tính
 * hay điện thoại.
 */
object Readings {

    /**
     * Khoảng cách giữa hai lần đọc /proc/stat.
     *
     * Mức dùng CPU là hiệu của hai lần đọc chứ không phải một con số đọc thẳng:
     * /proc/stat đếm tổng thời gian từ lúc khởi động, nên đọc một lần chỉ cho ra
     * mức trung bình kể từ ngày bật máy.
     */
    private const val CPU_GAP_SECONDS = 0.3

    /** Gộp mọi thứ cần biết vào một lệnh: mỗi phiên SSH là một vòng đi về qua mạng. */
    val INFO_COMMAND = """
        echo "SP_HOST=${'$'}(hostname 2>/dev/null)"
        echo "SP_KERNEL=${'$'}(uname -r 2>/dev/null)"
        . /etc/os-release 2>/dev/null; echo "SP_OS=${'$'}{PRETTY_NAME:-${'$'}(uname -s)}"
        echo "SP_CPU=${'$'}(nproc 2>/dev/null || echo 0)"
    """.trimIndent()

    val METRICS_COMMAND = """
        awk '/^cpu /{print "SP_CPU1=" ${'$'}2+${'$'}3+${'$'}4+${'$'}6+${'$'}7+${'$'}8 " " ${'$'}5}' /proc/stat 2>/dev/null
        sleep $CPU_GAP_SECONDS
        awk '/^cpu /{print "SP_CPU2=" ${'$'}2+${'$'}3+${'$'}4+${'$'}6+${'$'}7+${'$'}8 " " ${'$'}5}' /proc/stat 2>/dev/null
        awk '/^MemTotal:/{t=${'$'}2} /^MemAvailable:/{a=${'$'}2} END{print "SP_MEM=" t " " a}' /proc/meminfo 2>/dev/null
        df -kP / 2>/dev/null | awk 'NR==2{print "SP_DISK=" ${'$'}2 " " ${'$'}3}'
        echo "SP_LOAD=${'$'}(cut -d' ' -f1 /proc/loadavg 2>/dev/null)"
    """.trimIndent()

    /** Đọc kết quả của [INFO_COMMAND]. */
    fun parseInfo(output: String): HostInfo {
        val fields = fields(output)
        return HostInfo(
            hostname = fields["SP_HOST"].orEmpty(),
            kernel = fields["SP_KERNEL"].orEmpty(),
            os = fields["SP_OS"].orEmpty(),
            cpuCores = fields["SP_CPU"]?.toIntOrNull() ?: 0,
        )
    }

    /** Đọc kết quả của [METRICS_COMMAND]. */
    fun parseMetrics(output: String): Metrics {
        val fields = fields(output)

        val (busy1, idle1) = twoNumbers(fields["SP_CPU1"])
        val (busy2, idle2) = twoNumbers(fields["SP_CPU2"])
        val busy = busy2 - busy1
        val idle = idle2 - idle1
        val cpu = percent(busy, busy + idle)

        val (memTotal, memAvailable) = twoNumbers(fields["SP_MEM"])
        val (diskTotal, diskUsed) = twoNumbers(fields["SP_DISK"])

        return Metrics(
            cpu = cpu,
            memory = percent(memTotal - memAvailable, memTotal),
            disk = percent(diskUsed, diskTotal),
            load1 = fields["SP_LOAD"]?.toDoubleOrNull() ?: 0.0,
        )
    }

    /** Tách các dòng "SP_TÊN=giá trị" thành bảng tra. */
    private fun fields(output: String): Map<String, String> {
        val result = mutableMapOf<String, String>()
        for (line in output.lineSequence()) {
            val at = line.indexOf('=')
            if (at <= 0) continue
            val name = line.substring(0, at).trim()
            if (!name.startsWith("SP_")) continue
            result[name] = line.substring(at + 1).trim()
        }
        return result
    }

    /** Đọc hai số cách nhau bằng dấu cách. */
    private fun twoNumbers(value: String?): Pair<Long, Long> {
        val parts = value?.trim()?.split(Regex("\\s+")).orEmpty()
        if (parts.size < 2) return 0L to 0L
        return (parts[0].toLongOrNull() ?: 0L) to (parts[1].toLongOrNull() ?: 0L)
    }

    /**
     * Tính phần trăm và kẹp trong khoảng 0–100.
     *
     * Hai lần đọc /proc/stat có thể ra hiệu âm khi máy chủ vừa khởi động lại giữa
     * chừng, và một thanh dài âm hay dài hơn 100% thì vô nghĩa.
     */
    private fun percent(part: Long, total: Long): Double {
        if (total <= 0) return 0.0
        val value = part.toDouble() * 100.0 / total.toDouble()
        return value.coerceIn(0.0, 100.0)
    }
}
