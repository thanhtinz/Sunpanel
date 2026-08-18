package sysuser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

const passwdFixture = `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin
lan:x:1000:1000:Lan Nguyen,,,:/home/lan:/bin/bash
dong:x:1001:1001::/home/dong:/bin/sh
dòng-hỏng-không-đủ-trường
`

const shadowFixture = `root:$6$abc$xyz:19000:0:99999:7:::
lan:!$6$def$uvw:19100:0:99999:7:::
dong:*:19100:0:99999:7:::
`

const groupFixture = `root:x:0:
sudo:x:27:lan
www-data:x:33:
docker:x:998:lan,dong
`

func TestParsePasswd(t *testing.T) {
	users := ParsePasswd(passwdFixture)

	if len(users) != 5 {
		t.Fatalf("đọc được %d tài khoản, mong 5: %+v", len(users), users)
	}

	byName := map[string]User{}
	for _, user := range users {
		byName[user.Name] = user
	}

	lan := byName["lan"]
	if lan.UID != 1000 || lan.Home != "/home/lan" || lan.Shell != "/bin/bash" {
		t.Errorf("tài khoản lan đọc sai: %+v", lan)
	}
	// Dấu phẩy cuối trường ghi chú là quy ước của chfn, không phải nội dung.
	if lan.Comment != "Lan Nguyen" {
		t.Errorf("ghi chú = %q, mong \"Lan Nguyen\"", lan.Comment)
	}
	if lan.System {
		t.Error("tài khoản UID 1000 không phải tài khoản hệ thống")
	}
	if !byName["www-data"].System {
		t.Error("www-data phải là tài khoản hệ thống")
	}
	// root có UID 0 nhưng vẫn là tài khoản người ta đăng nhập thật.
	if byName["root"].System {
		t.Error("root không được xếp vào nhóm tài khoản hệ thống")
	}
}

func TestParseShadow(t *testing.T) {
	states := ParseShadow(shadowFixture)

	if states["lan"].Locked != true {
		t.Error("lan có dấu ! ở đầu chuỗi băm nên phải bị coi là đã khóa")
	}
	if states["root"].Locked {
		t.Error("root không bị khóa")
	}
	if !states["dong"].NoPassword {
		t.Error("dong có dấu * nên là chưa đặt mật khẩu")
	}
}

func TestParseGroups(t *testing.T) {
	groups := ParseGroups(groupFixture)

	if len(groups["lan"]) != 2 {
		t.Errorf("lan thuộc %v, mong hai nhóm", groups["lan"])
	}
	if len(groups["dong"]) != 1 || groups["dong"][0] != "docker" {
		t.Errorf("dong thuộc %v", groups["dong"])
	}
}

func TestValidName(t *testing.T) {
	for _, name := range []string{"lan", "web_1", "a-b", "user123"} {
		if !ValidName(name) {
			t.Errorf("%q phải hợp lệ", name)
		}
	}
	for _, name := range []string{"", "1user", "-user", "Lan", "có dấu", "ten;rm -rf /", strings.Repeat("a", 33)} {
		if ValidName(name) {
			t.Errorf("%q không được coi là hợp lệ", name)
		}
	}
}

// Khóa mẫu sinh bằng ssh-keygen, dùng cố định để bài kiểm thử không phụ thuộc
// vào việc máy chạy kiểm thử có ssh-keygen hay không.
const ed25519Key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJ7dEHmv5r4LU5tGrGXd9Uh0d3xJfhLNMr8h4c5wYQ0k lan@may-tinh"

func TestNormalizeKey(t *testing.T) {
	key, err := NormalizeKey("  " + ed25519Key + "  \n")
	if err != nil {
		t.Fatalf("khóa hợp lệ bị từ chối: %v", err)
	}
	if key.Type != "ssh-ed25519" {
		t.Errorf("loại khóa = %q", key.Type)
	}
	if key.Comment != "lan@may-tinh" {
		t.Errorf("chú thích = %q", key.Comment)
	}
	if !strings.HasPrefix(key.Fingerprint, "SHA256:") {
		t.Errorf("dấu vân tay = %q", key.Fingerprint)
	}
	// Khoảng trắng thừa không được lọt vào tệp: một dòng hỏng khiến sshd bỏ qua
	// toàn bộ phần còn lại của tệp.
	if strings.TrimSpace(key.Line) != key.Line {
		t.Errorf("dòng chuẩn hóa còn khoảng trắng thừa: %q", key.Line)
	}
}

