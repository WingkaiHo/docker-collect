// network
package main

import (
	"github.com/fsouza/go-dockerclient"
)

type NetRaw struct {
	Time      int64
	RxBytes   uint64
	RxDropped uint64
	RxErrors  uint64
	RxPackets uint64
	TxBytes   uint64
	TxDropped uint64
	TxErrors  uint64
	TxPackets uint64
}

type NetStats struct {
	RxBytes   float64
	RxDropped float64
	RxErrors  float64
	RxPackets float64
	TxBytes   float64
	TxDropped float64
	TxErrors  float64
	TxPackets float64
}

type NetService struct {
	NetworkStatPerContainer map[string]NetRaw
}

func createNetRaw(time int64, stats docker.Stats) NetRaw {
	var rxBytes, txBytes, rxDropped, txDropped, rxErrors, txErrors uint64

	for _, v := range stats.Networks {
		rxBytes += v.RxBytes
		txBytes += v.TxBytes
		rxDropped += v.RxDropped
		txDropped += v.TxDropped
		rxErrors += v.RxErrors
		txErrors += v.TxErrors
	}
	return NetRaw{
		Time:      time,
		RxBytes:   rxBytes,
		RxDropped: rxDropped,
		RxErrors:  rxErrors,
		TxBytes:   txBytes,
		TxDropped: txDropped,
		TxErrors:  txErrors,
	}
}

func (n *NetService) GetContainerNetworkStats(id string, stats docker.Stats, currTime int64) NetStats {
	var netStats NetStats
	newStats := createNetRaw(currTime, stats)
	oldStats, exits := n.NetworkStatPerContainer[id]

	duration := currTime - oldStats.Time

	if exits && duration > 0 {
		netStats.RxBytes = calculatePerSecond(duration, oldStats.RxBytes, newStats.RxBytes)
		netStats.RxDropped = calculatePerSecond(duration, oldStats.RxDropped, newStats.RxDropped)
		netStats.RxErrors = calculatePerSecond(duration, oldStats.RxErrors, newStats.RxErrors)
		netStats.TxBytes = calculatePerSecond(duration, oldStats.TxBytes, newStats.TxBytes)
		netStats.TxDropped = calculatePerSecond(duration, oldStats.TxDropped, newStats.TxDropped)
		netStats.TxErrors = calculatePerSecond(duration, oldStats.TxErrors, newStats.TxErrors)
	}

	n.NetworkStatPerContainer[id] = newStats
	return netStats
}
