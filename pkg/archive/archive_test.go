package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// memory là Sink giữ kết quả trong bộ nhớ để bài kiểm thử soi được.
type memory struct {
	files map[string]string
	dirs  []string
}

func newMemory() *memory { return &memory{files: map[string]string{}} }

func (m *memory) Dir(_ context.Context, name string, _ fs.FileMode) error {
	m.dirs = append(m.dirs, name)
	return nil
}

func (m *memory) File(_ context.Context, name string, _ fs.FileMode, r io.Reader) error {
	data, err := io.ReadAll(r)
	m.files[name] = string(data)
	return err
}

func extract(t *testing.T, data []byte, opts Options) (*memory, Result, error) {
	t.Helper()
	sink := newMemory()
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 1 << 20
	}
	result, err := Extract(context.Background(), bytes.NewReader(data), int64(len(data)), opts, sink)
	return sink, result, err
}

func zipOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("tạo mục %s: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("ghi mục %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("đóng tệp zip: %v", err)
	}
	return buf.Bytes()
}

func tarOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range entries {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("ghi tiêu đề %s: %v", name, err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("ghi mục %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("đóng tệp tar: %v", err)
	}
	return buf.Bytes()
}

func TestDetectName(t *testing.T) {
	cases := map[string]Format{
		"src.zip":            FormatZip,
		"backup.tar.gz":      FormatTarGz,
		"backup.tgz":         FormatTarGz,
		"logs.tar.bz2":       FormatTarBz2,
		"kho.tar.xz":         FormatTarXz,
		"kho.tar.zst":        FormatTarZst,
		"app.rar":            FormatRar,
		"app.7z":             FormatSevenZip,
		"dump.sql.gz":        FormatGz,
		"plugin.jar":         FormatZip,
		"THU-MUC/SRC.ZIP":    FormatZip,
		"/opt/data/site.tar": FormatTar,
	}

	for name, want := range cases {
		got, ok := DetectName(name)
		if !ok || got != want {
			t.Errorf("%s: nhận %q (%v), mong %q", name, got, ok, want)
		}
	}

	if _, ok := DetectName("ghi-chu.txt"); ok {
		t.Error("tệp văn bản không được nhận là tệp nén")
	}
}

// Tên tệp là thứ người dùng đặt nên nó nói dối được; chữ ký trong dữ liệu thì không.
func TestDetectMagicBeatsWrongName(t *testing.T) {
	data := zipOf(t, map[string]string{"a.txt": "xin chao"})

	format, ok := DetectMagic(data[:min(len(data), MagicSize)])
	if !ok || format != FormatZip {
		t.Fatalf("nhận %q (%v), mong zip", format, ok)
	}
}

func TestExtractZip(t *testing.T) {
	data := zipOf(t, map[string]string{
		"index.php":       "<?php echo 1;",
		"assets/app.css":  "body{}",
		"thu-muc/con.txt": "noi dung",
	})

	sink, result, err := extract(t, data, Options{Format: FormatZip})
	if err != nil {
		t.Fatalf("giải nén: %v", err)
	}
	if result.Files != 3 {
		t.Errorf("giải ra %d tệp, mong 3", result.Files)
	}
	if sink.files["assets/app.css"] != "body{}" {
		t.Errorf("nội dung sai: %q", sink.files["assets/app.css"])
	}
}

func TestExtractTarVariants(t *testing.T) {
	raw := tarOf(t, map[string]string{"src/main.go": "package main"})

	var gzipped bytes.Buffer
	gw := gzip.NewWriter(&gzipped)
	_, _ = gw.Write(raw)
	_ = gw.Close()

	var xzipped bytes.Buffer
	xw, err := xz.NewWriter(&xzipped)
	if err != nil {
		t.Fatalf("mở bộ nén xz: %v", err)
	}
	_, _ = xw.Write(raw)
	_ = xw.Close()

	var zstd0 bytes.Buffer
	zw, err := zstd.NewWriter(&zstd0)
	if err != nil {
		t.Fatalf("mở bộ nén zstd: %v", err)
	}
	_, _ = zw.Write(raw)
	_ = zw.Close()

	cases := map[Format][]byte{
		FormatTar:    raw,
		FormatTarGz:  gzipped.Bytes(),
		FormatTarXz:  xzipped.Bytes(),
		FormatTarZst: zstd0.Bytes(),
	}

	for format, data := range cases {
		sink, _, err := extract(t, data, Options{Format: format, Name: "kho." + string(format)})
		if err != nil {
			t.Errorf("%s: giải nén: %v", format, err)
			continue
		}
		if sink.files["src/main.go"] != "package main" {
			t.Errorf("%s: nội dung sai: %q", format, sink.files["src/main.go"])
		}
	}
}

// Một tệp .gz có thể là tar nén hoặc chỉ là một tệp đơn; panel phải nhìn vào dữ
// liệu chứ không tin phần đuôi tên tệp.
func TestExtractSingleFileNamesByArchive(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = io.WriteString(gw, "SELECT 1;")
	_ = gw.Close()

	sink, result, err := extract(t, buf.Bytes(), Options{Format: FormatGz, Name: "/kho/dump.sql.gz"})
	if err != nil {
		t.Fatalf("giải nén: %v", err)
	}
	if result.Files != 1 {
		t.Fatalf("giải ra %d tệp, mong 1", result.Files)
	}
	if sink.files["dump.sql"] != "SELECT 1;" {
		t.Errorf("tệp giải ra: %v", sink.files)
	}
}

// Tar nén gzip nhưng đặt tên ".gz" vẫn phải ra cả cây thư mục.
func TestExtractTarInsideGzNamedGz(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(tarOf(t, map[string]string{"a/b.txt": "noi dung"}))
	_ = gw.Close()

	sink, _, err := extract(t, buf.Bytes(), Options{Format: FormatGz, Name: "kho.gz"})
	if err != nil {
		t.Fatalf("giải nén: %v", err)
	}
	if sink.files["a/b.txt"] != "noi dung" {
		t.Errorf("tệp giải ra: %v", sink.files)
	}
}

// Lỗ hổng kinh điển của trình giải nén: một mục tên "../../etc/cron.d/x" ghi đè
// tệp hệ thống. Phải từ chối cả tệp nén chứ không chỉ bỏ qua mục đó.
func TestExtractRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../thoat.txt", "/etc/cron.d/x", "a/../../b.txt"} {
		_, _, err := extract(t, zipOf(t, map[string]string{name: "x"}), Options{Format: FormatZip})
		if !errors.Is(err, ErrUnsafePath) {
			t.Errorf("mục %q: mong ErrUnsafePath, nhận %v", name, err)
		}
	}
}

