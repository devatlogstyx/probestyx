package metrics

import (
	"time"

	"github.com/devatlogstyx/probestyx/internal/config"
	"github.com/devatlogstyx/probestyx/internal/utils"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
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
		case "cpu":
			if percent, err := cpu.Percent(time.Second, false); err == nil && len(percent) > 0 {
				metrics["cpu"] = utils.Round(percent[0], 2)
			}
		case "ram":
			if v, err := mem.VirtualMemory(); err == nil {
				metrics["ram"] = utils.Round(v.UsedPercent, 2)
			}
		case "disk":
			if usage, err := disk.Usage("/"); err == nil {
				metrics["disk"] = utils.Round(usage.UsedPercent, 2)
			}
		}
	}

	return metrics
}