// Dán nhầm khóa riêng vào ô khóa công khai là lỗi rất dễ gặp, và hậu quả là lộ
// khóa riêng vào một tệp mà cả nhóm đọc được.
func TestNormalizeKeyRejectsPrivateKey(t *testing.T) {
	private := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk=\n-----END OPENSSH PRIVATE KEY-----"

	for _, raw := range []string{"", "khong-phai-khoa", private, "ssh-ed25519 rác-không-phải-base64"} {
		if _, err := NormalizeKey(raw); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%.20q: lỗi = %v, mong ErrInvalidKey", raw, err)
		}
	}
}

func TestAddAndRemoveKey(t *testing.T) {
	key, err := NormalizeKey(ed25519Key)
	if err != nil {
		t.Fatalf("chuẩn hóa khóa: %v", err)
	}

	content, err := AddKey("# khóa của nhóm vận hành\n", key)
	if err != nil {
		t.Fatalf("thêm khóa: %v", err)
	}
	if !strings.Contains(content, "# khóa của nhóm vận hành") {
		t.Error("dòng chú thích có sẵn trong tệp bị mất")
	}
	if len(ParseKeys(content)) != 1 {
		t.Fatalf("tệp sau khi thêm: %q", content)
	}

	// Thêm lại đúng khóa đó phải bị từ chối, kể cả khi chú thích khác đi.
	if _, err := AddKey(content, key); !errors.Is(err, ErrKeyExists) {
		t.Errorf("thêm trùng: lỗi = %v, mong ErrKeyExists", err)
	}

	removed, err := RemoveKey(content, key.Fingerprint)
	if err != nil {
		t.Fatalf("xóa khóa: %v", err)
	}
	if len(ParseKeys(removed)) != 0 {
		t.Errorf("tệp sau khi xóa vẫn còn khóa: %q", removed)
	}
	if !strings.Contains(removed, "# khóa của nhóm vận hành") {
		t.Error("xóa khóa không được xóa luôn dòng chú thích của người dùng")
	}

	if _, err := RemoveKey(removed, key.Fingerprint); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("xóa khóa không có: lỗi = %v, mong ErrKeyNotFound", err)
	}
}

// Dòng panel không hiểu vẫn phải nằm nguyên trong tệp: nó có thể đang có tác
// dụng, và xóa hộ người dùng một thứ mình không hiểu là việc không nên làm.
func TestRemoveKeyKeepsUnknownLines(t *testing.T) {
	key, _ := NormalizeKey(ed25519Key)
	content, _ := AddKey("command=\"/usr/bin/backup\" dong-la\n", key)

	removed, err := RemoveKey(content, key.Fingerprint)
	if err != nil {
		t.Fatalf("xóa khóa: %v", err)
	}
	if !strings.Contains(removed, "command=") {
		t.Errorf("dòng không phân tích được đã bị mất: %q", removed)
	}
}

// Danh sách khóa rỗng phải là mảng rỗng chứ không phải nil: nil đi qua JSON
// thành null, và giao diện đếm độ dài của null thì hỏng nguyên trang.
func TestKeysReturnsEmptySliceNotNil(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatalf("tạo thư mục: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "passwd"), []byte(passwdFixture), 0o644); err != nil {
		t.Fatalf("ghi passwd: %v", err)
	}

	manager := New(host.NewLocalHost(root, nil))
	keys, err := manager.Keys(context.Background(), "lan")
	if err != nil {
		t.Fatalf("đọc khóa: %v", err)
	}
	if keys == nil {
		t.Fatal("danh sách khóa là nil")
	}
	if len(keys) != 0 {
		t.Errorf("tài khoản chưa có khóa nhưng đọc ra %d khóa", len(keys))
	}
}

// Đọc được /etc/passwd nghĩa là quản lý tài khoản được; không đọc được thì
// trang phải báo rõ thay vì hiện danh sách trống như thể máy không có ai.
func TestListFailsWithoutPasswd(t *testing.T) {
	manager := New(host.NewLocalHost(t.TempDir(), nil))

	if manager.Available(context.Background()) {
		t.Error("không có /etc/passwd mà vẫn báo là dùng được")
	}
	if _, err := manager.List(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("lỗi = %v, mong ErrUnavailable", err)
	}
}
