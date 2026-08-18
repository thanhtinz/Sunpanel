package sysservice

import (
	"errors"
	"testing"
)

// Đầu ra thật của: systemctl list-units --type=service --all --output=json
const unitsFixture = `[
  {"unit":"cron.service","load":"loaded","active":"active","sub":"running","description":"Regular background program processing daemon"},
  {"unit":"nginx.service","load":"loaded","active":"active","sub":"running","description":"A high performance web server and a reverse proxy server"},
  {"unit":"mysql.service","load":"loaded","active":"failed","sub":"failed","description":"MySQL Community Server"},
  {"unit":"apt-daily.service","load":"loaded","active":"inactive","sub":"dead","description":"Daily apt download activities"},
  {"unit":"not-found.service","load":"not-found","active":"inactive","sub":"dead","description":"not-found.service"},
  {"unit":"cron.timer","load":"loaded","active":"active","sub":"waiting","description":"Cron timer"}
]`

// Đầu ra thật của: systemctl list-unit-files --type=service --output=json
const unitFilesFixture = `[
  {"unit_file":"cron.service","state":"enabled","preset":"enabled"},
  {"unit_file":"nginx.service","state":"enabled","preset":"enabled"},
  {"unit_file":"mysql.service","state":"disabled","preset":"enabled"},
  {"unit_file":"apt-daily.service","state":"static","preset":""},
  {"unit_file":"rescue.service","state":"masked","preset":""}
]`

func TestParseUnits(t *testing.T) {
	services, err := parseUnits(unitsFixture)
	if err != nil {
		t.Fatalf("đọc danh sách: %v", err)
	}

	// Timer bị loại, chỉ còn 5 dịch vụ.
	if len(services) != 5 {
		t.Fatalf("số dịch vụ = %d, mong đợi 5 (timer phải bị loại)", len(services))
	}

	byName := map[string]Service{}
	for _, svc := range services {
		byName[svc.Name] = svc
	}

	if svc := byName["nginx.service"]; svc.State != StateActive || !svc.Running() {
		t.Errorf("nginx: trạng thái = %q, mong đợi active", svc.State)
	}
	if svc := byName["mysql.service"]; svc.State != StateFailed || svc.Running() {
		t.Errorf("mysql: trạng thái = %q, mong đợi failed", svc.State)
	}
	if svc := byName["apt-daily.service"]; svc.State != StateInactive {
		t.Errorf("apt-daily: trạng thái = %q, mong đợi inactive", svc.State)
	}
	if svc := byName["not-found.service"]; svc.Loaded {
		t.Error("not-found.service phải có Loaded = false")
	}
	if svc := byName["cron.service"]; svc.Description == "" {
		t.Error("thiếu mô tả dịch vụ")
	}
	if _, ok := byName["cron.timer"]; ok {
		t.Error("timer không được lọt vào danh sách dịch vụ")
	}
}

func TestParseUnitFiles(t *testing.T) {
	states, err := parseUnitFiles(unitFilesFixture)
	if err != nil {
		t.Fatalf("đọc trạng thái: %v", err)
	}

	cases := map[string]string{
		"cron.service":      "enabled",
		"mysql.service":     "disabled",
		"apt-daily.service": "static",
		"rescue.service":    "masked",
	}
	for unit, want := range cases {
		if got := states[unit]; got != want {
			t.Errorf("%s: trạng thái = %q, mong đợi %q", unit, got, want)
		}
	}
}

func TestParseEmptyOutput(t *testing.T) {
	services, err := parseUnits("")
	if err != nil || services != nil {
		t.Errorf("đầu ra rỗng: services = %v, err = %v", services, err)
	}

	states, err := parseUnitFiles("   ")
	if err != nil || len(states) != 0 {
		t.Errorf("đầu ra rỗng: states = %v, err = %v", states, err)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	if _, err := parseUnits("không phải JSON"); err == nil {
		t.Error("JSON hỏng phải trả về lỗi")
	}
}

// Tên dịch vụ đi thẳng vào tham số lệnh nên không có nguy cơ shell injection,
// nhưng một tên bắt đầu bằng dấu gạch sẽ bị systemctl hiểu là cờ dòng lệnh.
func TestNormalizeUnitRejectsFlagLikeNames(t *testing.T) {
	invalid := []string{
		"--now",
		"-h",
		"--force",
		"",
		"   ",
		"nginx; rm -rf /",
		"nginx service",
		"nginx$(whoami)",
		"nginx`id`",
		"nginx\x00.service",
		"nginx/../../etc/passwd",
	}

	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeUnit(name); !errors.Is(err, ErrInvalidName) {
				t.Fatalf("normalizeUnit(%q) lỗi = %v, mong đợi ErrInvalidName", name, err)
			}
		})
	}
}

func TestNormalizeUnitAddsSuffix(t *testing.T) {
	cases := map[string]string{
		"nginx":           "nginx.service",
		"nginx.service":   "nginx.service",
		"nginx.socket":    "nginx.socket",
		"  mysql  ":       "mysql.service",
		"getty@tty1":      "getty@tty1.service",
		"user-1000.slice": "user-1000.slice",
		"my_service":      "my_service.service",
	}
	for input, want := range cases {
		got, err := normalizeUnit(input)
		if err != nil {
			t.Errorf("normalizeUnit(%q) lỗi: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("normalizeUnit(%q) = %q, mong đợi %q", input, got, want)
		}
	}
}

func TestParseState(t *testing.T) {
	cases := map[string]State{
		"active":       StateActive,
		"inactive":     StateInactive,
		"failed":       StateFailed,
		"activating":   StateActivating,
		"deactivating": StateDeactivating,
		"reloading":    StateUnknown,
		"":             StateUnknown,
	}
	for input, want := range cases {
		if got := parseState(input); got != want {
			t.Errorf("parseState(%q) = %q, mong đợi %q", input, got, want)
		}
	}
}
