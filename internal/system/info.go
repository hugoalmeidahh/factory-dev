package system

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type CPUInfo struct {
	Percent []float64
	Overall float64
	Cores   int
}

type MemInfo struct {
	Total     uint64
	Used      uint64
	Available uint64
	Percent   float64
	SwapTotal uint64
	SwapUsed  uint64
}

type DiskInfo struct {
	Mount   string
	Total   uint64
	Used    uint64
	Percent float64
}

type NetInfo struct {
	Name      string
	BytesSent uint64
	BytesRecv uint64
}

type SystemInfo struct {
	Hostname       string
	OS             string
	Kernel         string
	Uptime         string
	CPU            CPUInfo
	Mem            MemInfo
	Disks          []DiskInfo
	Nets           []NetInfo
	GoroutineCount int
	GoMemAllocMB   float64
}

// Gather coleta informações do sistema com timeout interno de 4s.
func Gather(ctx context.Context) (*SystemInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	info := &SystemInfo{}

	// Host info
	if hi, err := host.InfoWithContext(ctx); err == nil {
		info.Hostname = hi.Hostname
		info.OS = hi.OS + " " + hi.Platform + " " + hi.PlatformVersion
		info.Kernel = hi.KernelVersion
		info.Uptime = formatUptime(hi.Uptime)
	}

	// CPU
	if percs, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, true); err == nil {
		info.CPU.Percent = percs
		var total float64
		for _, p := range percs {
			total += p
		}
		if len(percs) > 0 {
			info.CPU.Overall = total / float64(len(percs))
		}
		info.CPU.Cores = len(percs)
	}

	// Memory
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		info.Mem.Total = vm.Total
		info.Mem.Used = vm.Used
		info.Mem.Available = vm.Available
		info.Mem.Percent = vm.UsedPercent
	}
	if sm, err := mem.SwapMemoryWithContext(ctx); err == nil {
		info.Mem.SwapTotal = sm.Total
		info.Mem.SwapUsed = sm.Used
	}

	// Disks
	if parts, err := disk.PartitionsWithContext(ctx, false); err == nil {
		for _, p := range parts {
			if !strings.HasPrefix(p.Mountpoint, "/") {
				continue
			}
			// Skip pseudo filesystems
			if strings.Contains(p.Fstype, "tmpfs") || strings.Contains(p.Fstype, "devtmpfs") ||
				strings.Contains(p.Fstype, "squashfs") || strings.HasPrefix(p.Mountpoint, "/sys") ||
				strings.HasPrefix(p.Mountpoint, "/proc") || strings.HasPrefix(p.Mountpoint, "/dev") {
				continue
			}
			if us, err2 := disk.UsageWithContext(ctx, p.Mountpoint); err2 == nil {
				info.Disks = append(info.Disks, DiskInfo{
					Mount:   p.Mountpoint,
					Total:   us.Total,
					Used:    us.Used,
					Percent: us.UsedPercent,
				})
			}
		}
	}

	// Network
	if counters, err := net.IOCountersWithContext(ctx, true); err == nil {
		for _, c := range counters {
			if c.Name == "lo" {
				continue
			}
			info.Nets = append(info.Nets, NetInfo{
				Name:      c.Name,
				BytesSent: c.BytesSent,
				BytesRecv: c.BytesRecv,
			})
		}
	}

	// Go runtime
	info.GoroutineCount = runtime.NumGoroutine()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	info.GoMemAllocMB = float64(ms.Alloc) / 1024 / 1024

	return info, nil
}

// SetHostname define o hostname via hostnamectl (Linux com systemd).
func SetHostname(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("hostname não pode ser vazio")
	}
	cmd := exec.Command("hostnamectl", "set-hostname", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hostnamectl: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// MB converte bytes para megabytes.
func MB(b uint64) float64 { return float64(b) / 1024 / 1024 }

// GB converte bytes para gigabytes.
func GB(b uint64) float64 { return float64(b) / 1024 / 1024 / 1024 }

func formatUptime(seconds uint64) string {
	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
