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
	timestamp      int64
}

var prevMetrics previousMetrics

// Cache for collected metrics
var (
	cachedMetrics   atomic.Value
	cacheTimestamp  atomic.Int64
	cacheTTL        int64 // Store as nanoseconds for faster comparison
	collectionMutex sync.Mutex
)

// Pre-parsed metric lookup
var requestedMetrics map[string]bool

// Pre-parsed metric groups
type metricGroups struct {
	cpuUsage     bool
	cpuInfo      bool
	memory       bool
	swap         bool
	diskUsage    bool
	diskIO       bool
	network      bool
	netConn      bool
	processCount bool
	hostInfo     bool
}

var groups metricGroups

// Constants for conversions (pre-calculated)
const (
	bytesToMB = 1.0 / 1048576.0
	bytesToGB = 1.0 / 1073741824.0
)

func Init(c *config.Config) {
	cfg = c
	atomic.StoreInt64(&prevMetrics.timestamp, time.Now().UnixNano())
	
	// Set cache TTL as nanoseconds for faster comparison
	if c.System.CacheTTL > 0 {
		cacheTTL = int64(c.System.CacheTTL * 1e9)
	} else {
		cacheTTL = 15 * 1e9
	}
	
	// Pre-parse requested metrics into a map (done once at startup)
	requestedMetrics = make(map[string]bool, len(c.System.Metrics))
	for _, metric := range c.System.Metrics {
		requestedMetrics[metric] = true
	}
	
	// Pre-parse metric groups
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
	
	cacheTimestamp.Store(0)
}

func CollectSystem() map[string]interface{} {
	// Fast path: return cached metrics if still valid
	cachedTime := cacheTimestamp.Load()
	nowNano := time.Now().UnixNano()
	
	if cachedTime > 0 && (nowNano-cachedTime) < cacheTTL {
		if cached := cachedMetrics.Load(); cached != nil {
			return cached.(map[string]interface{})
		}
	}

	// Slow path: need to collect new metrics
	collectionMutex.Lock()
	defer collectionMutex.Unlock()

	// Double-check cache after acquiring lock
	cachedTime = cacheTimestamp.Load()
	nowNano = time.Now().UnixNano()
	if cachedTime > 0 && (nowNano-cachedTime) < cacheTTL {
		if cached := cachedMetrics.Load(); cached != nil {
			return cached.(map[string]interface{})
		}
	}

	// Actually collect metrics
	metrics := doActualCollection(nowNano)

	// Update cache atomically
	cachedMetrics.Store(metrics)
	cacheTimestamp.Store(nowNano)

	return metrics
}

