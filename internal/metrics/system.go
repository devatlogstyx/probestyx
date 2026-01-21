// internal/metrics/system.go
package metrics

import (
	"sync"
	"sync/atomic"
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
	timestamp      int64 // Use Unix nano for atomic operations
}

var prevMetrics previousMetrics

// Cache for collected metrics
var (
	cachedMetrics   atomic.Value // Stores map[string]interface{}
	cacheTimestamp  atomic.Int64 // Unix nano timestamp
	cacheTTL        time.Duration
	collectionMutex sync.Mutex
)

// Pre-parsed metric groups for faster lookup
type metricGroups struct {
	cpuUsage      bool
	cpuInfo       bool
	memory        bool
	swap          bool
	diskUsage     bool
	diskIO        bool
	network       bool
	netConn       bool
	processCount  bool
	hostInfo      bool
}

var groups metricGroups

func Init(c *config.Config) {
	cfg = c
	atomic.StoreInt64(&prevMetrics.timestamp, time.Now().UnixNano())
	
	// Set cache TTL from config, default to 15 seconds
	if c.System.CacheTTL > 0 {
		cacheTTL = time.Duration(c.System.CacheTTL) * time.Second
	} else {
		cacheTTL = 15 * time.Second
	}
	
	// Pre-parse requested metrics into groups (done once at startup)
	requestedMetrics := make(map[string]bool, len(c.System.Metrics))
	for _, metric := range c.System.Metrics {
		requestedMetrics[metric] = true
	}
	
	groups.cpuUsage = requestedMetrics["cpu_usage_percent"] || requestedMetrics["cpu_usage_per_core"]
	groups.cpuInfo = requestedMetrics["cpu_count"] || requestedMetrics["cpu_count_physical"] ||
		requestedMetrics["cpu_load_1min"] || requestedMetrics["cpu_load_5min"] || requestedMetrics["cpu_load_15min"]
	groups.memory = requestedMetrics["ram_usage_percent"] || requestedMetrics["available_ram_mb"] ||
		requestedMetrics["total_ram_mb"] || requestedMetrics["ram_cached_mb"] || requestedMetrics["ram_buffers_mb"]
	groups.swap = requestedMetrics["swap_usage_percent"] || requestedMetrics["swap_total_mb"] || requestedMetrics["swap_used_mb"]
	groups.diskUsage = requestedMetrics["disk_usage_percent"] || requestedMetrics["available_disk_gb"] ||
		requestedMetrics["total_disk_gb"] || requestedMetrics["inode_usage_percent"]
	groups.diskIO = requestedMetrics["disk_read_bytes"] || requestedMetrics["disk_write_bytes"] ||
		requestedMetrics["disk_read_bytes_per_sec"] || requestedMetrics["disk_write_bytes_per_sec"] ||
		requestedMetrics["disk_read_count"] || requestedMetrics["disk_write_count"]
	groups.network = requestedMetrics["network_bytes_sent"] || requestedMetrics["network_bytes_recv"] ||
		requestedMetrics["network_bytes_sent_per_sec"] || requestedMetrics["network_bytes_recv_per_sec"] ||
		requestedMetrics["network_packets_sent"] || requestedMetrics["network_packets_recv"] ||
		requestedMetrics["network_errors_in"] || requestedMetrics["network_errors_out"]
	groups.netConn = requestedMetrics["active_connections"]
	groups.processCount = requestedMetrics["process_count"]
	groups.hostInfo = requestedMetrics["system_uptime_seconds"] || requestedMetrics["boot_time_unix"] ||
		requestedMetrics["os_platform"] || requestedMetrics["os_version"] ||
		requestedMetrics["hostname"] || requestedMetrics["kernel_version"]
	
	// Initialize cache timestamp to zero
	cacheTimestamp.Store(0)
}

func CollectSystem() map[string]interface{} {
	// Fast path: return cached metrics if still valid
	cachedTime := cacheTimestamp.Load()
	if cachedTime > 0 {
		elapsed := time.Since(time.Unix(0, cachedTime))
		if elapsed < cacheTTL {
			if cached := cachedMetrics.Load(); cached != nil {
				return cached.(map[string]interface{})
			}
		}
	}

	// Slow path: need to collect new metrics
	collectionMutex.Lock()
	defer collectionMutex.Unlock()

	// Double-check cache after acquiring lock
	cachedTime = cacheTimestamp.Load()
	if cachedTime > 0 {
		elapsed := time.Since(time.Unix(0, cachedTime))
		if elapsed < cacheTTL {
			if cached := cachedMetrics.Load(); cached != nil {
				return cached.(map[string]interface{})
			}
		}
	}

	// Actually collect metrics
	metrics := doActualCollection()

	// Update cache atomically
	cachedMetrics.Store(metrics)
	cacheTimestamp.Store(time.Now().UnixNano())

	return metrics
}

