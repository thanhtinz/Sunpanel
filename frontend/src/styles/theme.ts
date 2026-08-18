import type { GlobalThemeOverrides } from 'naive-ui'

/**
 * Bảng màu của panel.
 *
 * Nền trung tính ám xanh lá rất nhạt thay vì xám lạnh: xám lạnh là mặc định của
 * mọi khung quản trị, còn sắc ám này gắn với chính màu thương hiệu nên panel
 * trông như một sản phẩm chứ không như một bản mẫu.
 *
 * Xanh lá là màu HÀNH ĐỘNG — nó chỉ xuất hiện ở nút bấm và trạng thái tốt.
 * Hổ phách là màu NHẬN DIỆN — chỉ dùng cho dấu hiệu thương hiệu và mức "cần chú
 * ý". Tách vai trò như vậy để màu mang thông tin, không phải để trang trí.
 */
export const palette = {
  light: {
    bg: '#f4f6f3',
    surface: '#ffffff',
    surfaceSunken: '#eef1ed',
    border: '#e0e4dd',
    borderStrong: '#cdd3c8',
    text: '#161917',
    textMuted: '#6a706c',
    textFaint: '#949a95',
    action: '#15a34a',
    actionHover: '#128b3f',
    actionPressed: '#0f7434',
    sun: '#e8930c',
    warn: '#d97706',
    danger: '#dc2626',
    info: '#2563eb',
  },
  dark: {
    bg: '#0e100f',
    surface: '#171a18',
    surfaceSunken: '#101312',
    border: '#262a27',
    borderStrong: '#343a36',
    text: '#e8ebe8',
    textMuted: '#9aa19c',
    textFaint: '#6d746f',
    action: '#31b866',
    actionHover: '#45c777',
    actionPressed: '#28a058',
    sun: '#f2a93b',
    warn: '#e08a2b',
    danger: '#ef4444',
    info: '#5b8cf5',
  },
} as const

/** Bán kính bo góc dùng chung, đủ mềm để không cứng nhắc, đủ nhỏ để còn nghiêm túc. */
const radius = { card: '10px', control: '7px' } as const

/** Bóng đổ: đúng MỘT mức.
 *
 * Nhiều mức bóng khiến giao diện trông như các mảnh rời chồng lên nhau; một mức
 * duy nhất giữ mọi thẻ nằm trên cùng một mặt phẳng và để đường viền làm việc
 * phân tách. */
const shadow = {
  light: '0 1px 2px rgba(22, 25, 23, 0.04), 0 1px 3px rgba(22, 25, 23, 0.06)',
  dark: '0 1px 2px rgba(0, 0, 0, 0.3), 0 1px 3px rgba(0, 0, 0, 0.35)',
} as const

/** Dựng bộ ghi đè theme cho Naive UI từ bảng màu. */
function build(mode: 'light' | 'dark'): GlobalThemeOverrides {
  const c = palette[mode]
  const elevation = shadow[mode]

  return {
    common: {
      primaryColor: c.action,
      primaryColorHover: c.actionHover,
      primaryColorPressed: c.actionPressed,
      primaryColorSuppl: c.actionHover,

      infoColor: c.info,
      successColor: c.action,
      warningColor: c.warn,
      errorColor: c.danger,

      textColorBase: c.text,
      textColor1: c.text,
      textColor2: c.text,
      textColor3: c.textMuted,

      bodyColor: c.bg,
      cardColor: c.surface,
      modalColor: c.surface,
      popoverColor: c.surface,
      tableColor: c.surface,
      inputColor: mode === 'light' ? c.surface : c.surfaceSunken,

      borderColor: c.border,
      dividerColor: c.border,

      borderRadius: radius.control,
      borderRadiusSmall: '5px',

      // Thang chữ: 13px là cỡ nền của một bảng điều khiển dày số liệu — đủ đọc
      // mà vẫn xếp được nhiều thông tin trên một màn hình.
      fontSize: '14px',
      fontSizeMini: '11px',
      fontSizeTiny: '12px',
      fontSizeSmall: '13px',
      fontSizeMedium: '14px',
      fontSizeLarge: '15px',
      fontSizeHuge: '17px',

      heightMini: '24px',
      heightTiny: '26px',
      heightSmall: '32px',
      heightMedium: '36px',
      heightLarge: '40px',

      boxShadow1: elevation,
      boxShadow2: elevation,
      boxShadow3: elevation,
    },

    Card: {
      borderRadius: radius.card,
      color: c.surface,
      borderColor: c.border,
      titleFontSizeSmall: '15px',
      titleFontWeight: '600',
      paddingSmall: '14px 16px',
    },

    Button: {
      fontWeight: '500',
      borderRadiusSmall: radius.control,
      borderRadiusMedium: radius.control,
    },

    DataTable: {
      thColor: mode === 'light' ? c.surfaceSunken : c.surfaceSunken,
      thTextColor: c.textMuted,
      thFontWeight: '600',
      tdColorHover: mode === 'light' ? '#f7f9f6' : '#1c201e',
      borderColor: c.border,
      thPaddingSmall: '9px 12px',
      tdPaddingSmall: '11px 12px',
    },

    Menu: {
      itemHeight: '38px',
      borderRadius: radius.control,
      itemColorActive: mode === 'light' ? 'rgba(21, 163, 74, 0.10)' : 'rgba(49, 184, 102, 0.14)',
      itemColorActiveHover: mode === 'light' ? 'rgba(21, 163, 74, 0.14)' : 'rgba(49, 184, 102, 0.18)',
      itemTextColorActive: c.action,
      itemTextColorActiveHover: c.action,
      itemIconColorActive: c.action,
      itemIconColorActiveHover: c.action,
      itemTextColor: c.textMuted,
      itemIconColor: c.textMuted,
      fontSize: '14px',
    },

    Layout: {
      siderColor: c.surface,
      headerColor: c.surface,
      color: c.bg,
    },

    Tag: {
      borderRadius: '5px',
    },

    Tabs: {
      tabFontWeightActive: '600',
      tabTextColorActiveLine: c.text,
    },

    Alert: {
      borderRadius: radius.card,
    },

    Input: {
      borderRadius: radius.control,
    },

    Progress: {
      railColor: mode === 'light' ? '#e6eae3' : '#242825',
    },
  }
}

export const lightOverrides = build('light')
export const darkOverrides = build('dark')
