package sglang

import (
	"strings"
	"testing"

	"github.com/bloc-org/bloc/internal/engine/docker"
	"github.com/bloc-org/bloc/internal/recipe"
)

func TestNew_SGLang_MissingImage(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "docker",
			Image:   "", // missing
		},
	}
	_, err := New(r)
	if err == nil {
		t.Error("expected error when engine.image is missing")
	}
	if !strings.Contains(err.Error(), "engine.image") {
		t.Errorf("error should mention engine.image, got: %v", err)
	}
}

func TestNew_SGLang_NativeUnsupported(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "native",
			Image:   "lmsysorg/sglang:v0.5.12.post1",
		},
	}
	_, err := New(r)
	if err == nil {
		t.Error("expected error for native runtime")
	}
	if !strings.Contains(err.Error(), "native") {
		t.Errorf("error should mention native, got: %v", err)
	}
}

func TestNew_SGLang_WithImage(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "docker",
			Image:   "lmsysorg/sglang:v0.5.12.post1",
		},
	}
	e, err := New(r)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	dockerEng, ok := e.(*docker.DockerEngine)
	if !ok {
		t.Fatalf("expected *docker.DockerEngine, got %T", e)
	}
	if dockerEng.Image != "lmsysorg/sglang:v0.5.12.post1" {
		t.Errorf("image = %q, want lmsysorg/sglang:v0.5.12.post1", dockerEng.Image)
	}
}
