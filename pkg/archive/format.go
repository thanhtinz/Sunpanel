// Package archive nhận biết và giải nén các định dạng tệp nén phổ biến.
//
// Panel phải mở được thứ người dùng thực sự có trong tay: mã nguồn tải từ
// GitHub là .zip, bản sao lưu từ máy chủ cũ là .tar.gz, tệp gửi qua Windows
// thường là .rar hoặc .7z. Đọc được đúng hai định dạng nghĩa là phần còn lại
// phải mở bằng dòng lệnh, mà dòng lệnh chính là thứ panel sinh ra để thay thế.
package archive

import (
	"errors"
	"path/filepath"
	"strings"
)

// Các lỗi dùng chung của gói.
var (
	// ErrUnsupported là định dạng panel chưa mở được.
	ErrUnsupported = errors.New("archive: định dạng không được hỗ trợ")
	// ErrNeedRandomAccess là định dạng đòi đọc ngẫu nhiên nhưng nguồn chỉ đọc tuần tự được.
	ErrNeedRandomAccess = errors.New("archive: định dạng này cần đọc ngẫu nhiên")
	// ErrTooLarge là nội dung giải nén vượt giới hạn cho phép.
	ErrTooLarge = errors.New("archive: nội dung giải nén vượt giới hạn")
	// ErrCorrupt là tệp nén hỏng hoặc không đúng định dạng khai báo.
	ErrCorrupt = errors.New("archive: tệp nén hỏng")
	// ErrEncrypted là tệp nén có mật khẩu.
	ErrEncrypted = errors.New("archive: tệp nén được đặt mật khẩu")
	// ErrUnsafePath là tên mục bên trong tệp nén trỏ ra ngoài thư mục đích.
	ErrUnsafePath = errors.New("archive: tên mục trong tệp nén không an toàn")
)

// Format là một định dạng tệp nén.
type Format string

// Các định dạng nhận biết được.
const (
	FormatZip      Format = "zip"
	FormatTar      Format = "tar"
	FormatTarGz    Format = "tar.gz"
	FormatTarBz2   Format = "tar.bz2"
	FormatTarXz    Format = "tar.xz"
	FormatTarZst   Format = "tar.zst"
	FormatRar      Format = "rar"
	FormatSevenZip Format = "7z"
	FormatGz       Format = "gz"
	FormatBz2      Format = "bz2"
	FormatXz       Format = "xz"
	FormatZst      Format = "zst"
)

// byExtension ánh xạ phần đuôi tên tệp sang định dạng.
//
// Xếp đuôi dài trước: ".tar.gz" phải thắng ".gz", nếu không mọi tệp tar nén
// gzip sẽ được mở ra thành đúng một tệp .tar thay vì cả cây thư mục.
var byExtension = []struct {
	suffix string
	format Format
}{
	{".tar.gz", FormatTarGz}, {".tgz", FormatTarGz},
	{".tar.bz2", FormatTarBz2}, {".tbz", FormatTarBz2}, {".tbz2", FormatTarBz2},
	{".tar.xz", FormatTarXz}, {".txz", FormatTarXz},
	{".tar.zst", FormatTarZst}, {".tzst", FormatTarZst},
	{".tar", FormatTar},
	{".zip", FormatZip}, {".jar", FormatZip}, {".war", FormatZip}, {".apk", FormatZip},
	{".xpi", FormatZip}, {".whl", FormatZip}, {".nupkg", FormatZip}, {".vsix", FormatZip},
	{".rar", FormatRar},
	{".7z", FormatSevenZip},
	{".gz", FormatGz},
	{".bz2", FormatBz2},
	{".xz", FormatXz},
	{".zst", FormatZst},
}

// DetectName đoán định dạng từ tên tệp.
func DetectName(name string) (Format, bool) {
	lower := strings.ToLower(filepath.Base(name))
	for _, entry := range byExtension {
		if strings.HasSuffix(lower, entry.suffix) {
			return entry.format, true
		}
	}
	return "", false
}

