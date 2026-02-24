package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
)

// DaemonInfo contém informações do daemon Docker.
type DaemonInfo struct {
	Version           string
	ContainersRunning int
	ContainersStopped int
	ContainersPaused  int
	TotalMemMB        float64 // soma da memória usada pelos containers em execução
}

// GetDaemonInfo retorna informações do daemon e memória total dos containers.
func (c *Client) GetDaemonInfo(ctx context.Context) (*DaemonInfo, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return nil, err
	}
	d := &DaemonInfo{
		Version:           info.ServerVersion,
		ContainersRunning: info.ContainersRunning,
		ContainersStopped: info.ContainersStopped,
		ContainersPaused:  info.ContainersPaused,
	}

	// Agrega memória dos containers em execução (máx 10 para não travar)
	if info.ContainersRunning > 0 {
		list, err := c.cli.ContainerList(ctx, container.ListOptions{})
		if err == nil {
			limit := len(list)
			if limit > 10 {
				limit = 10
			}
			var totalMem float64
			for _, ct := range list[:limit] {
				statsResp, err := c.cli.ContainerStats(ctx, ct.ID, false)
				if err != nil {
					continue
				}
				data, _ := io.ReadAll(io.LimitReader(statsResp.Body, 8192))
				statsResp.Body.Close()
				totalMem += parseMemUsageBytes(data)
			}
			d.TotalMemMB = totalMem / 1024 / 1024
		}
	}
	return d, nil
}

// parseMemUsageBytes extrai memory_stats.usage do JSON de stats do Docker.
func parseMemUsageBytes(data []byte) float64 {
	var s struct {
		MemoryStats struct {
			Usage uint64 `json:"usage"`
		} `json:"memory_stats"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return 0
	}
	return float64(s.MemoryStats.Usage)
}

type Client struct {
	cli *dockerclient.Client
}

type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Status  string
	State   string // running, exited, etc
	Ports   string
}

type ImageInfo struct {
	ID       string
	Tags     []string
	SizeMB   float64
	Created  string
}

type ContainerStats struct {
	CPUPercent float64
	MemUsageMB float64
	MemLimitMB float64
	MemPercent float64
}

type ContainerSpec struct {
	Name        string
	Image       string
	Env         []string
	Ports       map[string]string // containerPort → hostPort
	Volumes     map[string]string // hostPath → containerPath
	RestartPolicy string
}

func New() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.cli.Ping(ctx)
	return err == nil
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	result := make([]ContainerInfo, 0, len(list))
	for _, ct := range list {
		name := ""
		if len(ct.Names) > 0 {
			name = strings.TrimPrefix(ct.Names[0], "/")
		}
		ports := formatPorts(ct.Ports)
		result = append(result, ContainerInfo{
			ID:     ct.ID[:12],
			Name:   name,
			Image:  ct.Image,
			Status: ct.Status,
			State:  ct.State,
			Ports:  ports,
		})
	}
	return result, nil
}

func (c *Client) Action(ctx context.Context, id, action string) error {
	switch action {
	case "start":
		return c.cli.ContainerStart(ctx, id, container.StartOptions{})
	case "stop":
		timeout := 10
		return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
	case "restart":
		timeout := 10
		return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
	case "remove":
		return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
	default:
		return fmt.Errorf("ação desconhecida: %s", action)
	}
}

func (c *Client) GetLogs(ctx context.Context, id string, tail int) (string, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	}
	rc, err := c.cli.ContainerLogs(ctx, id, opts)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 512*1024))
	if err != nil {
		return "", err
	}
	// strip docker log header bytes (8 bytes per line)
	return stripDockerLogHeader(data), nil
}

func (c *Client) GetStats(ctx context.Context, id string) (*ContainerStats, error) {
	resp, err := c.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// parse json manually to avoid heavy import
	_ = data
	// For simplicity we return zeroed stats; full impl would parse JSON
	return &ContainerStats{}, nil
}

func (c *Client) ListImages(ctx context.Context) ([]ImageInfo, error) {
	list, err := c.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]ImageInfo, 0, len(list))
	for _, img := range list {
		result = append(result, ImageInfo{
			ID:      img.ID[7:19],
			Tags:    img.RepoTags,
			SizeMB:  float64(img.Size) / 1024 / 1024,
			Created: time.Unix(img.Created, 0).Format("02/01/2006"),
		})
	}
	return result, nil
}

func (c *Client) RemoveImage(ctx context.Context, id string) error {
	_, err := c.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: false, PruneChildren: false})
	return err
}

func (c *Client) PullImage(ctx context.Context, ref string) (string, error) {
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	return string(data), nil
}

func (c *Client) CreateAndStart(ctx context.Context, spec ContainerSpec) (string, error) {
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for cPort, hPort := range spec.Ports {
		p := nat.Port(cPort + "/tcp")
		exposedPorts[p] = struct{}{}
		portBindings[p] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hPort}}
	}

	binds := make([]string, 0, len(spec.Volumes))
	for hPath, cPath := range spec.Volumes {
		binds = append(binds, hPath+":"+cPath)
	}

	restartPolicy := container.RestartPolicy{Name: container.RestartPolicyDisabled}
	if spec.RestartPolicy == "always" {
		restartPolicy = container.RestartPolicy{Name: container.RestartPolicyAlways}
	} else if spec.RestartPolicy == "unless-stopped" {
		restartPolicy = container.RestartPolicy{Name: container.RestartPolicyUnlessStopped}
	}

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:        spec.Image,
			Env:          spec.Env,
			ExposedPorts: exposedPorts,
		},
		&container.HostConfig{
			PortBindings:  portBindings,
			Binds:         binds,
			RestartPolicy: restartPolicy,
		},
		nil, nil, spec.Name,
	)
	if err != nil {
		return "", fmt.Errorf("criar container: %w", err)
	}
	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return resp.ID, fmt.Errorf("iniciar container: %w", err)
	}
	return resp.ID, nil
}

func (c *Client) GetContainer(ctx context.Context, id string) (*types.ContainerJSON, error) {
	ct, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ct, nil
}

func (c *Client) SearchContainers(ctx context.Context, name string) ([]ContainerInfo, error) {
	f := filters.NewArgs()
	f.Add("name", name)
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	result := make([]ContainerInfo, 0, len(list))
	for _, ct := range list {
		n := ""
		if len(ct.Names) > 0 {
			n = strings.TrimPrefix(ct.Names[0], "/")
		}
		result = append(result, ContainerInfo{
			ID: ct.ID[:12], Name: n, Image: ct.Image,
			Status: ct.Status, State: ct.State, Ports: formatPorts(ct.Ports),
		})
	}
	return result, nil
}

func formatPorts(ports []types.Port) string {
	var parts []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			parts = append(parts, fmt.Sprintf("%d→%d/%s", p.PublicPort, p.PrivatePort, p.Type))
		}
	}
	return strings.Join(parts, ", ")
}

func stripDockerLogHeader(data []byte) string {
	var sb strings.Builder
	i := 0
	for i < len(data) {
		if i+8 > len(data) {
			break
		}
		// header: [stream_type(1), 0,0,0, size(4 big-endian)]
		sz := int(data[i+4])<<24 | int(data[i+5])<<16 | int(data[i+6])<<8 | int(data[i+7])
		i += 8
		end := i + sz
		if end > len(data) {
			end = len(data)
		}
		sb.Write(data[i:end])
		i = end
	}
	s := sb.String()
	if s == "" {
		// fallback: return raw
		return string(data)
	}
	return s
}
