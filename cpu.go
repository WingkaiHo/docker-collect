// cpu.go
package main

import (
	"github.com/fsouza/go-dockerclient"
)

func CalculateCPUPercentUnix(stats docker.Stats) float64 {
	var (
		previousCPU    = stats.PreCPUStats.CPUUsage.TotalUsage
		previousSystem = stats.PreCPUStats.SystemCPUUsage
		cpuPercent     = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(stats.CPUStats.SystemCPUUsage) - float64(previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}
