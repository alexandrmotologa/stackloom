package compose

// Merger handles combining multiple Compose configurations and profile filtering.
type Merger struct{}

// NewMerger returns a new Merger instance.
func NewMerger() *Merger {
	return &Merger{}
}

// Merge combines a base configuration with one or more override configurations.
func (m *Merger) Merge(base *ComposeConfig, overrides ...*ComposeConfig) *ComposeConfig {
	if base == nil {
		if len(overrides) > 0 {
			return overrides[0]
		}
		return &ComposeConfig{Services: make(map[string]ServiceConfig)}
	}

	result := cloneConfig(base)

	for _, ov := range overrides {
		if ov == nil {
			continue
		}
		if ov.Version != "" {
			result.Version = ov.Version
		}
		if ov.Name != "" {
			result.Name = ov.Name
		}

		// Merge Services
		for sName, ovService := range ov.Services {
			baseService, exists := result.Services[sName]
			if !exists {
				ovService.Name = sName
				result.Services[sName] = ovService
				continue
			}
			result.Services[sName] = mergeService(baseService, ovService)
		}

		// Merge Networks
		if result.Networks == nil {
			result.Networks = make(map[string]NetworkConfig)
		}
		for netName, netCfg := range ov.Networks {
			result.Networks[netName] = netCfg
		}

		// Merge Volumes
		if result.Volumes == nil {
			result.Volumes = make(map[string]VolumeConfig)
		}
		for volName, volCfg := range ov.Volumes {
			result.Volumes[volName] = volCfg
		}

		// Merge Configs
		if result.Configs == nil {
			result.Configs = make(map[string]ConfigItem)
		}
		for cfgName, item := range ov.Configs {
			result.Configs[cfgName] = item
		}

		// Merge Secrets
		if result.Secrets == nil {
			result.Secrets = make(map[string]SecretItem)
		}
		for secName, item := range ov.Secrets {
			result.Secrets[secName] = item
		}
	}

	return result
}

// FilterByProfiles returns a new ComposeConfig with services filtered by active profiles.
// Services with no profiles specified are always included.
// If activeProfiles contains "*", all services are included.
func (m *Merger) FilterByProfiles(cfg *ComposeConfig, activeProfiles []string) *ComposeConfig {
	if cfg == nil {
		return nil
	}

	allActive := false
	profileMap := make(map[string]bool)
	for _, p := range activeProfiles {
		if p == "*" || p == "all" {
			allActive = true
			break
		}
		profileMap[p] = true
	}

	filtered := cloneConfig(cfg)
	filtered.Services = make(map[string]ServiceConfig)

	for name, s := range cfg.Services {
		// Services with no profiles are default and always active
		if len(s.Profiles) == 0 || allActive {
			filtered.Services[name] = s
			continue
		}

		// Check if any service profile matches activeProfiles
		matches := false
		for _, sp := range s.Profiles {
			if profileMap[sp] {
				matches = true
				break
			}
		}

		if matches {
			filtered.Services[name] = s
		}
	}

	return filtered
}

// GetAllProfiles returns a unique sorted list of all profiles declared across services.
func (m *Merger) GetAllProfiles(cfg *ComposeConfig) []string {
	if cfg == nil {
		return nil
	}
	seen := make(map[string]bool)
	var list []string
	for _, s := range cfg.Services {
		for _, p := range s.Profiles {
			if !seen[p] {
				seen[p] = true
				list = append(list, p)
			}
		}
	}
	return list
}

func cloneConfig(src *ComposeConfig) *ComposeConfig {
	dst := &ComposeConfig{
		Version:  src.Version,
		Name:     src.Name,
		Services: make(map[string]ServiceConfig),
		Networks: make(map[string]NetworkConfig),
		Volumes:  make(map[string]VolumeConfig),
		Configs:  make(map[string]ConfigItem),
		Secrets:  make(map[string]SecretItem),
	}

	for k, v := range src.Services {
		dst.Services[k] = v
	}
	for k, v := range src.Networks {
		dst.Networks[k] = v
	}
	for k, v := range src.Volumes {
		dst.Volumes[k] = v
	}
	for k, v := range src.Configs {
		dst.Configs[k] = v
	}
	for k, v := range src.Secrets {
		dst.Secrets[k] = v
	}
	return dst
}

func mergeService(base, ov ServiceConfig) ServiceConfig {
	res := base

	if ov.Image != "" {
		res.Image = ov.Image
	}
	if ov.Build != nil {
		res.Build = ov.Build
	}
	if ov.ContainerName != "" {
		res.ContainerName = ov.ContainerName
	}
	if ov.Restart != "" {
		res.Restart = ov.Restart
	}
	if ov.User != "" {
		res.User = ov.User
	}
	if ov.Privileged {
		res.Privileged = true
	}
	if ov.NetworkMode != "" {
		res.NetworkMode = ov.NetworkMode
	}
	if ov.HealthCheck != nil {
		res.HealthCheck = ov.HealthCheck
	}
	if len(ov.Command) > 0 {
		res.Command = ov.Command
	}
	if len(ov.Entrypoint) > 0 {
		res.Entrypoint = ov.Entrypoint
	}

	// Merge Environment
	if len(ov.Environment) > 0 {
		if res.Environment == nil {
			res.Environment = make(StringMapOrList)
		}
		for k, v := range ov.Environment {
			res.Environment[k] = v
		}
	}

	// Merge Ports (append unique targets/published)
	if len(ov.Ports) > 0 {
		seenPort := make(map[string]bool)
		for _, p := range res.Ports {
			seenPort[p.Raw] = true
		}
		for _, p := range ov.Ports {
			if !seenPort[p.Raw] {
				res.Ports = append(res.Ports, p)
				seenPort[p.Raw] = true
			}
		}
	}

	// Merge Volumes
	if len(ov.Volumes) > 0 {
		seenVol := make(map[string]bool)
		for _, v := range res.Volumes {
			seenVol[v.Target] = true
		}
		for _, v := range ov.Volumes {
			if !seenVol[v.Target] {
				res.Volumes = append(res.Volumes, v)
				seenVol[v.Target] = true
			}
		}
	}

	// Merge DependsOn
	if len(ov.DependsOn) > 0 {
		if res.DependsOn == nil {
			res.DependsOn = make(map[string]DependsOnCondition)
		}
		for dep, cond := range ov.DependsOn {
			res.DependsOn[dep] = cond
		}
	}

	// Merge Networks
	if len(ov.Networks) > 0 {
		seenNet := make(map[string]bool)
		for _, n := range res.Networks {
			seenNet[n] = true
		}
		for _, n := range ov.Networks {
			if !seenNet[n] {
				res.Networks = append(res.Networks, n)
				seenNet[n] = true
			}
		}
	}

	// Merge Profiles
	if len(ov.Profiles) > 0 {
		seenProf := make(map[string]bool)
		for _, p := range res.Profiles {
			seenProf[p] = true
		}
		for _, p := range ov.Profiles {
			if !seenProf[p] {
				res.Profiles = append(res.Profiles, p)
				seenProf[p] = true
			}
		}
	}

	// Merge Labels
	if len(ov.Labels) > 0 {
		if res.Labels == nil {
			res.Labels = make(StringMapOrList)
		}
		for k, v := range ov.Labels {
			res.Labels[k] = v
		}
	}

	// Merge Deploy
	if ov.Deploy != nil {
		res.Deploy = ov.Deploy
	}

	return res
}
