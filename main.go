// docker-collectd project main.go
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/marpaia/graphite-golang"
)

var now int64
var hostname string
var timeout = time.Duration(10) * time.Second
var connectd bool

type Stat struct {
	Container docker.APIContainers
	Stats     docker.Stats
}

func createDockerClient(dockerHost string) *docker.Client {
	client, err := docker.NewClient(dockerHost)
	if err != nil {
		glog.Errorf("Fail to create docker client: %s\n", err.Error())
	}

	return client
}

func getContainersCPUMetrics(events []Stat) []graphite.Metric {

	var cpu float64
	var m graphite.Metric

	metrics := make([]graphite.Metric, 0, len(events))
	for _, event := range events {
		cpu = CalculateCPUPercentUnix(event.Stats)
		m.Timestamp = now
		m.Value = fmt.Sprintf("%.4f", cpu)
		m.Name = fmt.Sprintf("%s.%s.cpu.total_percent", hostname, event.Container.ID)
		metrics = append(metrics, m)
	}

	return metrics
}

func getContainersMemoryMetrics(events []Stat) []graphite.Metric {
	var m graphite.Metric
	var percent float64

	metrics := make([]graphite.Metric, 0, (len(events) * 3))
	for _, event := range events {
		m.Timestamp = now
		// memory used
		m.Value = fmt.Sprintf("%d", event.Stats.MemoryStats.Usage)
		m.Name = fmt.Sprintf("%s.%s.memory.usage", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// memory limit
		m.Value = fmt.Sprintf("%d", event.Stats.MemoryStats.Limit)
		m.Name = fmt.Sprintf("%s.%s.memory.limit", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// memory percent
		percent = 0.0
		if event.Stats.MemoryStats.Limit != 0 {
			percent = float64(event.Stats.MemoryStats.Usage) / float64(event.Stats.MemoryStats.Limit) * 100.0
		}
		m.Value = fmt.Sprintf("%.4f", percent)
		m.Name = fmt.Sprintf("%s.%s.memory.usage_percent", hostname, event.Container.ID)
		metrics = append(metrics, m)
	}

	return metrics
}

func getContainersNetworkMetrics(netService *NetService, events []Stat) []graphite.Metric {
	var m graphite.Metric

	metrics := make([]graphite.Metric, 0, (len(events) * 6))
	for _, event := range events {
		netStats := netService.GetContainerNetworkStats(event.Container.ID, event.Stats, now)

		m.Timestamp = now
		// rx bytes per second
		m.Value = fmt.Sprintf("%.2f", netStats.RxBytes)
		m.Name = fmt.Sprintf("%s.%s.network.rx_bytes", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// tx bytes per second
		m.Value = fmt.Sprintf("%.2f", netStats.TxBytes)
		m.Name = fmt.Sprintf("%s.%s.network.tx_bytes", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// rx dropped per second
		m.Value = fmt.Sprintf("%.2f", netStats.RxDropped)
		m.Name = fmt.Sprintf("%s.%s.network.rx_dropped", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// tx dropped per second
		m.Value = fmt.Sprintf("%.2f", netStats.TxDropped)
		m.Name = fmt.Sprintf("%s.%s.network.tx_dropped", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// rx error per second
		m.Value = fmt.Sprintf("%.2f", netStats.RxErrors)
		m.Name = fmt.Sprintf("%s.%s.network.rx_errors", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// tx error per second
		m.Value = fmt.Sprintf("%.2f", netStats.TxErrors)
		m.Name = fmt.Sprintf("%s.%s.network.tx_errors", hostname, event.Container.ID)
		metrics = append(metrics, m)
	}

	return metrics
}

func getContainersBlkioMetrics(blkioService *BLkioService, events []Stat) []graphite.Metric {
	var m graphite.Metric

	metrics := make([]graphite.Metric, 0, (len(events) * 6))
	for _, event := range events {
		blkioStats := blkioService.GetContainerBlkioStats(event.Container.ID, event.Stats, now)
		m.Timestamp = now
		// read bytes per second
		m.Value = fmt.Sprintf("%.2f", blkioStats.reads)
		m.Name = fmt.Sprintf("%s.%s.disk.read", hostname, event.Container.ID)
		metrics = append(metrics, m)

		// write bytes per second
		m.Value = fmt.Sprintf("%.2f", blkioStats.writes)
		m.Name = fmt.Sprintf("%s.%s.disk.write", hostname, event.Container.ID)
		metrics = append(metrics, m)

	}

	return metrics
}

func createGraphiteClient(strGraphiteUrl, strPerfix string) *graphite.Graphite {
	ip, strPort, err := net.SplitHostPort(strGraphiteUrl)
	if err != nil {
		glog.Errorf("interval graphite url %s\n", strGraphiteUrl)
		return nil
	}

	port, errAtoi := strconv.Atoi(strPort)
	if errAtoi != nil {
		glog.Errorf("Invalid graphite port %s\n", strPort)
	}

	graphiteClient, graphiteErr := graphite.GraphiteFactory("tcp", ip, port, strPerfix)
	if graphiteErr != nil {
		glog.Errorf("GraphiteFactory fail %s\n", graphiteErr.Error())
	}
	return graphiteClient
}

func reconnectGraphite(graphiteClient *graphite.Graphite) bool {
	graphiteClient.Disconnect()
	connectd = false
	err := graphiteClient.Connect()
	if err != nil {
		glog.Errorf("Reconnecting graphite(%s:%d) fail.\n", graphiteClient.Host, graphiteClient.Port)
	} else {
		connectd = true
		glog.Infof("Reconnecting graphite(%s:%d) successful.\n", graphiteClient.Host, graphiteClient.Port)
		return true
	}
	return false
}

func sendMetrics2Graphite(metrics []graphite.Metric, graphiteClient *graphite.Graphite) {
	glog.Infof("Sending %d events to graphite", len(metrics))

	if connectd == false {
		successful := reconnectGraphite(graphiteClient)
		if successful == false {
			return
		}
	}

	err := graphiteClient.SendMetrics(metrics)
	if err != nil {
		glog.Error("There were errors sending events to Graphite, reconecting")
		reconnectGraphite(graphiteClient)
	}
}

func fetchContainerStats(client *docker.Client) ([]Stat, error) {
	var wg sync.WaitGroup
	containers, err := client.ListContainers(docker.ListContainersOptions{All: false})
	if err != nil {
		glog.Infof("ListContainers fail: %s", err.Error())
		return nil, err
	}

	containersList := make([]Stat, 0, len(containers))
	statsQueue := make(chan Stat, 1)
	wg.Add(len(containers))

	for _, container := range containers {
		go func(container docker.APIContainers) {
			defer wg.Done()
			statsQueue <- exportContainerStats(&container, client)
		}(container)
	}

	go func() {
		wg.Wait()
		close(statsQueue)
	}()

	// This will break after the queue has been drained and queue is closed.
	for stat := range statsQueue {
		glog.Infoln(stat.Container.ID)
		// If names is empty, there is not data inside
		if len(stat.Container.Names) != 0 {
			containersList = append(containersList, stat)
		}
	}

	return containersList, err
}

func exportContainerStats(container *docker.APIContainers, client *docker.Client) Stat {
	//	var err error
	var wg sync.WaitGroup
	var event Stat

	glog.Infoln("Collect container stats: ", container.ID)
	errC := make(chan error, 1)
	statsC := make(chan *docker.Stats)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errC <- client.Stats(docker.StatsOptions{ID: container.ID, Stats: statsC, Stream: false, Timeout: timeout})
		close(errC)
	}()

	go func() {
		defer wg.Done()
		stats := <-statsC
		err := <-errC
		if stats != nil && err == nil {
			event.Stats = *stats
			event.Container = *container
		} else if err == nil && stats == nil {
			glog.Warningf("Container stopped when recovering stats: %s\n", container.ID)
		} else {
			glog.Errorf("An error occurred while getting docker stats: %s\n", err.Error())
		}
	}()

	wg.Wait()
	return event
}

func main() {
	// Init param from command line
	dockerHost := flag.String("docker_host", "unix:///var/run/docker.sock", "Expose metrics from docker host: --docker_host \"tcp://ip:port\"  ")
	interval := flag.Int64("interval", 30, "Interval to collect docker containers info.")
	strGraphiteUrl := flag.String("graphite_url", "", "The graphite_web host which metrics to save to: --graphite_url ip:2003")
	hostname, _ = os.Hostname()
	glog.Infoln("hostname:", hostname)
	strHostname := flag.String("hostname", hostname, "The hostname flag of host, default is hostname of system")
	flag.Parse()

	glog.Infof("param docker_host= %s\n", *dockerHost)
	glog.Infof("param interval = %d\n", *interval)
	glog.Infof("param graphite_url = %s\n,", *strGraphiteUrl)

	hostname = *strHostname
	hostname = strings.Replace(hostname, ".", "_", -1)
	glog.Infof("param hostname = %s\n,", hostname)

	client := createDockerClient(*dockerHost)
	if client == nil {
		glog.Errorln("CreateDockerClient fail and program exit.")
		return
	}

	graphiteClient := createGraphiteClient(*strGraphiteUrl, "docker")
	if graphiteClient == nil {
		glog.Errorf("CreateGraphiteClient fail and program exit.\n")
		return
	}
	connectd = true
	netService := &NetService{
		NetworkStatPerContainer: make(map[string]NetRaw),
	}

	blkioService := &BLkioService{
		BlkioSTatsPerContainer: make(map[string]BlkioRaw),
	}

	ticker := time.NewTicker(time.Second * time.Duration(*interval))
	for {
		now = time.Now().Unix()
		events, err := fetchContainerStats(client)

		if err == nil {
			glog.Infoln("Get containers cpu usage metrics.")
			metricsCPU := getContainersCPUMetrics(events)
			sendMetrics2Graphite(metricsCPU, graphiteClient)

			glog.Infoln("Get containers memory stats metrics.")
			metricsMemory := getContainersMemoryMetrics(events)
			sendMetrics2Graphite(metricsMemory, graphiteClient)

			glog.Infoln("Get container network stats metrics.")
			metricsNetwork := getContainersNetworkMetrics(netService, events)
			sendMetrics2Graphite(metricsNetwork, graphiteClient)

			glog.Infoln("Get container disk stats metrics")
			metricsDisk := getContainersBlkioMetrics(blkioService, events)
			sendMetrics2Graphite(metricsDisk, graphiteClient)

		}
		<-ticker.C
	}
}
