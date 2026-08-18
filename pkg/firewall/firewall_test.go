package firewall

import (
	"errors"
	"testing"
)

// Đầu ra thật của: ufw status numbered
const ufwFixture = `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere
[ 2] 80,443/tcp                 ALLOW IN    Anywhere
[ 3] 3306/tcp                   DENY IN     Anywhere
[ 4] 9527/tcp                   ALLOW IN    192.168.1.0/24
[ 5] 8000:8100/udp              ALLOW IN    Anywhere
[ 6] 22/tcp (v6)                ALLOW IN    Anywhere (v6)
[ 7] 5432                       REJECT IN   10.0.0.5
`

func TestParseUFWRules(t *testing.T) {
	rules := parseUFWRules(ufwFixture)

	if len(rules) != 7 {
		t.Fatalf("số quy tắc = %d, mong đợi 7", len(rules))
	}

	cases := []struct {
		index    int
		id       string
		action   Action
		protocol Protocol
		port     string
		source   string
		ipv6     bool
	}{
		{0, "1", ActionAllow, ProtocolTCP, "22", "", false},
		{2, "3", ActionDeny, ProtocolTCP, "3306", "", false},
		{3, "4", ActionAllow, ProtocolTCP, "9527", "192.168.1.0/24", false},
		{4, "5", ActionAllow, ProtocolUDP, "8000:8100", "", false},
		// ufw tự sinh dòng IPv6 song song; bỏ qua nó là giấu mất quy tắc thật.
		{5, "6", ActionAllow, ProtocolTCP, "22", "", true},
		{6, "7", ActionReject, ProtocolAny, "5432", "10.0.0.5", false},
	}

	for _, tc := range cases {
		rule := rules[tc.index]
		if rule.ID != tc.id || rule.Action != tc.action || rule.Protocol != tc.protocol ||
			rule.Port != tc.port || rule.Source != tc.source || rule.IPv6 != tc.ipv6 {
			t.Errorf("quy tắc %d = %+v, mong đợi id=%s action=%s proto=%s port=%s source=%s ipv6=%v",
				tc.index, rule, tc.id, tc.action, tc.protocol, tc.port, tc.source, tc.ipv6)
		}
	}
}

func TestParseUFWIgnoresHeaders(t *testing.T) {
	// Đầu ra khi tường lửa tắt không có dòng quy tắc nào.
	rules := parseUFWRules("Status: inactive\n")
	if len(rules) != 0 {
		t.Errorf("số quy tắc = %d, mong đợi 0", len(rules))
	}
}

func TestBuildUFWArgs(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
		want []string
	}{
		{
			"cho phép cổng tcp",
			Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80"},
			[]string{"allow", "port", "80", "proto", "tcp"},
		},
		{
			"chặn theo nguồn",
			Rule{Action: ActionDeny, Protocol: ProtocolTCP, Port: "3306", Source: "10.0.0.0/8"},
			[]string{"deny", "from", "10.0.0.0/8", "to", "any", "port", "3306", "proto", "tcp"},
		},
		{
			"mọi giao thức",
			Rule{Action: ActionAllow, Protocol: ProtocolAny, Port: "53"},
			[]string{"allow", "port", "53"},
		},
		{
			"dải cổng",
			Rule{Action: ActionAllow, Protocol: ProtocolUDP, Port: "8000:8100"},
			[]string{"allow", "port", "8000:8100", "proto", "udp"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildUFWArgs(tc.rule)
			if len(got) != len(tc.want) {
				t.Fatalf("tham số = %v, mong đợi %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("tham số = %v, mong đợi %v", got, tc.want)
				}
			}
		})
	}
}

// Giá trị của quy tắc đi thẳng vào tham số dòng lệnh của công cụ tường lửa, nên
// mọi thứ ngoài tập an toàn phải bị chặn ngay tại đây.
func TestValidateRuleRejectsUnsafeValues(t *testing.T) {
	invalid := []struct {
		name string
		rule Rule
	}{
		{"hành động lạ", Rule{Action: "xóa-hết", Protocol: ProtocolTCP, Port: "80"}},
		{"giao thức lạ", Rule{Action: ActionAllow, Protocol: "icmp; rm -rf /", Port: "80"}},
		{"cổng không phải số", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80; reboot"}},
		{"cổng bằng 0", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "0"}},
		{"cổng quá lớn", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "70000"}},
		{"cổng âm", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "-1"}},
		{"dải cổng ngược", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "9000:80"}},
		{"dải cổng ba phần", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80:90:100"}},
		{"nguồn không phải IP", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80", Source: "--force"}},
		{"nguồn có dấu chấm phẩy", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80", Source: "1.2.3.4; id"}},
		{"nguồn là tên miền", Rule{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80", Source: "example.com"}},
	}

	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRule(tc.rule); !errors.Is(err, ErrInvalidRule) {
				t.Fatalf("lỗi = %v, mong đợi ErrInvalidRule", err)
			}
		})
	}
}