func doActualCollection() map[string]interface{} {
	// Pre-allocate map with estimated capacity
	metrics := make(map[string]interface{}, 32)
	var metricsMu sync.Mutex
	var wg sync.WaitGroup
	
	now := time.Now()
	nowNano := now.UnixNano()
	
	// Read previous metrics atomically
	prevTimestamp := atomic.LoadInt64(&prevMetrics.timestamp)
	timeDelta := float64(nowNano-prevTimestamp) / 1e9
	prevDiskRead := atomic.LoadUint64(&prevMetrics.diskReadBytes)
	prevDiskWrite := atomic.LoadUint64(&prevMetrics.diskWriteBytes)
	prevNetSent := atomic.LoadUint64(&prevMetrics.netBytesSent)
	prevNetRecv := atomic.LoadUint64(&prevMetrics.netBytesRecv)

	// Create a map to check which metrics are requested
	requestedMetrics := make(map[string]bool, len(cfg.System.Metrics))
	for _, metric := range cfg.System.Metrics {
		requestedMetrics[metric] = true
	}

	// Helper to safely add metrics (using pointer to reduce allocations)
	addMetric := func(key string, value interface{}) {
		metricsMu.Lock()
		metrics[key] = value
		metricsMu.Unlock()
	}

	// CPU metrics - group related operations
	if groups.cpuUsage {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if requestedMetrics["cpu_usage_percent"] && requestedMetrics["cpu_usage_per_core"] {
				// Collect both in one call
				if percent, err := cpu.Percent(100*time.Millisecond, true); err == nil && len(percent) > 0 {
					// Overall is average of cores
					var total float64
					coreMetrics := make([]float64, len(percent))
					for i, p := range percent {
						rounded := utils.Round(p, 2)
						coreMetrics[i] = rounded
						total += rounded
					}
					addMetric("cpu_usage_per_core", coreMetrics)
					addMetric("cpu_usage_percent", utils.Round(total/float64(len(percent)), 2))
				}
			} else if requestedMetrics["cpu_usage_percent"] {
				if percent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(percent) > 0 {
					addMetric("cpu_usage_percent", utils.Round(percent[0], 2))
				}
			} else if requestedMetrics["cpu_usage_per_core"] {
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

	// CPU info and load
	if groups.cpuInfo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if requestedMetrics["cpu_count"] || requestedMetrics["cpu_count_physical"] {
				// Collect both counts if needed
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
			}
			
			if requestedMetrics["cpu_load_1min"] || requestedMetrics["cpu_load_5min"] || requestedMetrics["cpu_load_15min"] {
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
	if groups.memory {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if v, err := mem.VirtualMemory(); err == nil {
				if requestedMetrics["ram_usage_percent"] {
					addMetric("ram_usage_percent", utils.Round(v.UsedPercent, 2))
				}
				if requestedMetrics["available_ram_mb"] {
					addMetric("available_ram_mb", utils.Round(float64(v.Available)/1048576, 2))
				}
				if requestedMetrics["total_ram_mb"] {
					addMetric("total_ram_mb", utils.Round(float64(v.Total)/1048576, 2))
				}
				if requestedMetrics["ram_cached_mb"] {
					addMetric("ram_cached_mb", utils.Round(float64(v.Cached)/1048576, 2))
				}
				if requestedMetrics["ram_buffers_mb"] {
					addMetric("ram_buffers_mb", utils.Round(float64(v.Buffers)/1048576, 2))
				}
			}
		}()
	}

	// Swap metrics
	if groups.swap {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if s, err := mem.SwapMemory(); err == nil {
				if requestedMetrics["swap_usage_percent"] {
					addMetric("swap_usage_percent", utils.Round(s.UsedPercent, 2))
				}
				if requestedMetrics["swap_total_mb"] {
					addMetric("swap_total_mb", utils.Round(float64(s.Total)/1048576, 2))
				}
				if requestedMetrics["swap_used_mb"] {
					addMetric("swap_used_mb", utils.Round(float64(s.Used)/1048576, 2))
				}
			}
		}()
	}

	// Disk usage metrics
	if groups.diskUsage {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if usage, err := disk.Usage("/"); err == nil {
				if requestedMetrics["disk_usage_percent"] {
					addMetric("disk_usage_percent", utils.Round(usage.UsedPercent, 2))
				}
				if requestedMetrics["available_disk_gb"] {
					addMetric("available_disk_gb", utils.Round(float64(usage.Free)/1073741824, 2))
				}
				if requestedMetrics["total_disk_gb"] {
					addMetric("total_disk_gb", utils.Round(float64(usage.Total)/1073741824, 2))
				}
				if requestedMetrics["inode_usage_percent"] {
					addMetric("inode_usage_percent", utils.Round(usage.InodesUsedPercent, 2))
				}
			}
		}()
	}

	// Disk I/O metrics
	if groups.diskIO {
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
				
				// Update stored values atomically
				atomic.StoreUint64(&prevMetrics.diskReadBytes, totalRead)
				atomic.StoreUint64(&prevMetrics.diskWriteBytes, totalWrite)
			}
		}()
	}

	// Network metrics
	if groups.network {
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
				
				// Update stored values atomically
				atomic.StoreUint64(&prevMetrics.netBytesSent, c.BytesSent)
				atomic.StoreUint64(&prevMetrics.netBytesRecv, c.BytesRecv)
			}
		}()
	}

	// Network connections
	if groups.netConn {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if conns, err := net.Connections("all"); err == nil {
				addMetric("active_connections", len(conns))
			}
		}()
	}

	// Process count
	if groups.processCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if procs, err := process.Processes(); err == nil {
				addMetric("process_count", len(procs))
			}
		}()
	}

	// Host info
	if groups.hostInfo {
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

	// Update timestamp atomically
	atomic.StoreInt64(&prevMetrics.timestamp, nowNano)

	return metrics
}