package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig_WalksUp(t *testing.T) {
	tmp := t.TempDir()
	// Create .dv/config.yaml two levels up
	dvDir := filepath.Join(tmp, ".dv")
	if err := os.MkdirAll(dvDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `image:
  tag: test
  workdir: /app
  user: dev
`
	if err := os.WriteFile(filepath.Join(dvDir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a nested directory
	nested := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	pc, root, err := FindProjectConfig(nested)
	if err != nil {
		t.Fatal(err)
	}
	if pc == nil {
		t.Fatal("expected project config, got nil")
	}
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
	if pc.Image.Tag != "test" {
		t.Errorf("expected tag test, got %q", pc.Image.Tag)
	}
}

func TestFindProjectConfig_PrefersYAML(t *testing.T) {
	tmp := t.TempDir()
	dvDir := filepath.Join(tmp, ".dv")
	if err := os.MkdirAll(dvDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `image:
  tag: from-yaml
  workdir: /app
  user: dev
`
	ymlContent := `image:
  tag: from-yml
  workdir: /app
  user: dev
`
	if err := os.WriteFile(filepath.Join(dvDir, "config.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dvDir, "config.yml"), []byte(ymlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	pc, _, err := FindProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if pc.Image.Tag != "from-yaml" {
		t.Errorf("expected tag from-yaml (.yaml preferred), got %q", pc.Image.Tag)
	}
}

func TestFindProjectConfig_YMLFallback(t *testing.T) {
	tmp := t.TempDir()
	dvDir := filepath.Join(tmp, ".dv")
	if err := os.MkdirAll(dvDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yml := `image:
  tag: from-yml
  workdir: /app
  user: dev
`
	if err := os.WriteFile(filepath.Join(dvDir, "config.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	pc, _, err := FindProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if pc.Image.Tag != "from-yml" {
		t.Errorf("expected tag from-yml, got %q", pc.Image.Tag)
	}
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	tmp := t.TempDir()
	pc, root, err := FindProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if pc != nil {
		t.Errorf("expected nil config, got %+v", pc)
	}
	if root != "" {
		t.Errorf("expected empty root, got %q", root)
	}
}

func TestFindProjectConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	dvDir := filepath.Join(tmp, ".dv")
	if err := os.MkdirAll(dvDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dvDir, "config.yaml"), []byte("{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := FindProjectConfig(tmp)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_RequiresTag(t *testing.T) {
	pc := &ProjectConfig{Image: ProjectImageConfig{}}
	if err := pc.Validate(); err == nil {
		t.Fatal("expected error when tag is missing")
	}
}

func TestValidate_OK(t *testing.T) {
	pc := &ProjectConfig{Image: ProjectImageConfig{Tag: "test"}}
	if err := pc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToImageConfig_DefaultDockerfile(t *testing.T) {
	pc := &ProjectConfig{
		Image: ProjectImageConfig{
			Tag:           "test-img",
			Workdir:       "/app",
			ContainerPort: 8080,
			User:          "dev",
		},
	}
	ic := pc.ToImageConfig("/project")
	if ic.Kind != "custom" {
		t.Errorf("expected kind custom, got %q", ic.Kind)
	}
	if ic.Dockerfile.Source != "path" {
		t.Errorf("expected source path, got %q", ic.Dockerfile.Source)
	}
	if ic.Dockerfile.Path != "/project/.dv/Dockerfile" {
		t.Errorf("expected default Dockerfile path, got %q", ic.Dockerfile.Path)
	}
}

func TestToImageConfig_ExplicitDockerfile(t *testing.T) {
	pc := &ProjectConfig{
		Image: ProjectImageConfig{
			Dockerfile: "docker/Dockerfile.dev",
			Tag:        "test-img",
		},
	}
	ic := pc.ToImageConfig("/project")
	if ic.Dockerfile.Source != "path" {
		t.Errorf("expected source path, got %q", ic.Dockerfile.Source)
	}
	if ic.Dockerfile.Path != "/project/docker/Dockerfile.dev" {
		t.Errorf("expected resolved path, got %q", ic.Dockerfile.Path)
	}
}

func TestToImageConfig_AbsoluteDockerfile(t *testing.T) {
	pc := &ProjectConfig{
		Image: ProjectImageConfig{
			Dockerfile: "/opt/Dockerfile",
			Tag:        "test-img",
		},
	}
	ic := pc.ToImageConfig("/project")
	if ic.Dockerfile.Path != "/opt/Dockerfile" {
		t.Errorf("expected absolute path preserved, got %q", ic.Dockerfile.Path)
	}
}
