package dockerx

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Image là một image đang có trên máy.
type Image struct {
	ID string `json:"id"`
	// Tags là các tên đã gắn; image không tên sẽ có danh sách rỗng.
	Tags    []string `json:"tags"`
	Size    int64    `json:"size"`
	Created int64    `json:"created"`
	// Dangling cho biết image không còn tên nào trỏ tới — thường là rác sau khi
	// build lại và chiếm chỗ mà không ai để ý.
	Dangling bool `json:"dangling"`
}

type imageJSON struct {
	ID       string   `json:"Id"`
	RepoTags []string `json:"RepoTags"`
	Size     int64    `json:"Size"`
	Created  int64    `json:"Created"`
}

// ListImages liệt kê image trên máy.
func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var raw []imageJSON
	if err := c.do(ctx, http.MethodGet, "/images/json", nil, &raw); err != nil {
		return nil, err
	}

	images := make([]Image, 0, len(raw))
	for _, item := range raw {
		tags := item.RepoTags
		// Docker biểu diễn image không tên bằng "<none>:<none>".
		dangling := len(tags) == 0 || (len(tags) == 1 && tags[0] == "<none>:<none>")
		if dangling {
			tags = nil
		}

		images = append(images, Image{
			ID:       shortID(strings.TrimPrefix(item.ID, "sha256:")),
			Tags:     tags,
			Size:     item.Size,
			Created:  item.Created,
			Dangling: dangling,
		})
	}

	sort.Slice(images, func(i, j int) bool { return images[i].Created > images[j].Created })
	return images, nil
}

// RemoveImage xóa một image.
func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	if err := validateID(id); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	path := "/images/" + id + query(map[string]string{"force": strconv.FormatBool(force)})
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// PullImage tải một image về máy.
//
// Docker trả về luồng tiến trình dạng JSON từng dòng; panel đọc hết luồng để
// biết khi nào xong, nhưng không hiển thị từng bước.
func (c *Client) PullImage(ctx context.Context, ref string) error {
	if err := validateImageRef(ref); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, pullTimeout)
	defer cancel()

	body, err := c.stream(ctx, http.MethodPost,
		"/images/create"+query(map[string]string{"fromImage": ref}))
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()

	// Phải đọc hết luồng vì hai lý do: đóng sớm sẽ hủy việc tải giữa chừng, và
	// Docker báo lỗi tải NGAY TRONG luồng chứ không qua mã trạng thái HTTP —
	// một lần tải thất bại vẫn trả về 200 OK.
	return readProgressStream(body)
}

// Volume là một volume của Docker.
type Volume struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
	CreatedAt  string `json:"createdAt"`
}

// ListVolumes liệt kê volume.
func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var payload struct {
		Volumes []struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
			CreatedAt  string `json:"CreatedAt"`
		} `json:"Volumes"`
	}
	if err := c.do(ctx, http.MethodGet, "/volumes", nil, &payload); err != nil {
		return nil, err
	}

	volumes := make([]Volume, 0, len(payload.Volumes))
	for _, item := range payload.Volumes {
		volumes = append(volumes, Volume{
			Name: item.Name, Driver: item.Driver,
			Mountpoint: item.Mountpoint, CreatedAt: item.CreatedAt,
		})
	}

	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Name < volumes[j].Name })
	return volumes, nil
}

// RemoveVolume xóa một volume.
func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	if err := validateID(name); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	path := "/volumes/" + name + query(map[string]string{"force": strconv.FormatBool(force)})
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// Network là một mạng của Docker.
type Network struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
	// Internal cho biết mạng không có đường ra ngoài.
	Internal bool `json:"internal"`
	// Subnets là các dải địa chỉ của mạng.
	Subnets []string `json:"subnets"`
}

