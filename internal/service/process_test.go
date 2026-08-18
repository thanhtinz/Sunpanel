package service

import (
	"testing"

	"github.com/thanhtinz/sunpanel/pkg/procs"
)

// Từ khóa phải khớp cả dòng lệnh chứ không chỉ tên: một tiến trình Java hiện tên
// "java", nên tìm "nginx" mà chỉ so tên sẽ không ra tiến trình nào đúng.
func TestMatchesProcess(t *testing.T) {
	item := procs.Process{
		PID:      4321,
		Name:     "java",
		Username: "tomcat",
		Command:  "java -jar /opt/app/gateway.jar",
	}

	for _, keyword := range []string{"java", "tomcat", "gateway", "4321"} {
		if !matchesProcess(item, keyword) {
			t.Errorf("từ khóa %q phải khớp tiến trình", keyword)
		}
	}

	// Số hiệu tiến trình so khớp trọn vẹn: gõ "43" mà ra mọi PID chứa 43 thì ô
	// tìm kiếm vô dụng đúng lúc người dùng đã biết chính xác PID cần tìm.
	for _, keyword := range []string{"43", "postgres"} {
		if matchesProcess(item, keyword) {
			t.Errorf("từ khóa %q không được khớp tiến trình", keyword)
		}
	}
}
