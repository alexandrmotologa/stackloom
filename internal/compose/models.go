package compose

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeConfig represents the root Docker Compose specification.
type ComposeConfig struct {
	Version  string                    `yaml:"version,omitempty" json:"version,omitempty"`
	Name     string                    `yaml:"name,omitempty" json:"name,omitempty"`
	Services map[string]ServiceConfig  `yaml:"services" json:"services"`
	Networks map[string]NetworkConfig  `yaml:"networks,omitempty" json:"networks,omitempty"`
	Volumes  map[string]VolumeConfig   `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Configs  map[string]ConfigItem     `yaml:"configs,omitempty" json:"configs,omitempty"`
	Secrets  map[string]SecretItem     `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

// ServiceConfig represents a single service within Docker Compose.
type ServiceConfig struct {
	Name          string                        `yaml:"-" json:"name"`
	Image         string                        `yaml:"image,omitempty" json:"image,omitempty"`
	Build         *BuildConfig                  `yaml:"build,omitempty" json:"build,omitempty"`
	ContainerName string                        `yaml:"container_name,omitempty" json:"container_name,omitempty"`
	Ports         []PortMapping                 `yaml:"ports,omitempty" json:"ports,omitempty"`
	Expose        StringList                    `yaml:"expose,omitempty" json:"expose,omitempty"`
	Environment   StringMapOrList               `yaml:"environment,omitempty" json:"environment,omitempty"`
	EnvFile       StringList                    `yaml:"env_file,omitempty" json:"env_file,omitempty"`
	DependsOn     DependsOnMap                  `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Networks      ServiceNetworks               `yaml:"networks,omitempty" json:"networks,omitempty"`
	Volumes       []VolumeMapping               `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Command       CommandList                   `yaml:"command,omitempty" json:"command,omitempty"`
	Entrypoint    CommandList                   `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	HealthCheck   *HealthCheckConfig            `yaml:"healthcheck,omitempty" json:"healthcheck,omitempty"`
	Restart       string                        `yaml:"restart,omitempty" json:"restart,omitempty"`
	User          string                        `yaml:"user,omitempty" json:"user,omitempty"`
	Privileged    bool                          `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	NetworkMode   string                        `yaml:"network_mode,omitempty" json:"network_mode,omitempty"`
	Profiles      StringList                    `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	Deploy        *DeployConfig                 `yaml:"deploy,omitempty" json:"deploy,omitempty"`
	CapAdd        StringList                    `yaml:"cap_add,omitempty" json:"cap_add,omitempty"`
	Labels        StringMapOrList               `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// BuildConfig holds Dockerfile build settings.
type BuildConfig struct {
	Context    string            `yaml:"context,omitempty" json:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`
	Target     string            `yaml:"target,omitempty" json:"target,omitempty"`
	Args       StringMapOrList   `yaml:"args,omitempty" json:"args,omitempty"`
}

// PortMapping models host-to-container port bindings.
type PortMapping struct {
	Target    int    `json:"target"`
	Published int    `json:"published,omitempty"`
	HostIP    string `json:"host_ip,omitempty"`
	Protocol  string `json:"protocol,omitempty"` // tcp or udp
	Raw       string `json:"raw"`
}

// VolumeMapping models bind mounts and named volumes.
type VolumeMapping struct {
	Type     string `json:"type"` // "bind", "volume", etc.
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Raw      string `json:"raw"`
}

// DependsOnCondition specifies the readiness condition for dependencies.
type DependsOnCondition struct {
	Condition string `json:"condition"` // "service_started", "service_healthy", "service_completed_successfully"
	Restart   bool   `json:"restart,omitempty"`
	Required  bool   `json:"required,omitempty"`
}

// HealthCheckConfig defines service container health probing.
type HealthCheckConfig struct {
	Test        CommandList `yaml:"test,omitempty" json:"test,omitempty"`
	Interval    string      `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout     string      `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retries     int         `yaml:"retries,omitempty" json:"retries,omitempty"`
	StartPeriod string      `yaml:"start_period,omitempty" json:"start_period,omitempty"`
	Disable     bool        `yaml:"disable,omitempty" json:"disable,omitempty"`
}

// DeployConfig defines resource constraints and replication.
type DeployConfig struct {
	Replicas  int              `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	Resources *ResourcesConfig `yaml:"resources,omitempty" json:"resources,omitempty"`
}

// ResourcesConfig models limits and reservations.
type ResourcesConfig struct {
	Limits       *ResourceLimits `yaml:"limits,omitempty" json:"limits,omitempty"`
	Reservations *ResourceLimits `yaml:"reservations,omitempty" json:"reservations,omitempty"`
}

// ResourceLimits specifies CPU and memory ceilings.
type ResourceLimits struct {
	CPUs   string `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty" json:"memory,omitempty"`
}

// NetworkConfig models top-level networks.
type NetworkConfig struct {
	Driver     string            `yaml:"driver,omitempty" json:"driver,omitempty"`
	External   bool              `yaml:"external,omitempty" json:"external,omitempty"`
	Attachable bool              `yaml:"attachable,omitempty" json:"attachable,omitempty"`
	Labels     StringMapOrList   `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// VolumeConfig models top-level volumes.
type VolumeConfig struct {
	Driver   string          `yaml:"driver,omitempty" json:"driver,omitempty"`
	External bool            `yaml:"external,omitempty" json:"external,omitempty"`
	Labels   StringMapOrList `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ConfigItem models top-level configs.
type ConfigItem struct {
	File     string `yaml:"file,omitempty" json:"file,omitempty"`
	External bool   `yaml:"external,omitempty" json:"external,omitempty"`
}

// SecretItem models top-level secrets.
type SecretItem struct {
	File        string `yaml:"file,omitempty" json:"file,omitempty"`
	External    bool   `yaml:"external,omitempty" json:"external,omitempty"`
	Environment string `yaml:"environment,omitempty" json:"environment,omitempty"`
}

// StringList handles fields that can be a single string or a slice of strings.
type StringList []string

func (s *StringList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*s = []string{value.Value}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var list []string
		for _, item := range value.Content {
			list = append(list, item.Value)
		}
		*s = list
		return nil
	}
	return nil
}

// CommandList handles command / entrypoint arrays or shell strings.
type CommandList []string

func (c *CommandList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*c = strings.Fields(value.Value)
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var list []string
		for _, item := range value.Content {
			list = append(list, item.Value)
		}
		*c = list
		return nil
	}
	return nil
}

// StringMapOrList handles environment or labels as either ["KEY=VAL"] or {"KEY": "VAL"}.
type StringMapOrList map[string]string

func (m *StringMapOrList) UnmarshalYAML(value *yaml.Node) error {
	res := make(map[string]string)
	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content); i += 2 {
			k := value.Content[i].Value
			v := value.Content[i+1].Value
			res[k] = v
		}
		*m = res
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			parts := strings.SplitN(item.Value, "=", 2)
			if len(parts) == 2 {
				res[parts[0]] = parts[1]
			} else {
				res[parts[0]] = ""
			}
		}
		*m = res
		return nil
	}
	return nil
}

