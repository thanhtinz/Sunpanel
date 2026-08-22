package com.sunpanel.app

import android.content.Context
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.State

/**
 * Danh sách máy chủ lưu trên máy.
 *
 * Dùng SharedPreferences chứ không phải tệp riêng: dữ liệu chỉ vài dòng, và kho
 * này nằm trong vùng riêng của ứng dụng nên ứng dụng khác không đọc được địa chỉ
 * panel — thứ có kèm đường dẫn bí mật đứng giữa người lạ và trang đăng nhập.
 */
class ServerStore(context: Context) {

    private val prefs = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    private val state = mutableStateOf(Servers.decode(prefs.getString(KEY, null)))

    /** Danh sách hiện tại, giao diện đọc trực tiếp và tự vẽ lại khi đổi. */
    val servers: State<List<Server>> get() = state

    fun save(id: String, name: String, url: String) {
        write(Servers.save(state.value, id, name, url))
    }

    fun remove(id: String) = write(Servers.remove(state.value, id))

    fun markLast(id: String) = write(Servers.markLast(state.value, id))

    fun trustCertificate(id: String, fingerprint: String) =
        write(Servers.trustCertificate(state.value, id, fingerprint))

    fun last(): Server? = Servers.last(state.value)

    fun byId(id: String): Server? = state.value.firstOrNull { it.id == id }

    private fun write(list: List<Server>) {
        state.value = list
        prefs.edit().putString(KEY, Servers.encode(list)).apply()
    }

    private companion object {
        const val PREFS = "sunpanel"
        const val KEY = "servers"
    }
}
