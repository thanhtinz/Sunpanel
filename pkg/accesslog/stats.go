package accesslog

import (
	"bufio"
	"context"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// maxReadBytes giới hạn lượng dữ liệu đọc ngược từ cuối tệp.
//
// Nhật ký truy cập của một website đông khách tính bằng GB mỗi ngày. Bản tóm
// tắt này để trả lời nhanh "đang có gì xảy ra", nên đọc tám megabyte cuối là đủ
// và không bao giờ làm panel ăn hết RAM của máy.
const maxReadBytes = 8 << 20

// topLimit là số dòng của mỗi bảng xếp hạng.
const topLimit = 10

// Count là một mục trong bảng xếp hạng.
type Count struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
	Bytes int64  `json:"bytes"`
}

// Bucket là số liệu của một khung thời gian.
type Bucket struct {
	// Start là mốc đầu khung, tính bằng mili giây Unix.
	Start int64 `json:"start"`
	// Requests, Errors và Bytes là số liệu trong khung đó.
	Requests int64 `json:"requests"`
	Errors   int64 `json:"errors"`
	Bytes    int64 `json:"bytes"`
}

// Failure là một yêu cầu hỏng, giữ lại để tìm nguyên nhân.
type Failure struct {
	Time   int64  `json:"time"`
	IP     string `json:"ip"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}

// Stats là bản tóm tắt nhật ký truy cập.
type Stats struct {
	// Requests, Visitors và Bytes là các con số tổng.
	Requests int64 `json:"requests"`
	Visitors int64 `json:"visitors"`
	Bytes    int64 `json:"bytes"`
	// Status2xx tới Status5xx là số yêu cầu theo nhóm mã trạng thái.
	Status2xx int64 `json:"status2xx"`
	Status3xx int64 `json:"status3xx"`
	Status4xx int64 `json:"status4xx"`
	Status5xx int64 `json:"status5xx"`

	TopPaths     []Count `json:"topPaths"`
	TopIPs       []Count `json:"topIps"`
	TopReferrers []Count `json:"topReferrers"`
	TopAgents    []Count `json:"topAgents"`

	// Buckets là lưu lượng theo từng khung thời gian, cũ nhất trước.
	Buckets []Bucket `json:"buckets"`
	// BucketSeconds là độ dài một khung.
	BucketSeconds int64 `json:"bucketSeconds"`
	// Failures là các yêu cầu hỏng gần nhất.
	Failures []Failure `json:"failures"`

	// Lines là số dòng đã đọc, Skipped là số dòng không đọc được.
	Lines   int64 `json:"lines"`
	Skipped int64 `json:"skipped"`
	// Truncated cho biết tệp lớn hơn phần đã đọc, nên số liệu chỉ tính từ giữa
	// tệp trở đi chứ không phải toàn bộ lịch sử.
	Truncated bool `json:"truncated"`
	// From và To là khoảng thời gian thật sự của dữ liệu đã đọc.
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// Analyzer đọc và tóm tắt nhật ký truy cập qua lớp host.
type Analyzer struct {
	fs host.FileSystem
}

// New tạo bộ phân tích.
func New(fs host.FileSystem) *Analyzer { return &Analyzer{fs: fs} }

// maxFailures là số yêu cầu hỏng giữ lại.
const maxFailures = 50

// Analyze đọc phần cuối tệp và tóm tắt các yêu cầu trong khoảng window gần nhất.
//
// Mốc thời gian lấy từ dòng cuối cùng của tệp chứ không phải đồng hồ máy: nhật
// ký của một website không có khách nào từ hôm qua vẫn phải cho ra số liệu của
// hôm qua, thay vì một trang trống trơn khiến người dùng tưởng tính năng hỏng.
func (a *Analyzer) Analyze(ctx context.Context, path string, window time.Duration) (Stats, error) {
	info, err := a.fs.Stat(ctx, path)
	if err != nil {
		return Stats{}, err
	}

	start := int64(0)
	if info.Size > maxReadBytes {
		start = info.Size - maxReadBytes
	}

	entries, stats, err := a.read(ctx, path, start)
	if err != nil {
		return Stats{}, err
	}
	stats.Truncated = start > 0

	if len(entries) == 0 {
		stats.TopPaths, stats.TopIPs = []Count{}, []Count{}
		stats.TopReferrers, stats.TopAgents = []Count{}, []Count{}
		stats.Buckets, stats.Failures = []Bucket{}, []Failure{}
		return stats, nil
	}

	newest := entries[len(entries)-1].Time
	cutoff := newest.Add(-window)

	return summarize(entries, cutoff, bucketSize(window), stats), nil
}

// bucketSteps là các độ dài khung được phép, từ nhỏ tới lớn.
var bucketSteps = []time.Duration{
	time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute,
	time.Hour, 3 * time.Hour, 6 * time.Hour, 24 * time.Hour,
}

// bucketSize chọn độ dài khung sao cho biểu đồ có khoảng hai chục cột.
//
// Chia theo giờ cho mọi khoảng thì biểu đồ của một giờ chỉ còn đúng một cột —
// một cột đơn độc không vẽ thành đường, và người xem thấy một khung trống trơn
// dù số liệu bên trên vẫn có.
func bucketSize(window time.Duration) time.Duration {
	target := window / 24
	for _, step := range bucketSteps {
		if step >= target {
			return step
		}
	}
	return bucketSteps[len(bucketSteps)-1]
}

// read đọc từ vị trí start tới hết tệp và tách từng dòng.
func (a *Analyzer) read(ctx context.Context, path string, start int64) ([]Entry, Stats, error) {
	var stats Stats

	reader, err := a.fs.Open(ctx, path)
	if err != nil {
		return nil, stats, err
	}
	defer func() { _ = reader.Close() }()

	if start > 0 {
		seeker, ok := reader.(io.Seeker)
		if !ok {
			return nil, stats, err
		}
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return nil, stats, err
		}
	}

	scanner := bufio.NewScanner(io.LimitReader(reader, maxReadBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	entries := make([]Entry, 0, 4096)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		// Đoạn đọc từ giữa tệp gần như luôn bắt đầu ở giữa một dòng.
		if first && start > 0 {
			first = false
			continue
		}
		first = false

		stats.Lines++
		entry, ok := Parse(line)
		if !ok {
			stats.Skipped++
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, stats, err
	}
	return entries, stats, nil
}

// summarize gom các bản ghi từ mốc cutoff trở đi thành số liệu.
func summarize(entries []Entry, cutoff time.Time, step time.Duration, stats Stats) Stats {
	stats.BucketSeconds = int64(step.Seconds())

	paths := make(map[string]*Count)
	ips := make(map[string]*Count)
	referrers := make(map[string]*Count)
	agents := make(map[string]*Count)
	buckets := make(map[int64]*Bucket)
	visitors := make(map[string]struct{})

	for _, entry := range entries {
		if entry.Time.Before(cutoff) {
			continue
		}

		if stats.From == 0 {
			stats.From = entry.Time.UnixMilli()
		}
		stats.To = entry.Time.UnixMilli()

		stats.Requests++
		stats.Bytes += entry.Bytes
		visitors[entry.IP] = struct{}{}

		switch {
		case entry.Status >= 500:
			stats.Status5xx++
		case entry.Status >= 400:
			stats.Status4xx++
		case entry.Status >= 300:
			stats.Status3xx++
		default:
			stats.Status2xx++
		}

		// Tham số truy vấn bị cắt: "/tim?q=a" và "/tim?q=b" là cùng một trang, và
		// giữ nguyên chúng thì bảng xếp hạng chỉ toàn các dòng đếm được số 1.
		add(paths, trimQuery(entry.Path), entry.Bytes)
		add(ips, entry.IP, entry.Bytes)
		if entry.Referrer != "" && entry.Referrer != "-" {
			add(referrers, entry.Referrer, 0)
		}
		if entry.UserAgent != "" && entry.UserAgent != "-" {
			add(agents, entry.UserAgent, 0)
		}

		start := entry.Time.Truncate(step).UnixMilli()
		bucket, ok := buckets[start]
		if !ok {
			bucket = &Bucket{Start: start}
			buckets[start] = bucket
		}
		bucket.Requests++
		bucket.Bytes += entry.Bytes
		if entry.Status >= 400 {
			bucket.Errors++
		}

		if entry.Status >= 400 {
			stats.Failures = append(stats.Failures, Failure{
				Time:   entry.Time.UnixMilli(),
				IP:     entry.IP,
				Path:   entry.Path,
				Status: entry.Status,
			})
		}
	}

	stats.Visitors = int64(len(visitors))
	stats.TopPaths = top(paths)
	stats.TopIPs = top(ips)
	stats.TopReferrers = top(referrers)
	stats.TopAgents = top(agents)

	stats.Buckets = make([]Bucket, 0, len(buckets))
	for _, bucket := range buckets {
		stats.Buckets = append(stats.Buckets, *bucket)
	}
	sort.Slice(stats.Buckets, func(i, j int) bool { return stats.Buckets[i].Start < stats.Buckets[j].Start })

	// Lỗi mới nhất lên đầu: đó là thứ đang hỏng ngay lúc này.
	if len(stats.Failures) > maxFailures {
		stats.Failures = stats.Failures[len(stats.Failures)-maxFailures:]
	}
	for i, j := 0, len(stats.Failures)-1; i < j; i, j = i+1, j-1 {
		stats.Failures[i], stats.Failures[j] = stats.Failures[j], stats.Failures[i]
	}
	if stats.Failures == nil {
		stats.Failures = []Failure{}
	}
	return stats
}

func add(table map[string]*Count, key string, bytes int64) {
	item, ok := table[key]
	if !ok {
		item = &Count{Key: key}
		table[key] = item
	}
	item.Count++
	item.Bytes += bytes
}

// top lấy các mục nhiều lượt nhất, nhiều nhất lên đầu.
func top(table map[string]*Count) []Count {
	out := make([]Count, 0, len(table))
	for _, item := range table {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > topLimit {
		out = out[:topLimit]
	}
	return out
}

func trimQuery(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		return path[:i]
	}
	return path
}
