package dockerx

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxLogBytes giới hạn dung lượng nhật ký đọc về một lần.
const maxLogBytes = 2 << 20 // 2 MB

// Container là thông tin một container mà panel hiển thị.
type Container struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	// State là trạng thái ngắn gọn: running, exited, paused, created…
	State string `json:"state"`
	// Status là mô tả dài của Docker, ví dụ "Up 3 hours".
	Status  string `json:"status"`
	Created int64  `json:"created"`
	// Ports là các cổng đã ánh xạ ra ngoài.
	Ports []PortMapping `json:"ports"`
	// Compose là tên dự án compose nếu container thuộc một dự án.
	Compose string `json:"compose,omitempty"`
}

// Running cho biết container có đang chạy hay không.
func (c Container) Running() bool { return c.State == "running" }

// PortMapping là một ánh xạ cổng của container.
type PortMapping struct {
	// PublicPort là cổng trên máy chủ; 0 nghĩa là chưa công bố ra ngoài.
	PublicPort int `json:"publicPort"`
	// PrivatePort là cổng bên trong container.
	PrivatePort int    `json:"privatePort"`
	Type        string `json:"type"`
	IP          string `json:"ip,omitempty"`
}

// containerJSON là dạng dữ liệu Docker trả về cho /containers/json.
type containerJSON struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Created int64    `json:"Created"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

// ListContainers liệt kê container; all quyết định có kèm container đã dừng không.
func (c *Client) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var raw []containerJSON
	path := "/containers/json" + query(map[string]string{"all": strconv.FormatBool(all)})
	if err := c.do(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}

	containers := make([]Container, 0, len(raw))
	for _, item := range raw {
		containers = append(containers, Container{
			ID:      shortID(item.ID),
			Name:    containerName(item.Names),
			Image:   item.Image,
			State:   item.State,
			Status:  item.Status,
			Created: item.Created,
			Ports:   convertPorts(item.Ports),
			Compose: item.Labels["com.docker.compose.project"],
		})
	}

	// Container đang chạy lên trước, rồi tới tên — danh sách Docker trả về không
	// có thứ tự ổn định nên giao diện sẽ nhảy lung tung sau mỗi lần làm mới.
	sort.Slice(containers, func(i, j int) bool {
		if containers[i].Running() != containers[j].Running() {
			return containers[i].Running()
		}
		return containers[i].Name < containers[j].Name
	})

	return containers, nil
}

// CreateOptions là tham số tạo một container mới.
type CreateOptions struct {
	// Name là tên container; để trống thì Docker tự đặt.
	Name string
	// Image là image dùng để tạo.
	Image string
	// Cmd ghi đè lệnh mặc định của image.
	Cmd []string
	// Env là biến môi trường dạng "KEY=VALUE".
	Env []string
	// Ports ánh xạ cổng máy chủ sang cổng container.
	Ports []PortMapping
	// RestartPolicy là chính sách khởi động lại, ví dụ "unless-stopped".
	RestartPolicy string
}

// CreateContainer tạo một container mới và trả về ID của nó.
func (c *Client) CreateContainer(ctx context.Context, opts CreateOptions) (string, error) {
	if err := validateImageRef(opts.Image); err != nil {
		return "", err
	}
	if opts.Name != "" {
		if err := validateID(opts.Name); err != nil {
			return "", err
		}
	}

	exposed := map[string]struct{}{}
	bindings := map[string][]map[string]string{}
	for _, port := range opts.Ports {
		proto := port.Type
		if proto == "" {
			proto = "tcp"
		}
		key := strconv.Itoa(port.PrivatePort) + "/" + proto
		exposed[key] = struct{}{}
		if port.PublicPort > 0 {
			bindings[key] = []map[string]string{{"HostPort": strconv.Itoa(port.PublicPort)}}
		}
	}

	payload := map[string]any{
		"Image":        opts.Image,
		"Env":          opts.Env,
		"ExposedPorts": exposed,
		"HostConfig": map[string]any{
			"PortBindings":  bindings,
			"RestartPolicy": map[string]string{"Name": opts.RestartPolicy},
		},
	}
	if len(opts.Cmd) > 0 {
		payload["Cmd"] = opts.Cmd
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("dựng yêu cầu tạo container: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	var created struct {
		ID string `json:"Id"`
	}
	params := map[string]string{}
	if opts.Name != "" {
		params["name"] = opts.Name
	}
	err = c.do(ctx, http.MethodPost, "/containers/create"+query(params),
		strings.NewReader(string(body)), &created)
	if err != nil {
		return "", err
	}

	return shortID(created.ID), nil
}

// StartContainer khởi động một container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.action(ctx, id, "start", nil)
}

// StopContainer dừng một container, chờ tối đa timeout giây trước khi buộc dừng.
func (c *Client) StopContainer(ctx context.Context, id string, timeoutSeconds int) error {
	return c.action(ctx, id, "stop", map[string]string{"t": strconv.Itoa(timeoutSeconds)})
}

// RestartContainer khởi động lại một container.
func (c *Client) RestartContainer(ctx context.Context, id string, timeoutSeconds int) error {
	return c.action(ctx, id, "restart", map[string]string{"t": strconv.Itoa(timeoutSeconds)})
}

// PauseContainer tạm dừng mọi tiến trình trong container.
func (c *Client) PauseContainer(ctx context.Context, id string) error {
	return c.action(ctx, id, "pause", nil)
}

// UnpauseContainer tiếp tục một container đang tạm dừng.
func (c *Client) UnpauseContainer(ctx context.Context, id string) error {
	return c.action(ctx, id, "unpause", nil)
}

// RemoveContainer xóa một container; force cho phép xóa cả khi đang chạy.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	if err := validateID(id); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	path := "/containers/" + id + query(map[string]string{"force": strconv.FormatBool(force)})
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) action(ctx context.Context, id, verb string, params map[string]string) error {
	if err := validateID(id); err != nil {
		return err
	}

	// Dừng và khởi động lại có thể mất tới vài chục giây với dịch vụ nặng.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return c.do(ctx, http.MethodPost, "/containers/"+id+"/"+verb+query(params), nil, nil)
}

// ContainerLogs đọc nhật ký gần nhất của một container.
func (c *Client) ContainerLogs(ctx context.Context, id string, lines int) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}
	if lines <= 0 || lines > 5000 {
		lines = 200
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	path := "/containers/" + id + "/logs" + query(map[string]string{
		"stdout": "true", "stderr": "true", "tail": strconv.Itoa(lines), "timestamps": "false",
	})

	body, err := c.stream(ctx, http.MethodGet, path)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()

	return demultiplexLogs(io.LimitReader(body, maxLogBytes))
}

// demultiplexLogs bóc khung nhật ký của Docker.
//
// Với container không cấp TTY, Docker chèn header 8 byte trước mỗi đoạn để phân
// biệt stdout và stderr. Không bóc lớp này thì nhật ký hiện ra kèm ký tự rác ở
// đầu mỗi dòng.
func demultiplexLogs(r io.Reader) (string, error) {
	var out strings.Builder
	header := make([]byte, 8)

	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return out.String(), nil
			}
			return out.String(), fmt.Errorf("đọc nhật ký: %w", err)
		}

		// Byte đầu là luồng (1 = stdout, 2 = stderr). Giá trị khác nghĩa là
		// container có TTY và dữ liệu không được đóng khung — trả về nguyên văn.
		if header[0] > 2 {
			rest, _ := io.ReadAll(r)
			return out.String() + string(header) + string(rest), nil
		}

		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}

		chunk := make([]byte, size)
		if _, err := io.ReadFull(r, chunk); err != nil {
			out.Write(chunk)
			return out.String(), nil
		}
		out.Write(chunk)
	}
}

// Stats là mức sử dụng tài nguyên tức thời của một container.
type Stats struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryUsage   uint64  `json:"memoryUsage"`
	MemoryLimit   uint64  `json:"memoryLimit"`
	MemoryPercent float64 `json:"memoryPercent"`
	NetworkRX     uint64  `json:"networkRx"`
	NetworkTX     uint64  `json:"networkTx"`
}

// statsJSON là dạng dữ liệu Docker trả về cho /containers/{id}/stats.
type statsJSON struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

// ContainerStats đọc mức sử dụng tài nguyên của một container.
func (c *Client) ContainerStats(ctx context.Context, id string) (Stats, error) {
	if err := validateID(id); err != nil {
		return Stats{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, err := c.stream(ctx, http.MethodGet,
		"/containers/"+id+"/stats"+query(map[string]string{"stream": "false"}))
	if err != nil {
		return Stats{}, err
	}
	defer func() { _ = body.Close() }()

	var raw statsJSON
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return Stats{}, fmt.Errorf("đọc số liệu container: %w", err)
	}

	return convertStats(raw), nil
}

// convertStats tính các con số hiển thị từ dữ liệu thô của Docker.
func convertStats(raw statsJSON) Stats {
	stats := Stats{
		MemoryLimit: raw.MemoryStats.Limit,
	}

	// Bộ nhớ đệm trang không phải bộ nhớ mà ứng dụng thực sự giữ; trừ ra để con
	// số khớp với thứ "docker stats" hiển thị.
	usage := raw.MemoryStats.Usage
	if cache, ok := raw.MemoryStats.Stats["inactive_file"]; ok && cache < usage {
		usage -= cache
	}
	stats.MemoryUsage = usage
	if raw.MemoryStats.Limit > 0 {
		stats.MemoryPercent = round2(float64(usage) / float64(raw.MemoryStats.Limit) * 100)
	}

	// CPU tính bằng hiệu giữa hai lần lấy mẫu mà Docker đã kèm sẵn trong phản hồi.
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(raw.CPUStats.SystemCPUUsage) - float64(raw.PreCPUStats.SystemCPUUsage)
	if cpuDelta > 0 && systemDelta > 0 {
		cpus := float64(raw.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = float64(len(raw.CPUStats.CPUUsage.PercpuUsage))
		}
		if cpus == 0 {
			cpus = 1
		}
		stats.CPUPercent = round2(cpuDelta / systemDelta * cpus * 100)
	}

	for _, network := range raw.Networks {
		stats.NetworkRX += network.RxBytes
		stats.NetworkTX += network.TxBytes
	}

	return stats
}

// containerName lấy tên hiển thị từ danh sách tên của Docker.
func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	// Docker trả tên kèm dấu gạch chéo đầu.
	return strings.TrimPrefix(names[0], "/")
}

func convertPorts(raw []struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}) []PortMapping {
	ports := make([]PortMapping, 0, len(raw))
	for _, item := range raw {
		ports = append(ports, PortMapping{
			IP:          item.IP,
			PrivatePort: item.PrivatePort,
			PublicPort:  item.PublicPort,
			Type:        item.Type,
		})
	}
	return ports
}

// shortID rút gọn ID 64 ký tự của Docker về 12 ký tự như dòng lệnh vẫn hiển thị.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
