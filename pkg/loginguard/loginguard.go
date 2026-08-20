// Package loginguard chặn tạm thời những địa chỉ IP đang dò mật khẩu panel.
//
// Panel đã khóa tài khoản sau vài lần sai, nhưng lớp đó chỉ bảo vệ đúng tài
// khoản bị nhắm tới: một máy quét thử "admin", "root", "administrator"... mỗi
// tên một lần thì không tài khoản nào chạm ngưỡng, còn máy chủ vẫn nhận đủ
// lượng yêu cầu đó. Gói này đếm theo nguồn gửi thay vì theo tài khoản, nên
// việc dò rải tên đăng nhập cũng bị chặn.
package loginguard

import (
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// Options là các tham số của bộ chặn.
type Options struct {
	// Threshold là số lần đăng nhập sai trước khi chặn.
	Threshold int
	// Window là khoảng thời gian mà các lần sai được cộng dồn.
	Window time.Duration
	// Duration là thời gian chặn mỗi lần vượt ngưỡng.
	Duration time.Duration
	// Trusted là các IP hoặc dải CIDR không bao giờ bị chặn.
	Trusted []string
}

// Block là một địa chỉ đang bị chặn.
type Block struct {
	IP        string    `json:"ip"`
	Failures  int       `json:"failures"`
	BlockedAt time.Time `json:"blockedAt"`
	Until     time.Time `json:"until"`
	// LastUser là tên đăng nhập bị thử gần nhất, giúp nhận ra mình vừa tự chặn
	// chính mình hay đang có người khác dò.
	LastUser string `json:"lastUser,omitempty"`
}

// entry là trạng thái theo dõi của một địa chỉ.
type entry struct {
	failures  int
	firstFail time.Time
	lastFail  time.Time
	blockedAt time.Time
	until     time.Time
	lastUser  string
}

// Guard đếm số lần đăng nhập sai theo địa chỉ nguồn.
//
// Trạng thái nằm trong bộ nhớ và mất khi panel khởi động lại: một danh sách
// chặn tạm thời không đáng để ghi đĩa mỗi lần có người gõ sai mật khẩu, và
// khởi động lại panel vốn là thao tác của quản trị viên chứ không phải của kẻ
// đang dò.
type Guard struct {
	mu      sync.Mutex
	entries map[string]*entry

	threshold int
	window    time.Duration
	duration  time.Duration

	trustedIPs  []net.IP
	trustedNets []*net.IPNet
}

// New tạo bộ chặn. Threshold bằng 0 nghĩa là tắt hẳn tính năng.
func New(opts Options) *Guard {
	g := &Guard{
		entries:   make(map[string]*entry),
		threshold: opts.Threshold,
		window:    opts.Window,
		duration:  opts.Duration,
	}
	for _, raw := range opts.Trusted {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(value); err == nil {
			g.trustedNets = append(g.trustedNets, network)
			continue
		}
		if ip := net.ParseIP(value); ip != nil {
			g.trustedIPs = append(g.trustedIPs, ip)
		}
	}
	return g
}

// Enabled cho biết bộ chặn có đang hoạt động không.
func (g *Guard) Enabled() bool { return g != nil && g.threshold > 0 }

// Trusted cho biết một địa chỉ có nằm trong danh sách miễn trừ không.
//
// Địa chỉ nội bộ luôn được miễn trừ. Yêu cầu đến từ đó chỉ có hai nguồn: chính
// người đang ngồi trên máy chủ — họ đã có shell, chặn cũng vô nghĩa — hoặc một
// reverse proxy chạy cùng máy, và khi đó chặn nó đồng nghĩa với chặn mọi người
// dùng cùng một lúc. Đây cũng là đường quay lại cuối cùng cho quản trị viên vừa
// tự khóa mình ra ngoài.
func (g *Guard) Trusted(ip string) bool {
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	if addr.IsLoopback() {
		return true
	}
	for _, trusted := range g.trustedIPs {
		if trusted.Equal(addr) {
			return true
		}
	}
	for _, network := range g.trustedNets {
		if network.Contains(addr) {
			return true
		}
	}
	return false
}

// Fail ghi nhận một lần đăng nhập sai và cho biết địa chỉ có bị chặn hay không.
func (g *Guard) Fail(ip, username string) (Block, bool) {
	if !g.Enabled() || ip == "" || g.Trusted(ip) {
		return Block{}, false
	}

	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	e, ok := g.entries[ip]
	// Cửa sổ đếm trôi: lần sai cách lần đầu quá lâu thì bắt đầu đếm lại, để một
	// người gõ nhầm mỗi tháng một lần không bao giờ tích đủ ngưỡng.
	if !ok || now.Sub(e.firstFail) > g.window {
		e = &entry{firstFail: now}
		g.entries[ip] = e
	}
	e.failures++
	e.lastFail = now
	if username != "" {
		e.lastUser = username
	}

	if e.failures < g.threshold {
		return Block{}, false
	}

	// Mỗi lần sai tiếp theo lại đẩy mốc hết chặn ra xa: kẻ đang dò không thoát
	// ra được bằng cách chờ đúng lúc hết hạn rồi thử tiếp.
	e.until = now.Add(g.duration)
	if e.blockedAt.IsZero() {
		e.blockedAt = now
	}
	return toBlock(ip, e), true
}

// Succeed xóa bộ đếm sau một lần đăng nhập thành công.
func (g *Guard) Succeed(ip string) {
	if !g.Enabled() || ip == "" {
		return
	}
	g.mu.Lock()
	delete(g.entries, ip)
	g.mu.Unlock()
}

// Blocked cho biết địa chỉ có đang bị chặn không, kèm chi tiết lần chặn.
func (g *Guard) Blocked(ip string) (Block, bool) {
	if !g.Enabled() || ip == "" {
		return Block{}, false
	}

	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	e, ok := g.entries[ip]
	if !ok || e.until.IsZero() || !e.until.After(now) {
		return Block{}, false
	}
	return toBlock(ip, e), true
}

// List liệt kê các địa chỉ đang bị chặn, hết hạn muộn nhất lên đầu.
func (g *Guard) List() []Block {
	out := make([]Block, 0)
	if !g.Enabled() {
		return out
	}

	now := time.Now()

	g.mu.Lock()
	for ip, e := range g.entries {
		if e.until.After(now) {
			out = append(out, toBlock(ip, e))
		}
	}
	g.mu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].Until.After(out[j].Until) })
	return out
}

