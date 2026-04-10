package bootstrap

// HeartbeatRequest is sent by agent to provisioner every heartbeat_interval_sec.
type HeartbeatRequest struct {
	ContainerID       string  `json:"container_id"`
	AgentVersion      string  `json:"agent_version"`
	UptimeSec         int64   `json:"uptime_sec"`
	CPUPct            float64 `json:"cpu_pct"`
	MemUsedMB         uint64  `json:"mem_used_mb"`
	MemTotalMB        uint64  `json:"mem_total_mb"`
	DiskUsedGB        float64 `json:"disk_used_gb"`
	DiskTotalGB       float64 `json:"disk_total_gb"`
	Load1m            float64 `json:"load_1m"`
	WGHandshakeAgeSec int64   `json:"wg_handshake_age_sec"`
	WGEndpoint        string  `json:"wg_endpoint"`
}

// ConfigResponse is returned by provisioner on both POST /heartbeat and GET /config/{id}.
type ConfigResponse struct {
	WG                   WGConfig    `json:"wg"`
	Agent                AgentConfig `json:"agent"`
	HeartbeatIntervalSec int         `json:"heartbeat_interval_sec"`
}

type WGConfig struct {
	RouterPublicKey string `json:"router_public_key"`
	RouterEndpoint  string `json:"router_endpoint"`
	AssignedIP      string `json:"assigned_ip"`
}

type AgentConfig struct {
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
	SHA256        string `json:"sha256"`
}
