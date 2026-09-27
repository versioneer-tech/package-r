package metrics

import "github.com/prometheus/client_golang/prometheus"

type vfsCollector struct {
	provider          VFSStatsProvider
	cacheBytes        *prometheus.Desc
	uploadsQueued     *prometheus.Desc
	uploadsActive     *prometheus.Desc
	cacheErroredFiles *prometheus.Desc
	cacheOutOfSpace   *prometheus.Desc
}

func newVFSCollector(provider VFSStatsProvider) *vfsCollector {
	return &vfsCollector{
		provider: provider,
		cacheBytes: prometheus.NewDesc(
			"package_r_vfs_cache_bytes",
			"Current number of bytes used by the rclone VFS write cache.",
			nil,
			nil,
		),
		uploadsQueued: prometheus.NewDesc(
			"package_r_vfs_uploads_queued",
			"Current number of rclone VFS uploads waiting to start.",
			nil,
			nil,
		),
		uploadsActive: prometheus.NewDesc(
			"package_r_vfs_uploads_active",
			"Current number of rclone VFS uploads in progress.",
			nil,
			nil,
		),
		cacheErroredFiles: prometheus.NewDesc(
			"package_r_vfs_cache_errored_files",
			"Current number of errored files in the rclone VFS write cache.",
			nil,
			nil,
		),
		cacheOutOfSpace: prometheus.NewDesc(
			"package_r_vfs_cache_out_of_space",
			"Whether any rclone VFS write cache is out of space.",
			nil,
			nil,
		),
	}
}

func (c *vfsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cacheBytes
	ch <- c.uploadsQueued
	ch <- c.uploadsActive
	ch <- c.cacheErroredFiles
	ch <- c.cacheOutOfSpace
}

func (c *vfsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.provider.Stats()
	ch <- prometheus.MustNewConstMetric(c.cacheBytes, prometheus.GaugeValue, float64(stats.CacheBytes))
	ch <- prometheus.MustNewConstMetric(c.uploadsQueued, prometheus.GaugeValue, float64(stats.UploadsQueued))
	ch <- prometheus.MustNewConstMetric(c.uploadsActive, prometheus.GaugeValue, float64(stats.UploadsInProgress))
	ch <- prometheus.MustNewConstMetric(c.cacheErroredFiles, prometheus.GaugeValue, float64(stats.ErroredFiles))
	ch <- prometheus.MustNewConstMetric(c.cacheOutOfSpace, prometheus.GaugeValue, boolFloat(stats.OutOfSpace))
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
