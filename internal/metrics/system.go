package metrics

import (
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/utils"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

var cfg *config.Config

func Init(c *config.Config) {
	cfg = c
}

func CollectSystem() map[string]interface{} {
	metrics := make(map[string]interface{})

	for _, metric := range cfg.System.Metrics {
		switch metric {
		case "cpu_percent":
			if percent, err := cpu.Percent(time.Second, false); err == nil && len(percent) > 0 {
				metrics["cpu_percent"] = utils.Round(percent[0], 2)
			}
		case "ram_percent":
			if v, err := mem.VirtualMemory(); err == nil {
				metrics["ram_percent"] = utils.Round(v.UsedPercent, 2)
			}
		case "disk_percent":
			if usage, err := disk.Usage("/"); err == nil {
				metrics["disk_percent"] = utils.Round(usage.UsedPercent, 2)
			}
		case "available_ram_mb":
			if v, err := mem.VirtualMemory(); err == nil {
				// Convert bytes to megabytes for readability
				availableMB := float64(v.Available) / (1024 * 1024)
				metrics["available_ram_mb"] = utils.Round(availableMB, 2)
			}
		case "available_disk_gb":
			if usage, err := disk.Usage("/"); err == nil {
				// Convert bytes to gigabytes for readability
				availableGB := float64(usage.Free) / (1024 * 1024 * 1024)
				metrics["available_disk_gb"] = utils.Round(availableGB, 2)
			}
		case "total_ram_mb":
			if v, err := mem.VirtualMemory(); err == nil {
				totalMB := float64(v.Total) / (1024 * 1024)
				metrics["total_ram_mb"] = utils.Round(totalMB, 2)
			}
		case "total_disk_gb":
			if usage, err := disk.Usage("/"); err == nil {
				totalGB := float64(usage.Total) / (1024 * 1024 * 1024)
				metrics["total_disk_gb"] = utils.Round(totalGB, 2)
			}
		case "system_uptime_seconds":
			if info, err := host.Info(); err == nil {
				// Uptime in seconds (raw value from system)
				metrics["system_uptime_seconds"] = float64(info.Uptime)
			}
		}
	}

	return metrics
}