func TestValidateRuleAcceptsValidValues(t *testing.T) {
	valid := []Rule{
		{Action: ActionAllow, Protocol: ProtocolTCP, Port: "80"},
		{Action: ActionDeny, Protocol: ProtocolUDP, Port: "53", Source: "10.0.0.0/8"},
		{Action: ActionReject, Protocol: ProtocolAny, Port: "8000:8100"},
		{Action: ActionAllow, Protocol: ProtocolTCP, Port: "", Source: "192.168.1.10"},
		{Action: ActionAllow, Protocol: ProtocolTCP, Port: "443", Source: "any"},
		{Action: ActionAllow, Protocol: ProtocolTCP, Port: "22", Source: "2001:db8::/32"},
	}

	for _, rule := range valid {
		if err := ValidateRule(rule); err != nil {
			t.Errorf("quy tắc %+v phải hợp lệ, nhận lỗi: %v", rule, err)
		}
	}
}

// Đầu ra thật của: firewall-cmd --list-ports
const firewalldPortsFixture = "22/tcp 80/tcp 443/tcp 8000-8100/udp"

// Đầu ra thật của: firewall-cmd --list-rich-rules
const firewalldRichFixture = `rule family="ipv4" source address="192.168.1.0/24" port port="9527" protocol="tcp" accept
rule family="ipv4" source address="10.0.0.5" port port="3306" protocol="tcp" drop
rule family="ipv4" source address="172.16.0.0/12" reject`

func TestParseFirewalldPorts(t *testing.T) {
	rules := parseFirewalldPorts(firewalldPortsFixture)

	if len(rules) != 4 {
		t.Fatalf("số quy tắc = %d, mong đợi 4", len(rules))
	}
	if rules[0].Port != "22" || rules[0].Protocol != ProtocolTCP || rules[0].Action != ActionAllow {
		t.Errorf("quy tắc đầu = %+v", rules[0])
	}
	// Dải cổng của firewalld dùng dấu gạch nối, phải chuyển về dạng chuẩn hóa.
	if rules[3].Port != "8000:8100" {
		t.Errorf("dải cổng = %q, mong đợi \"8000:8100\"", rules[3].Port)
	}
	if rules[3].ID != "port:8000-8100/udp" {
		t.Errorf("id = %q", rules[3].ID)
	}
}

func TestParseFirewalldRichRules(t *testing.T) {
	rules := parseFirewalldRichRules(firewalldRichFixture)

	if len(rules) != 3 {
		t.Fatalf("số quy tắc = %d, mong đợi 3", len(rules))
	}

	if rules[0].Action != ActionAllow || rules[0].Source != "192.168.1.0/24" ||
		rules[0].Port != "9527" || rules[0].Protocol != ProtocolTCP {
		t.Errorf("quy tắc 0 = %+v", rules[0])
	}
	if rules[1].Action != ActionDeny {
		t.Errorf("quy tắc 1 hành động = %q, mong đợi deny", rules[1].Action)
	}
	if rules[2].Action != ActionReject || rules[2].Port != "" {
		t.Errorf("quy tắc 2 = %+v", rules[2])
	}
}

func TestBuildRichRule(t *testing.T) {
	got := buildRichRule(Rule{
		Action: ActionDeny, Protocol: ProtocolTCP, Port: "3306", Source: "10.0.0.5",
	})
	want := `rule family="ipv4" source address="10.0.0.5" port port="3306" protocol="tcp" drop`
	if got != want {
		t.Errorf("rich rule = %q, mong đợi %q", got, want)
	}
}

func TestFirewalldPortDefaultsToTCP(t *testing.T) {
	// firewalld bắt buộc khai báo giao thức cho mỗi cổng.
	if got := firewalldPort(Rule{Protocol: ProtocolAny, Port: "80"}); got != "80/tcp" {
		t.Errorf("cổng = %q, mong đợi \"80/tcp\"", got)
	}
	if got := firewalldPort(Rule{Protocol: ProtocolUDP, Port: "8000:8100"}); got != "8000-8100/udp" {
		t.Errorf("cổng = %q, mong đợi \"8000-8100/udp\"", got)
	}
}

func TestUnavailableManagerReturnsClearError(t *testing.T) {
	manager := NewUnavailable()

	if _, err := manager.ListRules(t.Context()); !errors.Is(err, ErrNoBackend) {
		t.Errorf("ListRules lỗi = %v, mong đợi ErrNoBackend", err)
	}
	if err := manager.AddRule(t.Context(), Rule{}); !errors.Is(err, ErrNoBackend) {
		t.Errorf("AddRule lỗi = %v, mong đợi ErrNoBackend", err)
	}

	status, err := manager.Status(t.Context())
	if err != nil || status.Available || status.Enabled {
		t.Errorf("trạng thái = %+v, lỗi = %v", status, err)
	}
}