func doActualCollection(nowNano int64) map[string]interface{} {
	// Pre-allocate map with exact capacity based on requested metrics
	capacity := len(cfg.System.Metrics)
	metrics := make(map[string]interface{}, capacity)
	
	// Use a pool of result channels to avoid allocations
	type result struct {
		key   string
		value interface{}
	}
	resultChan := make(chan result, capacity)
	
	var wg sync.WaitGroup
	
	// Read previous metrics atomically
	prevTimestamp := atomic.LoadInt64(&prevMetrics.timestamp)
	timeDelta := float64(nowNano-prevTimestamp) * 1e-9
	prevDiskRead := atomic.LoadUint64(&prevMetrics.diskReadBytes)
	prevDiskWrite := atomic.LoadUint64(&prevMetrics.diskWriteBytes)
	prevNetSent := atomic.LoadUint64(&prevMetrics.netBytesSent)
	prevNetRecv := atomic.LoadUint64(&prevMetrics.netBytesRecv)

	// Helper to send metrics to channel
	send := func(key string, value interface{}) {
		resultChan <- result{key, value}
	}

	// CPU metrics
	if groups.cpuUsage {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			wantPercent := requestedMetrics["cpu_usage_percent"]
			wantPerCore := requestedMetrics["cpu_usage_per_core"]
			
			if wantPercent && wantPerCore {
				// Collect per-core and calculate average
				if percent, err := cpu.Percent(100*time.Millisecond, true); err == nil && len(percent) > 0 {
					var total float64
					coreMetrics := make([]float64, len(percent))
					for i, p := range percent {
						rounded := utils.Round(p, 2)
						coreMetrics[i] = rounded
						total += rounded
					}
					send("cpu_usage_per_core", coreMetrics)
					send("cpu_usage_percent", utils.Round(total/float64(len(percent)), 2))
				}
			} else if wantPercent {
				if percent, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(percent) > 0 {
					send("cpu_usage_percent", utils.Round(percent[0], 2))
				}
			} else if wantPerCore {
				if percent, err := cpu.Percent(100*time.Millisecond, true); err == nil {
					coreMetrics := make([]float64, len(percent))
					for i, p := range percent {
						coreMetrics[i] = utils.Round(p, 2)
					}
					send("cpu_usage_per_core", coreMetrics)
				}
			}
		}()
	}

	// CPU info and load
	if groups.cpuInfo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			if requestedMetrics["cpu_count"] {
				if count, err := cpu.Counts(true); err == nil {
					send("cpu_count", count)
				}
			}
			if requestedMetrics["cpu_count_physical"] {
				if count, err := cpu.Counts(false); err == nil {
					send("cpu_count_physical", count)
				}
			}
			
			needLoad := requestedMetrics["cpu_load_1min"] || requestedMetrics["cpu_load_5min"] || requestedMetrics["cpu_load_15min"]
			if needLoad {
				if avg, err := load.Avg(); err == nil {
					if requestedMetrics["cpu_load_1min"] {
						send("cpu_load_1min", utils.Round(avg.Load1, 2))
					}
					if requestedMetrics["cpu_load_5min"] {
						send("cpu_load_5min", utils.Round(avg.Load5, 2))
					}
					if requestedMetrics["cpu_load_15min"] {
						send("cpu_load_15min", utils.Round(avg.Load15, 2))
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
					send("ram_usage_percent", utils.Round(v.UsedPercent, 2))
				}
				if requestedMetrics["available_ram_mb"] {
					send("available_ram_mb", utils.Round(float64(v.Available)*bytesToMB, 2))
				}
				if requestedMetrics["total_ram_mb"] {
					send("total_ram_mb", utils.Round(float64(v.Total)*bytesToMB, 2))
				}
				if requestedMetrics["ram_cached_mb"] {
					send("ram_cached_mb", utils.Round(float64(v.Cached)*bytesToMB, 2))
				}
				if requestedMetrics["ram_buffers_mb"] {
					send("ram_buffers_mb", utils.Round(float64(v.Buffers)*bytesToMB, 2))
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
					send("swap_usage_percent", utils.Round(s.UsedPercent, 2))
				}
				if requestedMetrics["swap_total_mb"] {
					send("swap_total_mb", utils.Round(float64(s.Total)*bytesToMB, 2))
				}
				if requestedMetrics["swap_used_mb"] {
					send("swap_used_mb", utils.Round(float64(s.Used)*bytesToMB, 2))
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
					send("disk_usage_percent", utils.Round(usage.UsedPercent, 2))
				}
				if requestedMetrics["available_disk_gb"] {
					send("available_disk_gb", utils.Round(float64(usage.Free)*bytesToGB, 2))
				}
				if requestedMetrics["total_disk_gb"] {
					send("total_disk_gb", utils.Round(float64(usage.Total)*bytesToGB, 2))
				}
				if requestedMetrics["inode_usage_percent"] {
					send("inode_usage_percent", utils.Round(usage.InodesUsedPercent, 2))
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
					send("disk_read_bytes", totalRead)
				}
				if requestedMetrics["disk_write_bytes"] {
					send("disk_write_bytes", totalWrite)
				}
				if requestedMetrics["disk_read_count"] {
					send("disk_read_count", totalReads)
				}
				if requestedMetrics["disk_write_count"] {
					send("disk_write_count", totalWrites)
				}
				
				if requestedMetrics["disk_read_bytes_per_sec"] && prevDiskRead > 0 && timeDelta > 0 {
					bytesPerSec := float64(totalRead-prevDiskRead) / timeDelta
					send("disk_read_bytes_per_sec", utils.Round(bytesPerSec, 2))
				}
				if requestedMetrics["disk_write_bytes_per_sec"] && prevDiskWrite > 0 && timeDelta > 0 {
					bytesPerSec := float64(totalWrite-prevDiskWrite) / timeDelta
					send("disk_write_bytes_per_sec", utils.Round(bytesPerSec, 2))
				}
				
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
					send("network_bytes_sent", c.BytesSent)
				}
				if requestedMetrics["network_bytes_recv"] {
					send("network_bytes_recv", c.BytesRecv)
				}
				if requestedMetrics["network_packets_sent"] {
					send("network_packets_sent", c.PacketsSent)
				}
				if requestedMetrics["network_packets_recv"] {
					send("network_packets_recv", c.PacketsRecv)
				}
				if requestedMetrics["network_errors_in"] {
					send("network_errors_in", c.Errin)
				}
				if requestedMetrics["network_errors_out"] {
					send("network_errors_out", c.Errout)
				}
				
				if requestedMetrics["network_bytes_sent_per_sec"] && prevNetSent > 0 && timeDelta > 0 {
					bytesPerSec := float64(c.BytesSent-prevNetSent) / timeDelta
					send("network_bytes_sent_per_sec", utils.Round(bytesPerSec, 2))
				}
				if requestedMetrics["network_bytes_recv_per_sec"] && prevNetRecv > 0 && timeDelta > 0 {
					bytesPerSec := float64(c.BytesRecv-prevNetRecv) / timeDelta
					send("network_bytes_recv_per_sec", utils.Round(bytesPerSec, 2))
				}
				
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
				send("active_connections", len(conns))
			}
		}()
	}

	// Process count
	if groups.processCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if procs, err := process.Processes(); err == nil {
				send("process_count", len(procs))
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
					send("system_uptime_seconds", float64(info.Uptime))
				}
				if requestedMetrics["boot_time_unix"] {
					send("boot_time_unix", info.BootTime)
				}
				if requestedMetrics["os_platform"] {
					send("os_platform", info.Platform)
				}
				if requestedMetrics["os_version"] {
					send("os_version", info.PlatformVersion)
				}
				if requestedMetrics["hostname"] {
					send("hostname", info.Hostname)
				}
				if requestedMetrics["kernel_version"] {
					send("kernel_version", info.KernelVersion)
				}
			}
		}()
	}

	// Collect results from channel in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results from channel (no mutex needed)
	for r := range resultChan {
		metrics[r.key] = r.value
	}

	// Update timestamp atomically
	atomic.StoreInt64(&prevMetrics.timestamp, nowNano)

	return metrics
}