// ServiceNetworks handles service-level networks: either list of names or map of configs.
type ServiceNetworks []string

func (sn *ServiceNetworks) UnmarshalYAML(value *yaml.Node) error {
	var list []string
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			list = append(list, item.Value)
		}
		*sn = list
		return nil
	}
	if value.Kind == yaml.MappingNode {
		for i := 0; i < len(value.Content); i += 2 {
			list = append(list, value.Content[i].Value)
		}
		*sn = list
		return nil
	}
	return nil
}

// UnmarshalYAML for PortMapping parses both short strings ("8080:80") and long map forms.
func (p *PortMapping) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		parsed, err := ParsePortString(value.Value)
		if err != nil {
			return err
		}
		*p = parsed
		return nil
	}
	if value.Kind == yaml.MappingNode {
		var raw struct {
			Target    int    `yaml:"target"`
			Published int    `yaml:"published"`
			HostIP    string `yaml:"host_ip"`
			Protocol  string `yaml:"protocol"`
		}
		if err := value.Decode(&raw); err != nil {
			return err
		}
		p.Target = raw.Target
		p.Published = raw.Published
		p.HostIP = raw.HostIP
		p.Protocol = raw.Protocol
		if p.Protocol == "" {
			p.Protocol = "tcp"
		}
		p.Raw = fmt.Sprintf("%d:%d/%s", p.Published, p.Target, p.Protocol)
		return nil
	}
	return nil
}