// Unblock gỡ chặn một địa chỉ và xóa luôn bộ đếm của nó.
func (g *Guard) Unblock(ip string) bool {
	if !g.Enabled() {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	e, ok := g.entries[ip]
	if !ok || !e.until.After(time.Now()) {
		return false
	}
	delete(g.entries, ip)
	return true
}

// Prune xóa các mục đã hết hạn chặn và không còn được đếm.
//
// Không có bước này thì mỗi địa chỉ từng gõ sai một lần sẽ nằm lại trong bộ
// nhớ mãi mãi, và một đợt quét từ hàng vạn địa chỉ đủ làm panel phình bộ nhớ.
func (g *Guard) Prune() int {
	if !g.Enabled() {
		return 0
	}

	now := time.Now()
	removed := 0

	g.mu.Lock()
	defer g.mu.Unlock()

	for ip, e := range g.entries {
		if e.until.After(now) {
			continue
		}
		if now.Sub(e.lastFail) > g.window {
			delete(g.entries, ip)
			removed++
		}
	}
	return removed
}

func toBlock(ip string, e *entry) Block {
	return Block{
		IP:        ip,
		Failures:  e.failures,
		BlockedAt: e.blockedAt,
		Until:     e.until,
		LastUser:  e.lastUser,
	}
}
