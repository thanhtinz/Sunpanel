package com.sunpanel.app

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext

// Cùng bảng màu với panel: xanh lá của nút chính và xanh đêm của thanh bên.
private val Green = Color(0xFF18A058)
private val GreenDark = Color(0xFF63E2B7)
private val Navy = Color(0xFF011C3E)

private val LightScheme =
    lightColorScheme(
        primary = Green,
        onPrimary = Color.White,
        secondary = Navy,
        background = Color(0xFFF4F6F3),
        surface = Color.White,
    )

private val DarkScheme =
    darkColorScheme(
        primary = GreenDark,
        onPrimary = Color(0xFF00251A),
        secondary = Color(0xFF8AB4F8),
    )

/**
 * Chủ đề của ứng dụng.
 *
 * Android 12 trở lên lấy màu theo hình nền người dùng đặt — đó là thứ họ mong đợi
 * ở một ứng dụng gốc, nên để hệ thống quyết định khi có thể, và chỉ dùng màu của
 * panel khi máy không hỗ trợ.
 */
@Composable
fun SunPanelTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val context = LocalContext.current
    val scheme =
        when {
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.S ->
                if (darkTheme) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
            darkTheme -> DarkScheme
            else -> LightScheme
        }

    MaterialTheme(colorScheme = scheme, content = content)
}