// ParsePortString parses formats: "80", "8080:80", "127.0.0.1:8080:80", "8080:80/tcp", "8080:80/udp".
func ParsePortString(raw string) (PortMapping, error) {
	pm := PortMapping{
		Protocol: "tcp",
		Raw:      raw,
	}

	work := raw
	if idx := strings.Index(work, "/"); idx != -1 {
		pm.Protocol = strings.ToLower(work[idx+1:])
		work = work[:idx]
	}

	parts := strings.Split(work, ":")
	switch len(parts) {
	case 1:
		// Target only (container port)
		target, err := strconv.Atoi(parts[0])
		if err != nil {
			return pm, fmt.Errorf("invalid port target: %s", parts[0])
		}
		pm.Target = target
	case 2:
		// published:target
		pub, err := strconv.Atoi(parts[0])
		if err != nil {
			return pm, fmt.Errorf("invalid published port: %s", parts[0])
		}
		target, err := strconv.Atoi(parts[1])
		if err != nil {
			return pm, fmt.Errorf("invalid target port: %s", parts[1])
		}
		pm.Published = pub
		pm.Target = target
	case 3:
		// hostIP:published:target
		pm.HostIP = parts[0]
		pub, err := strconv.Atoi(parts[1])
		if err != nil {
			return pm, fmt.Errorf("invalid published port: %s", parts[1])
		}
		target, err := strconv.Atoi(parts[2])
		if err != nil {
			return pm, fmt.Errorf("invalid target port: %s", parts[2])
		}
		pm.Published = pub
		pm.Target = target
	default:
		return pm, fmt.Errorf("unsupported port format: %s", raw)
	}

	return pm, nil
}

// UnmarshalYAML for VolumeMapping parses short strings ("./data:/var/lib/data:ro") and map objects.
func (v *VolumeMapping) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		parsed := ParseVolumeString(value.Value)
		*v = parsed
		return nil
	}
	if value.Kind == yaml.MappingNode {
		var raw struct {
			Type     string `yaml:"type"`
			Source   string `yaml:"source"`
			Target   string `yaml:"target"`
			ReadOnly bool   `yaml:"read_only"`
		}
		if err := value.Decode(&raw); err != nil {
			return err
		}
		v.Type = raw.Type
		v.Source = raw.Source
		v.Target = raw.Target
		v.ReadOnly = raw.ReadOnly
		v.Raw = fmt.Sprintf("%s:%s", v.Source, v.Target)
		return nil
	}
	return nil
}

// ParseVolumeString parses "source:target", "source:target:ro", or "target".
func ParseVolumeString(raw string) VolumeMapping {
	vm := VolumeMapping{
		Raw:  raw,
		Type: "volume",
	}
	parts := strings.Split(raw, ":")
	if len(parts) == 1 {
		vm.Target = parts[0]
		return vm
	}

	vm.Source = parts[0]
	vm.Target = parts[1]
	if len(parts) >= 3 && (parts[2] == "ro" || strings.Contains(parts[2], "ro")) {
		vm.ReadOnly = true
	}

	if strings.HasPrefix(vm.Source, ".") || strings.HasPrefix(vm.Source, "/") || strings.HasPrefix(vm.Source, "~") || strings.Contains(vm.Source, "\\") {
		vm.Type = "bind"
	}
	return vm
}

// DependsOnMap handles depends_on as either sequence of service names or mapping of conditions.
type DependsOnMap map[string]DependsOnCondition

func (d *DependsOnMap) UnmarshalYAML(value *yaml.Node) error {
	res := make(map[string]DependsOnCondition)
	if value.Kind == yaml.SequenceNode {
		for _, depNode := range value.Content {
			res[depNode.Value] = DependsOnCondition{
				Condition: "service_started",
			}
		}
		*d = res
		return nil
	}
	if value.Kind == yaml.MappingNode {
		for j := 0; j < len(value.Content); j += 2 {
			depName := value.Content[j].Value
			condNode := value.Content[j+1]
			var cond DependsOnCondition
			if condNode.Kind == yaml.MappingNode {
				_ = condNode.Decode(&cond)
				if cond.Condition == "" {
					cond.Condition = "service_started"
				}
			} else {
				cond = DependsOnCondition{Condition: "service_started"}
			}
			res[depName] = cond
		}
		*d = res
		return nil
	}
	return nil
}