func TestExtractStopsAtSizeLimit(t *testing.T) {
	data := zipOf(t, map[string]string{"lon.bin": strings.Repeat("a", 4096)})

	_, _, err := extract(t, data, Options{Format: FormatZip, MaxBytes: 1024})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("mong ErrTooLarge, nhận %v", err)
	}
}

func TestExtractDetectsEncryptedZip(t *testing.T) {
	if _, err := exec.LookPath("zip"); err != nil {
		t.Skip("máy không có lệnh zip để tạo tệp có mật khẩu")
	}

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/bimat.txt", []byte("noi dung"), 0o600); err != nil {
		t.Fatalf("ghi tệp: %v", err)
	}
	cmd := exec.Command("zip", "-q", "-P", "matkhau", dir+"/kho.zip", "bimat.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Skipf("không tạo được tệp zip có mật khẩu: %v", err)
	}

	data, err := os.ReadFile(dir + "/kho.zip")
	if err != nil {
		t.Fatalf("đọc tệp zip: %v", err)
	}

	if _, _, err := extract(t, data, Options{Format: FormatZip}); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("mong ErrEncrypted, nhận %v", err)
	}
}

func TestExtractCorruptArchive(t *testing.T) {
	_, _, err := extract(t, []byte("day khong phai tep nen"), Options{Format: FormatZip})
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("mong ErrCorrupt, nhận %v", err)
	}
}

// Định dạng đọc ngẫu nhiên phải báo lỗi rõ ràng khi nguồn chỉ đọc tuần tự được,
// để lớp gọi biết đường tải tệp về đĩa trước.
func TestExtractNeedsRandomAccess(t *testing.T) {
	sink := newMemory()
	stream := struct{ io.Reader }{strings.NewReader("noi dung")}

	_, err := Extract(context.Background(), stream, 8, Options{Format: FormatZip}, sink)
	if !errors.Is(err, ErrNeedRandomAccess) {
		t.Fatalf("mong ErrNeedRandomAccess, nhận %v", err)
	}
}

func TestExtractFixtures(t *testing.T) {
	cases := map[string]struct {
		format Format
		want   []string
	}{
		"testdata/sample.7z":  {FormatSevenZip, []string{"sunpanel.txt", "thu-muc/ghi-chu.txt"}},
		"testdata/sample.rar": {FormatRar, []string{"stest1.txt", "stest2.txt"}},
	}

	for path, want := range cases {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("đọc %s: %v", path, err)
		}

		sink, _, err := extract(t, data, Options{Format: want.format, Name: path})
		if err != nil {
			t.Errorf("%s: giải nén: %v", path, err)
			continue
		}
		for _, name := range want.want {
			if _, ok := sink.files[name]; !ok {
				t.Errorf("%s: thiếu mục %q, có %v", path, name, sink.files)
			}
		}
	}
}

func TestCanCreate(t *testing.T) {
	// RAR là định dạng độc quyền: panel đọc được nhưng không tạo ra được.
	if FormatRar.CanCreate() || FormatSevenZip.CanCreate() {
		t.Error("rar và 7z không được coi là định dạng nén ra được")
	}
	for _, format := range []Format{FormatZip, FormatTarGz, FormatTarZst} {
		if !format.CanCreate() {
			t.Errorf("%s phải nén ra được", format)
		}
	}
}