// ListNetworks liệt kê mạng.
func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var raw []struct {
		ID       string `json:"Id"`
		Name     string `json:"Name"`
		Driver   string `json:"Driver"`
		Scope    string `json:"Scope"`
		Internal bool   `json:"Internal"`
		IPAM     struct {
			Config []struct {
				Subnet string `json:"Subnet"`
			} `json:"Config"`
		} `json:"IPAM"`
	}
	if err := c.do(ctx, http.MethodGet, "/networks", nil, &raw); err != nil {
		return nil, err
	}

	networks := make([]Network, 0, len(raw))
	for _, item := range raw {
		subnets := make([]string, 0, len(item.IPAM.Config))
		for _, config := range item.IPAM.Config {
			if config.Subnet != "" {
				subnets = append(subnets, config.Subnet)
			}
		}
		networks = append(networks, Network{
			ID: shortID(item.ID), Name: item.Name, Driver: item.Driver,
			Scope: item.Scope, Internal: item.Internal, Subnets: subnets,
		})
	}

	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	return networks, nil
}

// RemoveNetwork xóa một mạng.
func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	return c.do(ctx, http.MethodDelete, "/networks/"+id, nil, nil)
}

// SystemInfo là thông tin tổng quan về Docker.
type SystemInfo struct {
	Version           string `json:"version"`
	APIVersion        string `json:"apiVersion"`
	Containers        int    `json:"containers"`
	ContainersRunning int    `json:"containersRunning"`
	Images            int    `json:"images"`
	StorageDriver     string `json:"storageDriver"`
	// DiskUsed là tổng dung lượng Docker đang chiếm.
	DiskUsed int64 `json:"diskUsed"`
	// Reclaimable là dung lượng có thể giải phóng bằng cách dọn rác.
	Reclaimable int64 `json:"reclaimable"`
}

// Info đọc thông tin tổng quan về Docker.
func (c *Client) Info(ctx context.Context) (SystemInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var info struct {
		Containers        int    `json:"Containers"`
		ContainersRunning int    `json:"ContainersRunning"`
		Images            int    `json:"Images"`
		Driver            string `json:"Driver"`
		ServerVersion     string `json:"ServerVersion"`
	}
	if err := c.do(ctx, http.MethodGet, "/info", nil, &info); err != nil {
		return SystemInfo{}, err
	}

	out := SystemInfo{
		Version:           info.ServerVersion,
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		Images:            info.Images,
		StorageDriver:     info.Driver,
	}

	// Dung lượng nằm ở endpoint khác; thiếu nó không đáng để cả lời gọi thất bại.
	if usage, err := c.diskUsage(ctx); err == nil {
		out.DiskUsed = usage.used
		out.Reclaimable = usage.reclaimable
	}

	return out, nil
}

type diskUsage struct {
	used        int64
	reclaimable int64
}

// diskUsage đọc dung lượng Docker đang chiếm và phần dọn được.
func (c *Client) diskUsage(ctx context.Context) (diskUsage, error) {
	var payload struct {
		LayersSize int64 `json:"LayersSize"`
		Images     []struct {
			Size       int64 `json:"Size"`
			Containers int   `json:"Containers"`
		} `json:"Images"`
	}
	if err := c.do(ctx, http.MethodGet, "/system/df", nil, &payload); err != nil {
		return diskUsage{}, err
	}

	usage := diskUsage{used: payload.LayersSize}
	for _, image := range payload.Images {
		// Image không container nào dùng là phần dọn được.
		if image.Containers == 0 {
			usage.reclaimable += image.Size
		}
	}
	return usage, nil
}

// Prune dọn các đối tượng không còn dùng và trả về dung lượng đã giải phóng.
func (c *Client) Prune(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var freed int64

	// Dọn theo thứ tự container trước rồi tới image: image chỉ giải phóng được
	// sau khi không còn container nào tham chiếu.
	var containers struct {
		SpaceReclaimed int64 `json:"SpaceReclaimed"`
	}
	if err := c.do(ctx, http.MethodPost, "/containers/prune", nil, &containers); err != nil {
		return freed, err
	}
	freed += containers.SpaceReclaimed

	var images struct {
		SpaceReclaimed int64 `json:"SpaceReclaimed"`
	}
	if err := c.do(ctx, http.MethodPost, "/images/prune", nil, &images); err != nil {
		return freed, err
	}
	freed += images.SpaceReclaimed

	return freed, nil
}
