package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents a project-level `.dv/config.yaml` configuration file.
type ProjectConfig struct {
	Image               ProjectImageConfig `yaml:"image"`
	OnCreate            []string           `yaml:"on_create,omitempty"`
	Services            []string           `yaml:"services,omitempty"`
	Lifecycle           ProjectLifecycle   `yaml:"lifecycle,omitempty"`
	Git                 ProjectGit         `yaml:"git,omitempty"`
	HostStartingPort    int                `yaml:"host_starting_port,omitempty"`
	EnvPassthrough      []string           `yaml:"env_passthrough,omitempty"`
	Env                 map[string]string  `yaml:"env,omitempty"`
	CopyRules           []CopyRule         `yaml:"copy_rules,omitempty"`
	ExtractBranchPrefix string             `yaml:"extract_branch_prefix,omitempty"`
	// Volumes are bind mounts in "host:container" format.
	// When set, these are used instead of copying the project into the container.
	Volumes []string `yaml:"volumes,omitempty"`
}

// ProjectImageConfig describes the Docker image for a project.
type ProjectImageConfig struct {
	Dockerfile    string   `yaml:"dockerfile,omitempty"`
	Tag           string   `yaml:"tag"`
	Workdir       string   `yaml:"workdir"`
	ContainerPort int      `yaml:"container_port"`
	User          string   `yaml:"user"`
	Command       []string `yaml:"command,omitempty"`
}

// ProjectLifecycle defines lifecycle hook commands.
type ProjectLifecycle struct {
	PostCheckout []string `yaml:"post_checkout,omitempty"`
	ResetDB      []string `yaml:"reset_db,omitempty"`
	Catchup      []string `yaml:"catchup,omitempty"`
}

// ProjectGit holds git-related project settings.
type ProjectGit struct {
	DefaultBranch string `yaml:"default_branch,omitempty"`
}

// FindProjectConfig walks up from startDir looking for `.dv/config.yaml` or `.dv/config.yml`.
// Returns (config, projectRoot, error). Returns (nil, "", nil) when not found.
func FindProjectConfig(startDir string) (*ProjectConfig, string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, "", err
	}

	for {
		for _, name := range []string{"config.yaml", "config.yml"} {
			candidate := filepath.Join(dir, ".dv", name)
			data, err := os.ReadFile(candidate)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, "", err
			}
			var pc ProjectConfig
			if err := yaml.Unmarshal(data, &pc); err != nil {
				return nil, "", fmt.Errorf("parsing %s: %w", candidate, err)
			}
			if err := pc.Validate(); err != nil {
				return nil, "", fmt.Errorf("validating %s: %w", candidate, err)
			}
			return &pc, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, "", nil
}

// Validate checks that the project config is self-consistent.
func (pc *ProjectConfig) Validate() error {
	if strings.TrimSpace(pc.Image.Tag) == "" {
		return fmt.Errorf("image: 'tag' is required")
	}
	return nil
}

// ToImageConfig converts a ProjectConfig to an ImageConfig for CLI consumption.
func (pc *ProjectConfig) ToImageConfig(projectRoot string) ImageConfig {
	img := ImageConfig{
		Kind:          "custom",
		Tag:           pc.Image.Tag,
		Workdir:       pc.Image.Workdir,
		ContainerPort: pc.Image.ContainerPort,
		User:          pc.Image.User,
		Command:       pc.Image.Command,
	}

	// Resolve Dockerfile path: explicit value or default to .dv/Dockerfile
	p := pc.Image.Dockerfile
	if strings.TrimSpace(p) == "" {
		p = ".dv/Dockerfile"
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(projectRoot, p)
	}
	img.Dockerfile = ImageSource{Source: "path", Path: p}

	return img
}
