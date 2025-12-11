package metrics

import (
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/utils"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

var cfg *config.Config

// Store previous values for rate calculations
type previousMetrics struct {
	diskReadBytes  uint64
	diskWriteBytes uint64
	netBytesSent   uint64
	netBytesRecv   uint64
	timestamp      time.Time
}

var prevMetrics previousMetrics

func Init(c *config.Config) {
	cfg = c
	prevMetrics.timestamp = time.Now()
}

func CollectSystem() map[string]interface{} {
	metrics := make(map[string]interface{})
	now := time.Now()
	timeDelta := now.Sub(prevMetrics.timestamp).Seconds()

	for _, metric := range cfg.System.Metrics {
		switch metric {
		case "cpu_usage_percent":
			if percent, err := cpu.Percent(time.Second, false); err == nil && len(percent) > 0 {
				metrics["cpu_usage_percent"] = utils.Round(percent[0], 2)
			}
		case "cpu_usage_per_core":
			if percent, err := cpu.Percent(time.Second, true); err == nil {
				coreMetrics := make([]float64, len(percent))
				for i, p := range percent {
					coreMetrics[i] = utils.Round(p, 2)
				}
				metrics["cpu_usage_per_core"] = coreMetrics
			}
		case "cpu_count":
			if count, err := cpu.Counts(true); err == nil {
				metrics["cpu_count"] = count
			}
		case "cpu_count_physical":
			if count, err := cpu.Counts(false); err == nil {
				metrics["cpu_count_physical"] = count
			}
		case "cpu_load_1min":
			if avg, err := load.Avg(); err == nil {
				metrics["cpu_load_1min"] = utils.Round(avg.Load1, 2)
			}
		case "cpu_load_5min":
			if avg, err := load.Avg(); err == nil {
				metrics["cpu_load_5min"] = utils.Round(avg.Load5, 2)
			}
		case "cpu_load_15min":
			if avg, err := load.Avg(); err == nil {
				metrics["cpu_load_15min"] = utils.Round(avg.Load15, 2)
			}
		case "ram_usage_percent":
			if v, err := mem.VirtualMemory(); err == nil {
				metrics["ram_percent"] = utils.Round(v.UsedPercent, 2)
			}
		case "available_ram_mb":
			if v, err := mem.VirtualMemory(); err == nil {
				availableMB := float64(v.Available) / (1024 * 1024)
				metrics["available_ram_mb"] = utils.Round(availableMB, 2)
			}
		case "total_ram_mb":
			if v, err := mem.VirtualMemory(); err == nil {
				totalMB := float64(v.Total) / (1024 * 1024)
				metrics["total_ram_mb"] = utils.Round(totalMB, 2)
			}
		case "ram_cached_mb":
			if v, err := mem.VirtualMemory(); err == nil {
				cachedMB := float64(v.Cached) / (1024 * 1024)
				metrics["ram_cached_mb"] = utils.Round(cachedMB, 2)
			}
		case "ram_buffers_mb":
			if v, err := mem.VirtualMemory(); err == nil {
				buffersMB := float64(v.Buffers) / (1024 * 1024)
				metrics["ram_buffers_mb"] = utils.Round(buffersMB, 2)
			}
		case "swap_usage_percent":
			if s, err := mem.SwapMemory(); err == nil {
				metrics["swap_usage_percent"] = utils.Round(s.UsedPercent, 2)
			}
		case "swap_total_mb":
			if s, err := mem.SwapMemory(); err == nil {
				totalMB := float64(s.Total) / (1024 * 1024)
				metrics["swap_total_mb"] = utils.Round(totalMB, 2)
			}
		case "swap_used_mb":
			if s, err := mem.SwapMemory(); err == nil {
				usedMB := float64(s.Used) / (1024 * 1024)
				metrics["swap_used_mb"] = utils.Round(usedMB, 2)
			}
		case "disk_usage_percent":
			if usage, err := disk.Usage("/"); err == nil {
				metrics["disk_percent"] = utils.Round(usage.UsedPercent, 2)
			}
		case "available_disk_gb":
			if usage, err := disk.Usage("/"); err == nil {
				availableGB := float64(usage.Free) / (1024 * 1024 * 1024)
				metrics["available_disk_gb"] = utils.Round(availableGB, 2)
			}
		case "total_disk_gb":
			if usage, err := disk.Usage("/"); err == nil {
				totalGB := float64(usage.Total) / (1024 * 1024 * 1024)
				metrics["total_disk_gb"] = utils.Round(totalGB, 2)
			}
		case "inode_usage_percent":
			if usage, err := disk.Usage("/"); err == nil {
				metrics["inode_usage_percent"] = utils.Round(usage.InodesUsedPercent, 2)
			}
		case "disk_read_bytes":
			if counters, err := disk.IOCounters(); err == nil {
				var totalRead uint64
				for _, counter := range counters {
					totalRead += counter.ReadBytes
				}
				metrics["disk_read_bytes"] = totalRead
			}
		case "disk_write_bytes":
			if counters, err := disk.IOCounters(); err == nil {
				var totalWrite uint64
				for _, counter := range counters {
					totalWrite += counter.WriteBytes
				}
				metrics["disk_write_bytes"] = totalWrite
			}
		case "disk_read_bytes_per_sec":
			if counters, err := disk.IOCounters(); err == nil {
				var totalRead uint64
				for _, counter := range counters {
					totalRead += counter.ReadBytes
				}
				if prevMetrics.diskReadBytes > 0 && timeDelta > 0 {
					bytesPerSec := float64(totalRead-prevMetrics.diskReadBytes) / timeDelta
					metrics["disk_read_bytes_per_sec"] = utils.Round(bytesPerSec, 2)
				}
				prevMetrics.diskReadBytes = totalRead
			}
		case "disk_write_bytes_per_sec":
			if counters, err := disk.IOCounters(); err == nil {
				var totalWrite uint64
				for _, counter := range counters {
					totalWrite += counter.WriteBytes
				}
				if prevMetrics.diskWriteBytes > 0 && timeDelta > 0 {
					bytesPerSec := float64(totalWrite-prevMetrics.diskWriteBytes) / timeDelta
					metrics["disk_write_bytes_per_sec"] = utils.Round(bytesPerSec, 2)
				}
				prevMetrics.diskWriteBytes = totalWrite
			}
		case "disk_read_count":
			if counters, err := disk.IOCounters(); err == nil {
				var totalReads uint64
				for _, counter := range counters {
					totalReads += counter.ReadCount
				}
				metrics["disk_read_count"] = totalReads
			}
		case "disk_write_count":
			if counters, err := disk.IOCounters(); err == nil {
				var totalWrites uint64
				for _, counter := range counters {
					totalWrites += counter.WriteCount
				}
				metrics["disk_write_count"] = totalWrites
			}
		case "network_bytes_sent":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				metrics["network_bytes_sent"] = counters[0].BytesSent
			}
		case "network_bytes_recv":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				metrics["network_bytes_recv"] = counters[0].BytesRecv
			}
		case "network_bytes_sent_per_sec":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				if prevMetrics.netBytesSent > 0 && timeDelta > 0 {
					bytesPerSec := float64(counters[0].BytesSent-prevMetrics.netBytesSent) / timeDelta
					metrics["network_bytes_sent_per_sec"] = utils.Round(bytesPerSec, 2)
				}
				prevMetrics.netBytesSent = counters[0].BytesSent
			}
		case "network_bytes_recv_per_sec":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				if prevMetrics.netBytesRecv > 0 && timeDelta > 0 {
					bytesPerSec := float64(counters[0].BytesRecv-prevMetrics.netBytesRecv) / timeDelta
					metrics["network_bytes_recv_per_sec"] = utils.Round(bytesPerSec, 2)
				}
				prevMetrics.netBytesRecv = counters[0].BytesRecv
			}
		case "network_packets_sent":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				metrics["network_packets_sent"] = counters[0].PacketsSent
			}
		case "network_packets_recv":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				metrics["network_packets_recv"] = counters[0].PacketsRecv
			}
		case "network_errors_in":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				metrics["network_errors_in"] = counters[0].Errin
			}
		case "network_errors_out":
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				metrics["network_errors_out"] = counters[0].Errout
			}
		case "active_connections":
			if conns, err := net.Connections("all"); err == nil {
				metrics["active_connections"] = len(conns)
			}
		case "process_count":
			if procs, err := process.Processes(); err == nil {
				metrics["process_count"] = len(procs)
			}
		case "system_uptime_seconds":
			if info, err := host.Info(); err == nil {
				metrics["system_uptime_seconds"] = float64(info.Uptime)
			}
		case "boot_time_unix":
			if info, err := host.Info(); err == nil {
				metrics["boot_time_unix"] = info.BootTime
			}
		case "os_platform":
			if info, err := host.Info(); err == nil {
				metrics["os_platform"] = info.Platform
			}
		case "os_version":
			if info, err := host.Info(); err == nil {
				metrics["os_version"] = info.PlatformVersion
			}
		case "hostname":
			if info, err := host.Info(); err == nil {
				metrics["hostname"] = info.Hostname
			}
		case "kernel_version":
			if info, err := host.Info(); err == nil {
				metrics["kernel_version"] = info.KernelVersion
			}
		}
	}

	// Update timestamp for next collection
	prevMetrics.timestamp = now

	return metrics
}