package config

// Config stores runtime settings for StackLoom.
type Config struct {
	Port             int      `json:"port"`
	Host             string   `json:"host"`
	ComposeFiles     []string `json:"compose_files"`
	ActiveProfiles   []string `json:"active_profiles"`
	DockerHost       string   `json:"docker_host"`
	ProbeHostSockets bool     `json:"probe_host_sockets"`
	ProjectName      string   `json:"project_name"`
	OpenBrowser      bool     `json:"open_browser"`
}

// DefaultConfig provides standard operational defaults.
func DefaultConfig() *Config {
	return &Config{
		Port:             9095,
		Host:             "127.0.0.1",
		ComposeFiles:     []string{"docker-compose.yml"},
		ActiveProfiles:   []string{},
		DockerHost:       "",
		ProbeHostSockets: true,
		ProjectName:      "",
		OpenBrowser:      false,
	}
}
