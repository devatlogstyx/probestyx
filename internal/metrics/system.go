// internal/metrics/system.go
package metrics

import (
	"sync"
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

var (
	prevMetrics  previousMetrics
	metricsMutex sync.RWMutex
)

// Cache for collected metrics
var (
	cachedMetrics    map[string]interface{}
	cacheTimestamp   time.Time
	cacheMutex       sync.RWMutex
	cacheTTL         time.Duration      // Will be set from config
	collectionMutex  sync.Mutex         // Prevents multiple simultaneous collections
)

func Init(c *config.Config) {
	cfg = c
	metricsMutex.Lock()
	prevMetrics.timestamp = time.Now()
	metricsMutex.Unlock()
	
	// Set cache TTL from config, default to 15 seconds
	if c.System.CacheTTL > 0 {
		cacheTTL = time.Duration(c.System.CacheTTL) * time.Second
	} else {
		cacheTTL = 15 * time.Second
	}
	
	// Initialize cache
	cacheMutex.Lock()
	cacheTimestamp = time.Time{} // Zero time so first request triggers collection
	cacheMutex.Unlock()
}

func CollectSystem() map[string]interface{} {
	// Fast path: return cached metrics if still valid
	cacheMutex.RLock()
	if time.Since(cacheTimestamp) < cacheTTL && cachedMetrics != nil {
		result := cachedMetrics
		cacheMutex.RUnlock()
		return result
	}
	cacheMutex.RUnlock()

	// Slow path: need to collect new metrics
	// Use collectionMutex to ensure only ONE goroutine collects at a time
	collectionMutex.Lock()
	defer collectionMutex.Unlock()

	// Double-check cache after acquiring lock (another goroutine might have just updated it)
	cacheMutex.RLock()
	if time.Since(cacheTimestamp) < cacheTTL && cachedMetrics != nil {
		result := cachedMetrics
		cacheMutex.RUnlock()
		return result
	}
	cacheMutex.RUnlock()

	// Actually collect metrics (only one request does this)
	metrics := doActualCollection()

	// Update cache
	cacheMutex.Lock()
	cachedMetrics = metrics
	cacheTimestamp = time.Now()
	cacheMutex.Unlock()

	return metrics
}

func doActualCollection() map[string]interface{} {
	metrics := make(map[string]interface{})
	var metricsMu sync.Mutex
	var wg sync.WaitGroup
	
	now := time.Now()
	
	// Read previous metrics with lock
	metricsMutex.RLock()
	timeDelta := now.Sub(prevMetrics.timestamp).Seconds()
	prevDiskRead := prevMetrics.diskReadBytes
	prevDiskWrite := prevMetrics.diskWriteBytes
	prevNetSent := prevMetrics.netBytesSent
	prevNetRecv := prevMetrics.netBytesRecv
	metricsMutex.RUnlock()

	// Create a map to check which metrics are requested
	requestedMetrics := make(map[string]bool)
	for _, metric := range cfg.System.Metrics {
		requestedMetrics[metric] = true
	}

	// Helper to safely add metrics
	addMetric := func(key string, value interface{}) {
		metricsMu.Lock()
		metrics[key] = value
		metricsMu.Unlock()
	}

	// CPU metrics (these have built-in delays)
	if requestedMetrics["cpu_usage_percent"] || requestedMetrics["cpu_usage_per_core"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if requestedMetrics["cpu_usage_percent"] {
				if percent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(percent) > 0 {
					addMetric("cpu_usage_percent", utils.Round(percent[0], 2))
				}
			}
			
			if requestedMetrics["cpu_usage_per_core"] {
				if percent, err := cpu.Percent(100*time.Millisecond, true); err == nil {
					coreMetrics := make([]float64, len(percent))
					for i, p := range percent {
						coreMetrics[i] = utils.Round(p, 2)
					}
					addMetric("cpu_usage_per_core", coreMetrics)
				}
			}
		}()
	}

	// CPU info and load (fast operations)
	if requestedMetrics["cpu_count"] || requestedMetrics["cpu_count_physical"] || 
	   requestedMetrics["cpu_load_1min"] || requestedMetrics["cpu_load_5min"] || 
	   requestedMetrics["cpu_load_15min"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if requestedMetrics["cpu_count"] {
				if count, err := cpu.Counts(true); err == nil {
					addMetric("cpu_count", count)
				}
			}
			
			if requestedMetrics["cpu_count_physical"] {
				if count, err := cpu.Counts(false); err == nil {
					addMetric("cpu_count_physical", count)
				}
			}
			
			if requestedMetrics["cpu_load_1min"] || requestedMetrics["cpu_load_5min"] || 
			   requestedMetrics["cpu_load_15min"] {
				if avg, err := load.Avg(); err == nil {
					if requestedMetrics["cpu_load_1min"] {
						addMetric("cpu_load_1min", utils.Round(avg.Load1, 2))
					}
					if requestedMetrics["cpu_load_5min"] {
						addMetric("cpu_load_5min", utils.Round(avg.Load5, 2))
					}
					if requestedMetrics["cpu_load_15min"] {
						addMetric("cpu_load_15min", utils.Round(avg.Load15, 2))
					}
				}
			}
		}()
	}

	// Memory metrics
	if requestedMetrics["ram_usage_percent"] || requestedMetrics["available_ram_mb"] || 
	   requestedMetrics["total_ram_mb"] || requestedMetrics["ram_cached_mb"] || 
	   requestedMetrics["ram_buffers_mb"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if v, err := mem.VirtualMemory(); err == nil {
				if requestedMetrics["ram_usage_percent"] {
					addMetric("ram_usage_percent", utils.Round(v.UsedPercent, 2))
				}
				if requestedMetrics["available_ram_mb"] {
					addMetric("available_ram_mb", utils.Round(float64(v.Available)/(1024*1024), 2))
				}
				if requestedMetrics["total_ram_mb"] {
					addMetric("total_ram_mb", utils.Round(float64(v.Total)/(1024*1024), 2))
				}
				if requestedMetrics["ram_cached_mb"] {
					addMetric("ram_cached_mb", utils.Round(float64(v.Cached)/(1024*1024), 2))
				}
				if requestedMetrics["ram_buffers_mb"] {
					addMetric("ram_buffers_mb", utils.Round(float64(v.Buffers)/(1024*1024), 2))
				}
			}
		}()
	}

	// Swap metrics
	if requestedMetrics["swap_usage_percent"] || requestedMetrics["swap_total_mb"] || 
	   requestedMetrics["swap_used_mb"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if s, err := mem.SwapMemory(); err == nil {
				if requestedMetrics["swap_usage_percent"] {
					addMetric("swap_usage_percent", utils.Round(s.UsedPercent, 2))
				}
				if requestedMetrics["swap_total_mb"] {
					addMetric("swap_total_mb", utils.Round(float64(s.Total)/(1024*1024), 2))
				}
				if requestedMetrics["swap_used_mb"] {
					addMetric("swap_used_mb", utils.Round(float64(s.Used)/(1024*1024), 2))
				}
			}
		}()
	}

	// Disk usage metrics
	if requestedMetrics["disk_usage_percent"] || requestedMetrics["available_disk_gb"] || 
	   requestedMetrics["total_disk_gb"] || requestedMetrics["inode_usage_percent"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if usage, err := disk.Usage("/"); err == nil {
				if requestedMetrics["disk_usage_percent"] {
					addMetric("disk_usage_percent", utils.Round(usage.UsedPercent, 2))
				}
				if requestedMetrics["available_disk_gb"] {
					addMetric("available_disk_gb", utils.Round(float64(usage.Free)/(1024*1024*1024), 2))
				}
				if requestedMetrics["total_disk_gb"] {
					addMetric("total_disk_gb", utils.Round(float64(usage.Total)/(1024*1024*1024), 2))
				}
				if requestedMetrics["inode_usage_percent"] {
					addMetric("inode_usage_percent", utils.Round(usage.InodesUsedPercent, 2))
				}
			}
		}()
	}

	// Disk I/O metrics
	diskIOMetrics := requestedMetrics["disk_read_bytes"] || requestedMetrics["disk_write_bytes"] ||
		requestedMetrics["disk_read_bytes_per_sec"] || requestedMetrics["disk_write_bytes_per_sec"] ||
		requestedMetrics["disk_read_count"] || requestedMetrics["disk_write_count"]
	
	if diskIOMetrics {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if counters, err := disk.IOCounters(); err == nil {
				var totalRead, totalWrite, totalReads, totalWrites uint64
				for _, counter := range counters {
					totalRead += counter.ReadBytes
					totalWrite += counter.WriteBytes
					totalReads += counter.ReadCount
					totalWrites += counter.WriteCount
				}
				
				if requestedMetrics["disk_read_bytes"] {
					addMetric("disk_read_bytes", totalRead)
				}
				if requestedMetrics["disk_write_bytes"] {
					addMetric("disk_write_bytes", totalWrite)
				}
				if requestedMetrics["disk_read_count"] {
					addMetric("disk_read_count", totalReads)
				}
				if requestedMetrics["disk_write_count"] {
					addMetric("disk_write_count", totalWrites)
				}
				
				if requestedMetrics["disk_read_bytes_per_sec"] && prevDiskRead > 0 && timeDelta > 0 {
					bytesPerSec := float64(totalRead-prevDiskRead) / timeDelta
					addMetric("disk_read_bytes_per_sec", utils.Round(bytesPerSec, 2))
				}
				if requestedMetrics["disk_write_bytes_per_sec"] && prevDiskWrite > 0 && timeDelta > 0 {
					bytesPerSec := float64(totalWrite-prevDiskWrite) / timeDelta
					addMetric("disk_write_bytes_per_sec", utils.Round(bytesPerSec, 2))
				}
				
				// Update stored values
				metricsMutex.Lock()
				prevMetrics.diskReadBytes = totalRead
				prevMetrics.diskWriteBytes = totalWrite
				metricsMutex.Unlock()
			}
		}()
	}

	// Network metrics
	netMetrics := requestedMetrics["network_bytes_sent"] || requestedMetrics["network_bytes_recv"] ||
		requestedMetrics["network_bytes_sent_per_sec"] || requestedMetrics["network_bytes_recv_per_sec"] ||
		requestedMetrics["network_packets_sent"] || requestedMetrics["network_packets_recv"] ||
		requestedMetrics["network_errors_in"] || requestedMetrics["network_errors_out"]
	
	if netMetrics {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
				c := counters[0]
				
				if requestedMetrics["network_bytes_sent"] {
					addMetric("network_bytes_sent", c.BytesSent)
				}
				if requestedMetrics["network_bytes_recv"] {
					addMetric("network_bytes_recv", c.BytesRecv)
				}
				if requestedMetrics["network_packets_sent"] {
					addMetric("network_packets_sent", c.PacketsSent)
				}
				if requestedMetrics["network_packets_recv"] {
					addMetric("network_packets_recv", c.PacketsRecv)
				}
				if requestedMetrics["network_errors_in"] {
					addMetric("network_errors_in", c.Errin)
				}
				if requestedMetrics["network_errors_out"] {
					addMetric("network_errors_out", c.Errout)
				}
				
				if requestedMetrics["network_bytes_sent_per_sec"] && prevNetSent > 0 && timeDelta > 0 {
					bytesPerSec := float64(c.BytesSent-prevNetSent) / timeDelta
					addMetric("network_bytes_sent_per_sec", utils.Round(bytesPerSec, 2))
				}
				if requestedMetrics["network_bytes_recv_per_sec"] && prevNetRecv > 0 && timeDelta > 0 {
					bytesPerSec := float64(c.BytesRecv-prevNetRecv) / timeDelta
					addMetric("network_bytes_recv_per_sec", utils.Round(bytesPerSec, 2))
				}
				
				// Update stored values
				metricsMutex.Lock()
				prevMetrics.netBytesSent = c.BytesSent
				prevMetrics.netBytesRecv = c.BytesRecv
				metricsMutex.Unlock()
			}
		}()
	}

	// Network connections
	if requestedMetrics["active_connections"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if conns, err := net.Connections("all"); err == nil {
				addMetric("active_connections", len(conns))
			}
		}()
	}

	// Process count
	if requestedMetrics["process_count"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if procs, err := process.Processes(); err == nil {
				addMetric("process_count", len(procs))
			}
		}()
	}

	// Host info
	hostMetrics := requestedMetrics["system_uptime_seconds"] || requestedMetrics["boot_time_unix"] ||
		requestedMetrics["os_platform"] || requestedMetrics["os_version"] ||
		requestedMetrics["hostname"] || requestedMetrics["kernel_version"]
	
	if hostMetrics {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if info, err := host.Info(); err == nil {
				if requestedMetrics["system_uptime_seconds"] {
					addMetric("system_uptime_seconds", float64(info.Uptime))
				}
				if requestedMetrics["boot_time_unix"] {
					addMetric("boot_time_unix", info.BootTime)
				}
				if requestedMetrics["os_platform"] {
					addMetric("os_platform", info.Platform)
				}
				if requestedMetrics["os_version"] {
					addMetric("os_version", info.PlatformVersion)
				}
				if requestedMetrics["hostname"] {
					addMetric("hostname", info.Hostname)
				}
				if requestedMetrics["kernel_version"] {
					addMetric("kernel_version", info.KernelVersion)
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Update timestamp
	metricsMutex.Lock()
	prevMetrics.timestamp = now
	metricsMutex.Unlock()

	return metrics
}