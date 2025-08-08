package appinsights

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	// cpuIdleFieldIndex is the index of the idle field in /proc/stat CPU line (0-indexed)
	cpuIdleFieldIndex = 4
	// diskSectorSize is the standard disk sector size in bytes
	diskSectorSize = 512
)

// SystemMetricsCollector collects system-level performance metrics
type SystemMetricsCollector struct {
	lastCPUStats *cpuStats
}

// cpuStats holds CPU statistics for calculation
type cpuStats struct {
	idle  uint64
	total uint64
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector() *SystemMetricsCollector {
	return &SystemMetricsCollector{}
}

// Name returns the collector name
func (s *SystemMetricsCollector) Name() string {
	return "System Metrics"
}

// Collect gathers system-level metrics
func (s *SystemMetricsCollector) Collect(client TelemetryClient) {
	s.collectCPUMetrics(client)
	s.collectMemoryMetrics(client)
	s.collectDiskMetrics(client)
}

// collectCPUMetrics collects CPU usage metrics
func (s *SystemMetricsCollector) collectCPUMetrics(client TelemetryClient) {
	if runtime.GOOS == "linux" {
		s.collectLinuxCPUMetrics(client)
	} else {
		// For non-Linux systems, collect basic CPU count
		client.TrackMetric("system.cpu.count", float64(runtime.NumCPU()))
	}
}

// collectLinuxCPUMetrics collects detailed CPU metrics on Linux
func (s *SystemMetricsCollector) collectLinuxCPUMetrics(client TelemetryClient) {
	stats, err := s.readCPUStats()
	if err != nil {
		// Fallback to basic metrics
		client.TrackMetric("system.cpu.count", float64(runtime.NumCPU()))
		return
	}
	
	client.TrackMetric("system.cpu.count", float64(runtime.NumCPU()))
	
	if s.lastCPUStats != nil {
		// Calculate CPU usage percentage
		totalDiff := stats.total - s.lastCPUStats.total
		idleDiff := stats.idle - s.lastCPUStats.idle
		
		if totalDiff > 0 {
			cpuUsage := 100.0 * (1.0 - float64(idleDiff)/float64(totalDiff))
			client.TrackMetric("system.cpu.usage_percent", cpuUsage)
		}
	}
	
	s.lastCPUStats = stats
}

// readCPUStats reads CPU statistics from /proc/stat
func (s *SystemMetricsCollector) readCPUStats() (*cpuStats, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}
			
			var total, idle uint64
			for i := 1; i < len(fields); i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					continue
				}
				total += val
				if i == cpuIdleFieldIndex { // idle is the 4th field (0-indexed)
					idle = val
				}
			}
			
			return &cpuStats{idle: idle, total: total}, nil
		}
	}
	
	return nil, fmt.Errorf("cpu stats not found")
}

// collectMemoryMetrics collects memory usage metrics
func (s *SystemMetricsCollector) collectMemoryMetrics(client TelemetryClient) {
	if runtime.GOOS == "linux" {
		s.collectLinuxMemoryMetrics(client)
	}
	// For other OS, we could add platform-specific implementations
}

// collectLinuxMemoryMetrics collects memory metrics on Linux
func (s *SystemMetricsCollector) collectLinuxMemoryMetrics(client TelemetryClient) {
	memInfo, err := s.readMemInfo()
	if err != nil {
		return
	}
	
	for key, value := range memInfo {
		switch key {
		case "MemTotal":
			client.TrackMetric("system.memory.total", value)
		case "MemFree":
			client.TrackMetric("system.memory.free", value)
		case "MemAvailable":
			client.TrackMetric("system.memory.available", value)
		case "Buffers":
			client.TrackMetric("system.memory.buffers", value)
		case "Cached":
			client.TrackMetric("system.memory.cached", value)
		case "SwapTotal":
			client.TrackMetric("system.memory.swap_total", value)
		case "SwapFree":
			client.TrackMetric("system.memory.swap_free", value)
		}
	}
	
	// Calculate memory usage percentage
	if total, exists := memInfo["MemTotal"]; exists {
		if available, exists := memInfo["MemAvailable"]; exists {
			usedPercent := 100.0 * (1.0 - available/total)
			client.TrackMetric("system.memory.usage_percent", usedPercent)
		} else if free, exists := memInfo["MemFree"]; exists {
			usedPercent := 100.0 * (1.0 - free/total)
			client.TrackMetric("system.memory.usage_percent", usedPercent)
		}
	}
}

// readMemInfo reads memory information from /proc/meminfo
func (s *SystemMetricsCollector) readMemInfo() (map[string]float64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	memInfo := make(map[string]float64)
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		
		key := strings.TrimSuffix(fields[0], ":")
		if key == "" {
			continue
		}
		valueStr := fields[1]
		
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}
		
		// Convert from kB to bytes
		if len(fields) > 2 && fields[2] == "kB" {
			value *= 1024
		}
		
		memInfo[key] = value
	}
	
	return memInfo, scanner.Err()
}

// collectDiskMetrics collects disk usage metrics
func (s *SystemMetricsCollector) collectDiskMetrics(client TelemetryClient) {
	if runtime.GOOS == "linux" {
		s.collectLinuxDiskMetrics(client)
	}
	// For other OS, we could add platform-specific implementations
}

// collectLinuxDiskMetrics collects disk metrics on Linux
func (s *SystemMetricsCollector) collectLinuxDiskMetrics(client TelemetryClient) {
	// Collect disk I/O statistics from /proc/diskstats
	diskStats, err := s.readDiskStats()
	if err != nil {
		return
	}
	
	for device, stats := range diskStats {
		prefix := fmt.Sprintf("system.disk.%s", device)
		client.TrackMetric(prefix+".reads", stats["reads"])
		client.TrackMetric(prefix+".writes", stats["writes"])
		client.TrackMetric(prefix+".read_bytes", stats["read_bytes"])
		client.TrackMetric(prefix+".write_bytes", stats["write_bytes"])
	}
}

// readDiskStats reads disk statistics from /proc/diskstats
func (s *SystemMetricsCollector) readDiskStats() (map[string]map[string]float64, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	diskStats := make(map[string]map[string]float64)
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		
		device := fields[2]
		
		// Skip loop devices and other virtual devices
		if strings.HasPrefix(device, "loop") ||
			strings.HasPrefix(device, "ram") ||
			strings.HasPrefix(device, "dm-") {
			continue
		}
		
		// Parse relevant fields
		reads, _ := strconv.ParseFloat(fields[3], 64)
		readSectors, _ := strconv.ParseFloat(fields[5], 64)
		writes, _ := strconv.ParseFloat(fields[7], 64)
		writeSectors, _ := strconv.ParseFloat(fields[9], 64)
		
		// Convert sectors to bytes (assuming 512 bytes per sector)
		readBytes := readSectors * diskSectorSize
		writeBytes := writeSectors * diskSectorSize
		
		diskStats[device] = map[string]float64{
			"reads":       reads,
			"writes":      writes,
			"read_bytes":  readBytes,
			"write_bytes": writeBytes,
		}
	}
	
	return diskStats, scanner.Err()
}