// magic là chữ ký ở đầu tệp của từng định dạng.
//
// Tên tệp là thứ người dùng đặt, nên nó nói dối được: một bản tải về đặt tên
// "source.download" vẫn là tệp zip. Chữ ký nằm trong chính dữ liệu nên đúng
// kể cả khi tên sai.
var magic = []struct {
	prefix []byte
	format Format
}{
	{[]byte{0x50, 0x4B, 0x03, 0x04}, FormatZip},
	{[]byte{0x50, 0x4B, 0x05, 0x06}, FormatZip},
	{[]byte("Rar!\x1A\x07"), FormatRar},
	{[]byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}, FormatSevenZip},
	{[]byte{0xFD, '7', 'z', 'X', 'Z', 0x00}, FormatXz},
	{[]byte{0x28, 0xB5, 0x2F, 0xFD}, FormatZst},
	{[]byte{0x1F, 0x8B}, FormatGz},
	{[]byte("BZh"), FormatBz2},
}

// MagicSize là số byte đầu tệp cần đọc để DetectMagic làm việc được.
const MagicSize = 512

// DetectMagic đoán định dạng từ vài byte đầu tệp.
//
// Với gz, bz2, xz và zst thì chữ ký chỉ nói tệp được nén bằng gì, không nói bên
// trong là một tệp đơn hay một tệp tar; phần đó do lớp giải nén phía trên tự
// nhận ra khi đọc tới nội dung.
func DetectMagic(head []byte) (Format, bool) {
	for _, entry := range magic {
		if len(head) >= len(entry.prefix) && string(head[:len(entry.prefix)]) == string(entry.prefix) {
			return entry.format, true
		}
	}
	// tar không có chữ ký ở đầu tệp: "ustar" nằm ở byte thứ 257.
	if len(head) >= 262 && string(head[257:262]) == "ustar" {
		return FormatTar, true
	}
	return "", false
}

// NeedsRandomAccess cho biết định dạng đòi đọc ngẫu nhiên toàn tệp.
//
// zip và 7z để bảng thư mục ở cuối tệp, nên phải nhảy tới cuối trước khi đọc
// được mục đầu tiên — không thể vừa tải vừa giải nén như tar.
func (f Format) NeedsRandomAccess() bool {
	return f == FormatZip || f == FormatSevenZip
}

// SingleFile cho biết định dạng chỉ chứa đúng một tệp, không có cây thư mục.
func (f Format) SingleFile() bool {
	switch f {
	case FormatGz, FormatBz2, FormatXz, FormatZst:
		return true
	default:
		return false
	}
}

// CanCreate cho biết panel nén ra được định dạng này.
//
// RAR là định dạng độc quyền, không có bộ nén tự do nào tạo ra được, và 7z thì
// panel chỉ đọc. Nói thẳng ra thay vì để người dùng chọn rồi nhận lỗi.
func (f Format) CanCreate() bool {
	switch f {
	case FormatZip, FormatTar, FormatTarGz, FormatTarXz, FormatTarZst:
		return true
	default:
		return false
	}
}

// Formats liệt kê mọi định dạng đọc được.
func Formats() []Format {
	return []Format{
		FormatZip, FormatTar, FormatTarGz, FormatTarBz2, FormatTarXz, FormatTarZst,
		FormatRar, FormatSevenZip, FormatGz, FormatBz2, FormatXz, FormatZst,
	}
}

// TrimExtension bỏ phần đuôi định dạng khỏi tên tệp.
//
// Dùng để đặt tên cho nội dung của tệp nén một-tệp: "backup.sql.gz" giải ra
// phải thành "backup.sql", chứ không phải một tệp trùng tên với tệp nén.
func TrimExtension(name string) string {
	lower := strings.ToLower(name)
	for _, entry := range byExtension {
		if strings.HasSuffix(lower, entry.suffix) {
			return name[:len(name)-len(entry.suffix)]
		}
	}
	return name
}

// Extensions liệt kê mọi phần đuôi tên tệp mà panel nhận là tệp nén.
//
// Giao diện dùng danh sách này để biết khi nào nên mời người dùng giải nén, nên
// nó phải sinh ra từ chính bảng nhận dạng thay vì được chép tay sang JavaScript.
func Extensions() []string {
	out := make([]string, 0, len(byExtension))
	for _, entry := range byExtension {
		out = append(out, entry.suffix)
	}
	return out
}
