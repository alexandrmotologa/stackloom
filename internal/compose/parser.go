package compose

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseResult holds the parsed compose config and metadata about environment resolution.
type ParseResult struct {
	Config          *ComposeConfig
	ReferencedEnvs  map[string]string // VarName -> substituted value
	UnsetEnvs       []string          // Variables that were referenced but not set in env
	SourcePath      string
}

// Parser manages compose file loading and variable expansion.
type Parser struct {
	envMap map[string]string
}

// NewParser creates a parser initialized with host environment variables.
func NewParser(customEnv map[string]string) *Parser {
	combined := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			combined[parts[0]] = parts[1]
		}
	}
	for k, v := range customEnv {
		combined[k] = v
	}
	return &Parser{envMap: combined}
}

// LoadDotEnv reads a .env file and adds keys into the parser's environment.
func (p *Parser) LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip matching quotes if wrapped
			if len(val) >= 2 {
				if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
					val = val[1 : len(val)-1]
				}
			}
			p.envMap[key] = val
		}
	}
	return scanner.Err()
}

// ParseFile reads a compose file, auto-detects .env in the same directory, interpolates variables, and returns ParseResult.
func (p *Parser) ParseFile(filePath string) (*ParseResult, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	// Auto-load .env if located beside compose file
	dir := filepath.Dir(absPath)
	dotEnvPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(dotEnvPath); err == nil {
		_ = p.LoadDotEnv(dotEnvPath)
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file %s: %w", absPath, err)
	}

	return p.ParseBytes(contentBytes, absPath)
}

// ParseBytes interpolates environment variables and unmarshals YAML into ComposeConfig.
func (p *Parser) ParseBytes(content []byte, sourcePath string) (*ParseResult, error) {
	rawText := string(content)
	interpolated, refEnvs, unsetEnvs := p.interpolateVariables(rawText)

	var config ComposeConfig
	if err := yaml.Unmarshal([]byte(interpolated), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compose YAML (%s): %w", sourcePath, err)
	}

	// Assign names to services based on map keys
	for name, s := range config.Services {
		s.Name = name
		config.Services[name] = s
	}

	return &ParseResult{
		Config:         &config,
		ReferencedEnvs: refEnvs,
		UnsetEnvs:      unsetEnvs,
		SourcePath:     sourcePath,
	}, nil
}

// regex for compose env interpolation:
// 1. $${...} or $$VAR -> escaped to ${...} or $VAR
// 2. ${VAR:-default}, ${VAR-default}, ${VAR:?error}, ${VAR?error}, ${VAR:+replacement}, ${VAR+replacement}, ${VAR}
// 3. $VAR
var (
	envEscapeRegex = regexp.MustCompile(`\$\$`)
	envBlockRegex  = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?::?([-+?])([^}]*))?\}`)
	envSimpleRegex = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)
)

func (p *Parser) interpolateVariables(text string) (string, map[string]string, []string) {
	referenced := make(map[string]string)
	unsetsMap := make(map[string]bool)

	// Step 1: Temporarily replace $$ with a unique placeholder
	placeholder := "__STACKLOOM_ESCAPED_DOLLAR__"
	text = envEscapeRegex.ReplaceAllString(text, placeholder)

	// Step 2: Replace ${VAR...} blocks
	replaced := envBlockRegex.ReplaceAllStringFunc(text, func(m string) string {
		submatches := envBlockRegex.FindStringSubmatch(m)
		if len(submatches) < 2 {
			return m
		}
		varName := submatches[1]
		modifier := submatches[2]
		modifierVal := submatches[3]

		val, exists := p.envMap[varName]
		referenced[varName] = val

		// Modifier rules:
		// :- : default if unset or empty
		// -  : default if unset
		// :? : error if unset or empty
		// ?  : error if unset
		// :+ : replacement if set and not empty
		// +  : replacement if set
		hasColon := strings.Contains(m, ":"+modifier)

		switch modifier {
		case "-":
			if (!exists) || (hasColon && val == "") {
				return modifierVal
			}
			return val
		case "?":
			if (!exists) || (hasColon && val == "") {
				unsetsMap[varName] = true
				if modifierVal != "" {
					return modifierVal
				}
				return ""
			}
			return val
		case "+":
			if exists && (!hasColon || val != "") {
				return modifierVal
			}
			return ""
		default:
			if !exists {
				unsetsMap[varName] = true
				return ""
			}
			return val
		}
	})

	// Step 3: Replace remaining simple $VAR
	replaced = envSimpleRegex.ReplaceAllStringFunc(replaced, func(m string) string {
		varName := m[1:]
		val, exists := p.envMap[varName]
		referenced[varName] = val
		if !exists {
			unsetsMap[varName] = true
			return ""
		}
		return val
	})

	// Step 4: Restore escaped dollars
	replaced = strings.ReplaceAll(replaced, placeholder, "$")

	var unsets []string
	for k := range unsetsMap {
		unsets = append(unsets, k)
	}

	return replaced, referenced, unsets
}
