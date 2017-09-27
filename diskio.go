// diskio
package main

import (
	"strings"

	"github.com/fsouza/go-dockerclient"
)

type BlkioStats struct {
	reads  float64
	writes float64
}

type BlkioRaw struct {
	Time   int64
	reads  uint64
	writes uint64
}

type BLkioService struct {
	BlkioSTatsPerContainer map[string]BlkioRaw
}

func createBlkioRaw(time int64, stats docker.Stats) BlkioRaw {
	var blkRead, blkWrite uint64

	for _, bioEntry := range stats.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}

	return BlkioRaw{
		Time:   time,
		reads:  blkRead,
		writes: blkWrite}
}

func (io *BLkioService) GetContainerBlkioStats(id string, stats docker.Stats, currTime int64) BlkioStats {
	var blkioStats BlkioStats
	newBlkioStats := createBlkioRaw(currTime, stats)
	oldBlkioStats, exist := io.BlkioSTatsPerContainer[id]
	duration := currTime - oldBlkioStats.Time

	if exist && duration > 0 {
		blkioStats.reads = calculatePerSecond(duration, oldBlkioStats.reads, newBlkioStats.reads)
		blkioStats.reads = calculatePerSecond(duration, oldBlkioStats.writes, newBlkioStats.writes)
	}

	io.BlkioSTatsPerContainer[id] = newBlkioStats

	return blkioStats
}
