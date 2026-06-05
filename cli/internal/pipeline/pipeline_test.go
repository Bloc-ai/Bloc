package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bloc-org/bloc/internal/pipeline"
	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── Pipeline orchestrator tests ──────────────────────────────────────────────

// testStage is a minimal Stage implementation for orchestrator tests.
type testStage struct {
	name    string
	runFunc func(ctx context.Context, state *pipeline.RunState) error
	ran     bool
}

func (s *testStage) Name() string { return s.name }
func (s *testStage) Run(ctx context.Context, state *pipeline.RunState) error {
	s.ran = true
	if s.runFunc != nil {
		return s.runFunc(ctx, state)
	}
	return nil
}

func TestPipeline_Execute_AllStagesRun(t *testing.T) {
	a := &testStage{name: "A"}
	b := &testStage{name: "B"}
	c := &testStage{name: "C"}

	p := pipeline.New(a, b, c)
	state := &pipeline.RunState{}
	if err := p.Execute(context.Background(), state); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !a.ran || !b.ran || !c.ran {
		t.Error("expected all stages to run, but at least one did not")
	}
}

func TestPipeline_Execute_StopsOnFirstError(t *testing.T) {
	boom := errors.New("stage B exploded")
	a := &testStage{name: "A"}
	b := &testStage{name: "B", runFunc: func(_ context.Context, _ *pipeline.RunState) error {
		return boom
	}}
	c := &testStage{name: "C"}

	p := pipeline.New(a, b, c)
	err := p.Execute(context.Background(), &pipeline.RunState{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped boom error, got: %v", err)
	}
	if c.ran {
		t.Error("stage C should NOT run after stage B failed")
	}
}

func TestPipeline_Execute_ErrorWrapsWithStageName(t *testing.T) {
	p := pipeline.New(&testStage{
		name:    "MyStage",
		runFunc: func(_ context.Context, _ *pipeline.RunState) error { return fmt.Errorf("something broke") },
	})
	err := p.Execute(context.Background(), &pipeline.RunState{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MyStage") {
		t.Errorf("error should mention stage name; got: %v", err)
	}
}

func TestPipeline_Execute_StateFlowsThrough(t *testing.T) {
	// Stage A writes to state, Stage B reads it.
	a := &testStage{name: "A", runFunc: func(_ context.Context, s *pipeline.RunState) error {
		s.ModelPath = "/tmp/model.gguf"
		return nil
	}}
	var gotPath string
	b := &testStage{name: "B", runFunc: func(_ context.Context, s *pipeline.RunState) error {
		gotPath = s.ModelPath
		return nil
	}}

	p := pipeline.New(a, b)
	if err := p.Execute(context.Background(), &pipeline.RunState{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPath != "/tmp/model.gguf" {
		t.Errorf("state.ModelPath not propagated: got %q", gotPath)
	}
}

func TestPipeline_Execute_ContextCancelBetweenStages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	a := &testStage{name: "A", runFunc: func(_ context.Context, _ *pipeline.RunState) error {
		cancel() // cancel after A
		return nil
	}}
	b := &testStage{name: "B"}

	p := pipeline.New(a, b)
	err := p.Execute(ctx, &pipeline.RunState{})

	// The pipeline should return ctx.Err() because the context was cancelled
	// between A and B.
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if b.ran {
		t.Error("stage B should not run after context is cancelled")
	}
}

// ─── RunState helpers ─────────────────────────────────────────────────────────

func TestRunState_EngineName_Default(t *testing.T) {
	s := &pipeline.RunState{Recipe: &recipe.Recipe{}}
	if got := s.EngineName(); got != "llama.cpp" {
		t.Errorf("EngineName() = %q, want llama.cpp", got)
	}
}

func TestRunState_EngineName_ExplicitVLLM(t *testing.T) {
	s := &pipeline.RunState{
		Recipe: &recipe.Recipe{
			Engine: recipe.Engine{Name: "vllm"},
		},
	}
	if got := s.EngineName(); got != "vllm" {
		t.Errorf("EngineName() = %q, want vllm", got)
	}
}

func TestRunState_ResolvedPort_DefaultTo8080(t *testing.T) {
	s := &pipeline.RunState{Recipe: &recipe.Recipe{}}
	if got := s.ResolvedPort(); got != 8080 {
		t.Errorf("ResolvedPort() = %d, want 8080", got)
	}
}

func TestRunState_ResolvedPort_FromRecipe(t *testing.T) {
	s := &pipeline.RunState{
		Recipe: &recipe.Recipe{EngineConfig: recipe.EngineConfig{Port: 9000}},
	}
	if got := s.ResolvedPort(); got != 9000 {
		t.Errorf("ResolvedPort() = %d, want 9000", got)
	}
}

// ─── FetchRecipeStage — local path detection ──────────────────────────────────

func TestFetchRecipeStage_Local_NotFound(t *testing.T) {
	// With the Bug 13 fix, a non-existent .yaml path is no longer treated as
	// local — it falls through to fetchRemote, which returns an "invalid recipe ID"
	// error because the path has slashes.
	stage := &pipeline.FetchRecipeStage{}
	state := &pipeline.RunState{
		RecipeID: "/definitely/does/not/exist.yaml",
		APIBase:  "https://example.com",
	}
	err := stage.Run(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for non-existent local file")
	}
	// Now falls through to fetchRemote — gets "invalid recipe ID" or "invalid author"
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestFetchRecipeStage_LocalYAML_Exists verifies that an existing .yaml file is
// correctly treated as local (Bug 13: existence check for .yaml paths).
func TestFetchRecipeStage_LocalYAML_Exists(t *testing.T) {
	// Write a minimal valid YAML recipe to a temp file.
	yamlContent := []byte(`schema: "bloc/v1"
metadata:
  name: "local-test"
  description: "local test recipe"
model:
  file: "local.gguf"
  download_url: "https://example.com/local.gguf"
  size_gb: 1.0
engine:
  name: "llama.cpp"
`)
	tmpFile, err := os.CreateTemp(t.TempDir(), "recipe-*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tmpFile.Write(yamlContent); err != nil {
		t.Fatalf("Write: %v", err)
	}
	tmpFile.Close()

	stage := &pipeline.FetchRecipeStage{}
	state := &pipeline.RunState{
		RecipeID: tmpFile.Name(),
		APIBase:  "https://example.com",
	}
	if err := stage.Run(context.Background(), state); err != nil {
		t.Fatalf("unexpected error for existing local file: %v", err)
	}
	if !state.IsLocal {
		t.Error("expected state.IsLocal = true for local file")
	}
	if state.Recipe == nil || state.Recipe.Metadata.Name != "local-test" {
		t.Errorf("expected recipe name 'local-test', got %v", state.Recipe)
	}
}

// TestFetchRecipeStage_TypoYAML_GivesClearError verifies that a typo in a .yaml
// filename (file doesn't exist) produces a clear error instead of
// "cannot parse local recipe" (Bug 13 fix).
func TestFetchRecipeStage_TypoYAML_GivesClearError(t *testing.T) {
	stage := &pipeline.FetchRecipeStage{}
	state := &pipeline.RunState{
		RecipeID: "my-modeel.yaml", // typo — file doesn't exist
		APIBase:  "https://example.com",
	}
	err := stage.Run(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for non-existent yaml file")
	}
	// Should get "invalid recipe ID" (falls through to remote), NOT "cannot parse local recipe"
	if strings.Contains(err.Error(), "cannot parse local recipe") {
		t.Errorf("Bug 13: got misleading 'cannot parse local recipe' for a typo'd filename: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid recipe ID") {
		t.Errorf("expected 'invalid recipe ID' error for typo'd yaml name, got: %v", err)
	}
}

func TestFetchRecipeStage_Remote_InvalidID_NoSlash(t *testing.T) {
	stage := &pipeline.FetchRecipeStage{}
	state := &pipeline.RunState{
		RecipeID: "justonepart", // no slash, not a local file
		APIBase:  "https://example.com",
	}
	err := stage.Run(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for invalid recipe ID")
	}
	if !strings.Contains(err.Error(), "invalid recipe ID") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchRecipeStage_Remote_InvalidAuthor(t *testing.T) {
	stage := &pipeline.FetchRecipeStage{}
	state := &pipeline.RunState{
		RecipeID: "../evil/recipe", // path traversal attempt
		APIBase:  "https://example.com",
	}
	err := stage.Run(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for path-traversal author")
	}
	if !strings.Contains(err.Error(), "invalid author") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchRecipeStage_Remote_Mock_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	stage := &pipeline.FetchRecipeStage{APIClient: srv.Client()}
	state := &pipeline.RunState{
		RecipeID: "author/recipe",
		APIBase:  srv.URL,
	}
	err := stage.Run(context.Background(), state)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error; got: %v", err)
	}
}

func TestFetchRecipeStage_Remote_Mock_YAML(t *testing.T) {
	yamlBody := `schema: "bloc/v1"
metadata:
  name: "test-model"
  description: "A test model"
model:
  file: "test.gguf"
  download_url: "https://example.com/test.gguf"
  size_gb: 1.0
engine:
  name: "llama.cpp"
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the User-Agent header is set.
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "bloc-cli") {
			t.Errorf("missing or wrong User-Agent: %q", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(yamlBody))
	}))
	defer srv.Close()

	stage := &pipeline.FetchRecipeStage{APIClient: srv.Client()}
	state := &pipeline.RunState{
		RecipeID: "author/recipe",
		APIBase:  srv.URL,
	}
	if err := stage.Run(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Recipe == nil {
		t.Fatal("state.Recipe is nil after successful fetch")
	}
	if state.Recipe.Metadata.Name != "test-model" {
		t.Errorf("Recipe.Metadata.Name = %q, want test-model", state.Recipe.Metadata.Name)
	}
	if state.IsLocal {
		t.Error("IsLocal should be false for remote fetch")
	}
}

func TestFetchRecipeStage_Remote_Mock_JSONEnvelope(t *testing.T) {
	// Hub API wraps YAML inside a JSON envelope: {"yaml_content": "..."}
	yamlContent := `schema: "bloc/v1"
metadata:
  name: "envelope-model"
  description: "Envelope test"
model:
  file: "x.gguf"
  download_url: "https://example.com/x.gguf"
  size_gb: 0.5
engine:
  name: "llama.cpp"
`
	jsonBody := `{"yaml_content": ` + string(mustMarshalString(yamlContent)) + `}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer srv.Close()

	stage := &pipeline.FetchRecipeStage{APIClient: srv.Client()}
	state := &pipeline.RunState{RecipeID: "author/recipe", APIBase: srv.URL}
	if err := stage.Run(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Recipe.Metadata.Name != "envelope-model" {
		t.Errorf("Recipe.Metadata.Name = %q, want envelope-model", state.Recipe.Metadata.Name)
	}
}

// ─── BuildFlagsStage dry-run sentinel ────────────────────────────────────────

func TestIsDryRunDone_True(t *testing.T) {
	stage := &testStage{
		name: "flags",
		runFunc: func(_ context.Context, _ *pipeline.RunState) error {
			return pipeline.ErrDryRunDoneForTest()
		},
	}
	p := pipeline.New(stage)
	err := p.Execute(context.Background(), &pipeline.RunState{})
	if !pipeline.IsDryRunDone(errors.Unwrap(err)) {
		t.Errorf("IsDryRunDone should return true for dry-run sentinel, got: %v", err)
	}
}

func TestIsDryRunDone_False_ForRealError(t *testing.T) {
	if pipeline.IsDryRunDone(errors.New("real error")) {
		t.Error("IsDryRunDone should return false for a real error")
	}
}

func TestIsDryRunDone_False_ForNil(t *testing.T) {
	if pipeline.IsDryRunDone(nil) {
		t.Error("IsDryRunDone should return false for nil")
	}
}

// ─── Log file helpers ─────────────────────────────────────────────────────────

func TestSanitizeLogSlug_Normal(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"my-recipe", "my-recipe"},
		{"My Recipe 123", "My-Recipe-123"},
		{"../../etc/passwd", "..-..-etc-passwd"}, // path separators sanitised
		{"", "unknown"},
		{strings.Repeat("a", 100), strings.Repeat("a", 48)},
	}
	for _, tt := range tests {
		got := pipeline.SanitizeLogSlugForTest(tt.in)
		if got != tt.want {
			t.Errorf("sanitizeLogSlug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOpenEngineLogFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	f, err := pipeline.OpenEngineLogFileForTest(dir, "test-recipe")
	if err != nil {
		t.Fatalf("openEngineLogFile: %v", err)
	}
	defer f.Close()

	if _, err := os.Stat(f.Name()); err != nil {
		t.Errorf("log file does not exist at %s: %v", f.Name(), err)
	}
	if !strings.Contains(filepath.Base(f.Name()), "engine-test-recipe-") {
		t.Errorf("log filename %q does not match expected pattern", f.Name())
	}
}

func TestPruneEngineLogs_KeepsOnlyN(t *testing.T) {
	dir := t.TempDir()

	// Create 15 fake log files with distinct timestamps in the name.
	for i := 0; i < 15; i++ {
		name := fmt.Sprintf("engine-test-%08d.log", i)
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		f.Close()
	}

	if err := pipeline.PruneEngineLogsForTest(dir, 10); err != nil {
		t.Fatalf("pruneEngineLogs: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 10 {
		t.Errorf("expected 10 log files after pruning, got %d", len(entries))
	}
	// Files 00000000–00000004 (the 5 oldest) should be deleted.
	// Files 00000005–00000014 (the 10 newest) should remain.
	deletedPrefixes := []string{
		"engine-test-00000000",
		"engine-test-00000001",
		"engine-test-00000002",
		"engine-test-00000003",
		"engine-test-00000004",
	}
	remainingNames := make(map[string]bool)
	for _, e := range entries {
		remainingNames[e.Name()] = true
	}
	for _, prefix := range deletedPrefixes {
		name := prefix + ".log"
		if remainingNames[name] {
			t.Errorf("file %q should have been pruned but still exists", name)
		}
	}

}

// ─── Test helpers ─────────────────────────────────────────────────────────────

// mustMarshalString JSON-encodes a string (for JSON envelope tests).
func mustMarshalString(s string) []byte {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return []byte(b.String())
}

// TestWaitForEngineReady_ContextCancel verifies that if the context passed to
// waitForEngineReady is cancelled, it returns immediately with context.Canceled.
func TestWaitForEngineReady_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel it immediately

	engineDone := make(chan error, 1)
	err := pipeline.WaitForEngineReadyForTest(ctx, "http://127.0.0.1:12345/health", 10*time.Second, "", engineDone)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// TestWaitForEngineReady_EngineCrash verifies that if the engine exits before
// becoming ready, waitForEngineReady returns the exit error immediately.
func TestWaitForEngineReady_EngineCrash(t *testing.T) {
	ctx := context.Background()
	engineDone := make(chan error, 1)
	engineDone <- errors.New("oom-killed")

	err := pipeline.WaitForEngineReadyForTest(ctx, "http://127.0.0.1:12345/health", 10*time.Second, "", engineDone)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "engine exited before becoming ready") || !strings.Contains(err.Error(), "oom-killed") {
		t.Errorf("expected engine exited error wrapping 'oom-killed', got: %v", err)
	}
}
