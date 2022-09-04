package libdocker

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/ethereum/hive/internal/libdocker/prom"
	"github.com/ethereum/hive/internal/libhive"
)

type Metrics struct {
	cb *ContainerBackend

	hivePromDockerNetID string

	grafana    *libhive.ContainerInfo
	prometheus *libhive.ContainerInfo
}

func InitMetrics(ctx context.Context, cb *ContainerBackend) (*Metrics, error) {
	var id [10]byte
	rand.Read(id[:])
	netID, err := cb.CreateNetwork(fmt.Sprintf("hive-prom-%x", id[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to set up prometheus network: %w", err)
	}

	promOpts := libhive.ContainerOptions{CheckLive: 9090, HostPorts: map[string][]string{"9090/tcp": {"9090"}}}
	// create prometheus
	promID, err := cb.CreateContainer(ctx, hivepromTag, promOpts)
	if err != nil {
		_ = cb.RemoveNetwork(netID)
		return nil, fmt.Errorf("failed to create prometheus container: %w", err)
	}
	promOpts.LogFile = filepath.Join(cb.config.Inventory.BaseDir, "workspace", "logs", fmt.Sprintf("prometheus-%s.log", promID))
	promContainer, err := cb.StartContainer(ctx, promID, promOpts)
	if err != nil {
		_ = cb.DeleteContainer(promID)
		return nil, fmt.Errorf("failed to start prometheus container %s: %w", promID, err)
	}
	if err := cb.ConnectContainer(promID, netID); err != nil {
		_ = cb.DeleteContainer(promID)
		return nil, fmt.Errorf("failed to connect prometheus container %s to its own network: %w", promID, err)
	}

	// create grafana
	//cb.CreateContainer()
	//cb.StartContainer()

	return &Metrics{
		cb:                  cb,
		hivePromDockerNetID: netID,
		//grafana:             grafContainer,
		prometheus: promContainer,
	}, nil
}

type scrapeTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func packageScrapeTargets(filePath string, targets []scrapeTarget) (io.Reader, error) {
	var fileContent bytes.Buffer
	if err := json.NewEncoder(&fileContent).Encode(targets); err != nil {
		return nil, fmt.Errorf("failed to encode scrape target config: %w", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: filePath, Mode: 0777, Size: int64(fileContent.Len())}); err != nil {
		return nil, fmt.Errorf("metrics tar writer header failed: %w", err)
	}
	if _, err := io.Copy(tw, &fileContent); err != nil {
		return nil, fmt.Errorf("metrics tar writer content failed: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("metrics tar writer close failed: %w", err)
	}
	return &buf, nil
}

// ScrapeMetrics adds the given container as a scrape target to the prometheus container managed by Hive.
func (m *Metrics) ScrapeMetrics(ctx context.Context, info *libhive.ContainerInfo, opts *libhive.MetricsOptions) error {
	// connect the container to the prometheus network so it's reachable
	if err := m.cb.ConnectContainer(info.ID, m.hivePromDockerNetID); err != nil {
		return fmt.Errorf("failed to connect container %s to hive metrics network %s", info.ID, m.hivePromDockerNetID)
	}

	// create a file for prometheus scrape-target-discovery
	filePath := fmt.Sprintf("/etc/prometheus/hive-metrics/hive-%s.json", info.ID)
	target := scrapeTarget{
		Targets: []string{fmt.Sprintf("%s:%d", info.IP, opts.Port)}, // TODO: is this the correct IP (diff network?)
		Labels:  opts.Labels,
	}
	pkg, err := packageScrapeTargets(filePath, []scrapeTarget{target})
	if err != nil {
		return fmt.Errorf("failed to make scrape target: %w", err)
	}

	// Upload the tar stream into the destination container.
	if err := m.cb.client.UploadToContainer(m.prometheus.ID, docker.UploadToContainerOptions{
		Context:     ctx,
		InputStream: pkg,
		Path:        "/",
	}); err != nil {
		return fmt.Errorf("failed to upload metrics scrape target config of %s to prometheus container %s", info.ID, m.prometheus.ID)
	}
	return nil
}

func (m *Metrics) StopScrapingMetrics(ctx context.Context, id string) error {
	// override with empty list to remove the scrape target
	filePath := fmt.Sprintf("/etc/prometheus/hive-metrics/hive-%s.json", id)
	pkg, err := packageScrapeTargets(filePath, []scrapeTarget{})
	if err != nil {
		return fmt.Errorf("failed to make scrape target: %w", err)
	}

	if m.cb.client.UploadToContainer(m.prometheus.ID, docker.UploadToContainerOptions{
		Context:     ctx,
		InputStream: pkg,
		Path:        "/",
	}); err != nil {
		return fmt.Errorf("failed to upload metrics scrape target cleaning of %s to prometheus container %s", id, m.prometheus.ID)
	}
	return nil
}

const hivepromTag = "hive/hiveprom"

// BuildMetrics builds the docker images for metrics usage: prometheus and grafana.
func (cb *ContainerBackend) BuildMetrics(ctx context.Context, b libhive.Builder) error {
	// TODO: build grafana image

	return b.BuildImage(ctx, hivepromTag, prom.PrometheusSource)
}

// InitMetrics starts the docker network and docker containers for metrics collection
func (cb *ContainerBackend) InitMetrics(ctx context.Context) error {
	m, err := InitMetrics(ctx, cb)
	if err != nil {
		return err
	}
	cb.metrics = m
	return nil
}
