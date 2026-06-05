package vllm

import (
	"strings"
	"testing"

	"github.com/bloc-org/bloc/internal/engine/docker"
	"github.com/bloc-org/bloc/internal/recipe"
)

func TestNew_DockerVLLM_MissingImage(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "vllm",
			Runtime: "docker",
			Image:   "", // missing
		},
	}
	_, err := New(r)
	if err == nil {
		t.Error("expected error when engine.image is missing for docker runtime")
	}
	if !strings.Contains(err.Error(), "engine.image") {
		t.Errorf("error should mention engine.image, got: %v", err)
	}
}

func TestNew_DockerVLLM_WithImage(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "vllm",
			Runtime: "docker",
			Image:   "vllm/vllm-openai:v0.9.0",
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
	if dockerEng.Image != "vllm/vllm-openai:v0.9.0" {
		t.Errorf("image = %q, want vllm/vllm-openai:v0.9.0", dockerEng.Image)
	}
}

func TestNativeVLLMEngine_Name_WithVersion(t *testing.T) {
	rt := &NativeVLLMEngine{version: "0.9.0"}
	want := "vLLM 0.9.0 (native)"
	if rt.Name() != want {
		t.Errorf("Name() = %q, want %q", rt.Name(), want)
	}
}

func TestNativeVLLMEngine_Name_NoVersion(t *testing.T) {
	rt := &NativeVLLMEngine{}
	want := "vLLM (native)"
	if rt.Name() != want {
		t.Errorf("Name() = %q, want %q", rt.Name(), want)
	}
}
