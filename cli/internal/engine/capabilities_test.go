package engine_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── FlagSpec.Emit tests ──────────────────────────────────────────────────────

func TestFlagSpec_Emit_BoolOnOff(t *testing.T) {
	f := engine.FlagSpec{
		Name:       "--flash-attn",
		TakesValue: true,
		ValueType:  engine.ValueTypeBoolOnOff,
	}
	got := f.Emit("on")
	want := []string{"--flash-attn", "on"}
	assertSliceEqual(t, got, want)
}

func TestFlagSpec_Emit_BoolOnOff_Off(t *testing.T) {
	f := engine.FlagSpec{
		Name:       "--flash-attn",
		TakesValue: true,
		ValueType:  engine.ValueTypeBoolOnOff,
	}
	got := f.Emit("off")
	want := []string{"--flash-attn", "off"}
	assertSliceEqual(t, got, want)
}

func TestFlagSpec_Emit_BoolImplicit(t *testing.T) {
	f := engine.FlagSpec{
		Name:       "-fa",
		TakesValue: false,
		ValueType:  engine.ValueTypeBoolImplicit,
	}
	// Value is ignored for implicit booleans — flag alone enables the feature.
	got := f.Emit("")
	want := []string{"-fa"}
	assertSliceEqual(t, got, want)
}

func TestFlagSpec_Emit_IntFlag(t *testing.T) {
	f := engine.FlagSpec{
		Name:       "--ctx-size",
		TakesValue: true,
		ValueType:  engine.ValueTypeInt,
	}
	got := f.Emit("4096")
	want := []string{"--ctx-size", "4096"}
	assertSliceEqual(t, got, want)
}

func TestFlagSpec_Emit_StringFlag(t *testing.T) {
	f := engine.FlagSpec{
		Name:       "--model",
		TakesValue: true,
		ValueType:  engine.ValueTypeString,
	}
	got := f.Emit("/models/qwen.gguf")
	want := []string{"--model", "/models/qwen.gguf"}
	assertSliceEqual(t, got, want)
}

// ─── BuildCapabilities — flash_attn ──────────────────────────────────────────

func TestBuildCapabilities_FlashAttn_NewForm(t *testing.T) {
	// Current llama.cpp builds use --flash-attn (bool_on_off)
	rawFlags := map[string]engine.FlagSpec{
		"--flash-attn": {Name: "--flash-attn", TakesValue: true, ValueType: engine.ValueTypeBoolOnOff},
		"--ctx-size":   {Name: "--ctx-size", TakesValue: true, ValueType: engine.ValueTypeInt},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)

	if !caps.HasFeature("flash_attn") {
		t.Error("expected flash_attn feature to be detected from --flash-attn")
	}
	spec, ok := caps.FlagFor("flash_attn")
	if !ok {
		t.Fatal("FlagFor(flash_attn) returned false")
	}
	if spec.Name != "--flash-attn" {
		t.Errorf("FlagFor(flash_attn).Name = %q, want --flash-attn", spec.Name)
	}
	if spec.ValueType != engine.ValueTypeBoolOnOff {
		t.Errorf("FlagFor(flash_attn).ValueType = %q, want bool_on_off", spec.ValueType)
	}
}

func TestBuildCapabilities_FlashAttn_LegacyForm(t *testing.T) {
	// Older llama.cpp builds used -fa (bool_implicit)
	rawFlags := map[string]engine.FlagSpec{
		"-fa":        {Name: "-fa", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
		"--ctx-size": {Name: "--ctx-size", TakesValue: true, ValueType: engine.ValueTypeInt},
	}
	caps := engine.BuildCapabilities("llama-server", "b3500", rawFlags)

	if !caps.HasFeature("flash_attn") {
		t.Error("expected flash_attn feature to be detected from -fa (legacy)")
	}
	spec, ok := caps.FlagFor("flash_attn")
	if !ok {
		t.Fatal("FlagFor(flash_attn) returned false for legacy -fa")
	}
	if spec.Name != "-fa" {
		t.Errorf("FlagFor(flash_attn).Name = %q, want -fa", spec.Name)
	}
	if spec.ValueType != engine.ValueTypeBoolImplicit {
		t.Errorf("FlagFor(flash_attn).ValueType = %q, want bool_implicit", spec.ValueType)
	}
}

func TestBuildCapabilities_FlashAttn_Missing(t *testing.T) {
	// Binary that supports neither form
	rawFlags := map[string]engine.FlagSpec{
		"--ctx-size": {Name: "--ctx-size", TakesValue: true, ValueType: engine.ValueTypeInt},
	}
	caps := engine.BuildCapabilities("llama-server", "b3000", rawFlags)

	if caps.HasFeature("flash_attn") {
		t.Error("expected flash_attn NOT to be detected when neither --flash-attn nor -fa is present")
	}
	if _, ok := caps.FlagFor("flash_attn"); ok {
		t.Error("FlagFor(flash_attn) should return false when feature is absent")
	}
}

func TestBuildCapabilities_FlashAttn_NewFormPreferredOverLegacy(t *testing.T) {
	// If both --flash-attn and -fa are present (unusual but possible),
	// --flash-attn (new form) should win because it is checked first.
	rawFlags := map[string]engine.FlagSpec{
		"--flash-attn": {Name: "--flash-attn", TakesValue: true, ValueType: engine.ValueTypeBoolOnOff},
		"-fa":          {Name: "-fa", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
	}
	caps := engine.BuildCapabilities("llama-server", "b4000", rawFlags)

	spec, ok := caps.FlagFor("flash_attn")
	if !ok {
		t.Fatal("FlagFor(flash_attn) returned false")
	}
	if spec.Name != "--flash-attn" {
		t.Errorf("new form should win: got spec.Name = %q, want --flash-attn", spec.Name)
	}
}

// ─── BuildCapabilities — speculative decoding ─────────────────────────────────

func TestBuildCapabilities_SpecDecoding_CurrentFlagName(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--spec-type": {Name: "--spec-type", TakesValue: true, ValueType: engine.ValueTypeString},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)

	if !caps.HasFeature("speculative_decoding") {
		t.Error("expected speculative_decoding feature from --spec-type")
	}
	spec, ok := caps.FlagFor("spec_type")
	if !ok {
		t.Fatal("FlagFor(spec_type) should return true")
	}
	if spec.Name != "--spec-type" {
		t.Errorf("FlagFor(spec_type).Name = %q, want --spec-type", spec.Name)
	}
}

func TestBuildCapabilities_SpecDecoding_LegacyFlagName(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--speculative-type": {Name: "--speculative-type", TakesValue: true, ValueType: engine.ValueTypeString},
	}
	caps := engine.BuildCapabilities("llama-server", "b3700", rawFlags)

	if !caps.HasFeature("speculative_decoding") {
		t.Error("expected speculative_decoding from --speculative-type (legacy)")
	}
}

func TestBuildCapabilities_SpecDecoding_Missing(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b3000", map[string]engine.FlagSpec{})
	if caps.HasFeature("speculative_decoding") {
		t.Error("expected speculative_decoding NOT detected when no spec flags present")
	}
}

// ─── BuildCapabilities — kv_cache_types ──────────────────────────────────────

func TestBuildCapabilities_KVCacheTypes_BothPresent(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"-ctk": {Name: "-ctk", TakesValue: true, ValueType: engine.ValueTypeString},
		"-ctv": {Name: "-ctv", TakesValue: true, ValueType: engine.ValueTypeString},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)

	if !caps.HasFeature("kv_cache_types") {
		t.Error("expected kv_cache_types when both -ctk and -ctv are present")
	}
	if _, ok := caps.FlagFor("kv_cache_type_k"); !ok {
		t.Error("FlagFor(kv_cache_type_k) should be set")
	}
	if _, ok := caps.FlagFor("kv_cache_type_v"); !ok {
		t.Error("FlagFor(kv_cache_type_v) should be set")
	}
}

func TestBuildCapabilities_KVCacheTypes_OnlyKPresent(t *testing.T) {
	// Require BOTH — only K present means feature is absent
	rawFlags := map[string]engine.FlagSpec{
		"-ctk": {Name: "-ctk", TakesValue: true, ValueType: engine.ValueTypeString},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)

	if caps.HasFeature("kv_cache_types") {
		t.Error("kv_cache_types should NOT be detected when only -ctk is present (requires both -ctk and -ctv)")
	}
}

// ─── BuildCapabilities — other features ──────────────────────────────────────

func TestBuildCapabilities_Jinja(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--jinja": {Name: "--jinja", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)
	if !caps.HasFeature("jinja") {
		t.Error("expected jinja feature from --jinja")
	}
}

func TestBuildCapabilities_MoECPUOffload(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--n-cpu-moe": {Name: "--n-cpu-moe", TakesValue: true, ValueType: engine.ValueTypeInt},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)
	if !caps.HasFeature("moe_cpu_offload") {
		t.Error("expected moe_cpu_offload from --n-cpu-moe")
	}
}

func TestBuildCapabilities_MMap_MLock(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--no-mmap": {Name: "--no-mmap", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
		"--mlock":   {Name: "--mlock", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)
	if !caps.HasFeature("mmap") {
		t.Error("expected mmap from --no-mmap")
	}
	if !caps.HasFeature("mlock") {
		t.Error("expected mlock from --mlock")
	}
}

func TestBuildCapabilities_MultiGPU(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--split-mode":  {Name: "--split-mode", TakesValue: true, ValueType: engine.ValueTypeString},
		"--tensor-split": {Name: "--tensor-split", TakesValue: true, ValueType: engine.ValueTypeString},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)
	if !caps.HasFeature("split_mode") {
		t.Error("expected split_mode from --split-mode")
	}
	if !caps.HasFeature("tensor_split") {
		t.Error("expected tensor_split from --tensor-split")
	}
}

func TestBuildCapabilities_EmptyFlags_NoFeatures(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "", map[string]engine.FlagSpec{})
	for _, feature := range []string{
		"flash_attn", "speculative_decoding", "kv_cache_types",
		"jinja", "moe_cpu_offload", "mmap", "mlock", "split_mode", "tensor_split",
	} {
		if caps.HasFeature(feature) {
			t.Errorf("expected feature %q to be absent for empty rawFlags, but HasFeature returned true", feature)
		}
	}
}

// ─── CapabilitySet.RequireFeatures ───────────────────────────────────────────

func TestCapabilitySet_RequireFeatures_AllPresent(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--flash-attn": {Name: "--flash-attn", TakesValue: true, ValueType: engine.ValueTypeBoolOnOff},
		"--jinja":      {Name: "--jinja", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)

	if err := caps.RequireFeatures("flash_attn", "jinja"); err != nil {
		t.Errorf("expected no error when all features present, got: %v", err)
	}
}

func TestCapabilitySet_RequireFeatures_OneMissing(t *testing.T) {
	rawFlags := map[string]engine.FlagSpec{
		"--jinja": {Name: "--jinja", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
	}
	caps := engine.BuildCapabilities("llama-server", "b4100", rawFlags)

	err := caps.RequireFeatures("flash_attn", "jinja")
	if err == nil {
		t.Fatal("expected error when flash_attn is missing, got nil")
	}
	if !strings.Contains(err.Error(), "flash_attn") {
		t.Errorf("error should mention the missing feature; got: %v", err)
	}
}

func TestCapabilitySet_RequireFeatures_AllMissing_ListsAll(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b3000", map[string]engine.FlagSpec{})

	err := caps.RequireFeatures("flash_attn", "jinja", "kv_cache_types")
	if err == nil {
		t.Fatal("expected error for all missing features")
	}
	for _, feature := range []string{"flash_attn", "jinja", "kv_cache_types"} {
		if !strings.Contains(err.Error(), feature) {
			t.Errorf("error should mention %q; full error: %v", feature, err)
		}
	}
}

func TestCapabilitySet_RequireFeatures_Empty_NoError(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4100", map[string]engine.FlagSpec{})
	if err := caps.RequireFeatures(); err != nil {
		t.Errorf("RequireFeatures() with no args should return nil, got: %v", err)
	}
}

// ─── CapabilitySet metadata ───────────────────────────────────────────────────

func TestBuildCapabilities_MetadataPreserved(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4100", map[string]engine.FlagSpec{})
	if caps.EngineName != "llama-server" {
		t.Errorf("EngineName = %q, want llama-server", caps.EngineName)
	}
	if caps.Version != "b4100" {
		t.Errorf("Version = %q, want b4100", caps.Version)
	}
}

// ─── Registry tests ───────────────────────────────────────────────────────────

// fakeEngine is a minimal Engine implementation for registry tests.
type fakeEngine struct{ name string }

func (e *fakeEngine) Name() string                              { return e.name }
func (e *fakeEngine) Capabilities(_ context.Context) (*engine.CapabilitySet, error) {
	return engine.BuildCapabilities(e.name, "1.0", map[string]engine.FlagSpec{}), nil
}
func (e *fakeEngine) BuildArgs(_ *engine.CapabilitySet, _ recipe.EngineConfig) ([]string, error) {
	return nil, nil
}
func (e *fakeEngine) NewSupervisor(_ engine.LaunchConfig) (*process.Supervisor, error) {
	return nil, fmt.Errorf("not implemented in fakeEngine")
}
func (e *fakeEngine) OfferInstall() bool { return false }

// TestRegistry_ResolveUnknownEngine verifies that Resolve() returns an error
// listing available engines when the recipe requests an unknown engine name.
func TestRegistry_ResolveUnknownEngine(t *testing.T) {
	// Deliberately not registering "onnxruntime" — it should produce an error.
	r := &recipe.Recipe{
		Schema:   "bloc/v1",
		Metadata: recipe.Metadata{Name: "test"},
		Engine:   recipe.Engine{Name: "onnxruntime"},
	}
	_, err := engine.Resolve(r)
	if err == nil {
		t.Fatal("expected error for unknown engine 'onnxruntime'")
	}
	if !strings.Contains(err.Error(), "onnxruntime") {
		t.Errorf("error should mention the unknown engine name; got: %v", err)
	}
}

// TestRegistry_NormalizeAlias verifies that "llama-cpp" normalizes
// to "llama.cpp" before the registry lookup. When no engine is registered the
// error must not quote the raw alias — it should quote the normalized canonical name.
func TestRegistry_NormalizeAlias_LlamaCpp(t *testing.T) {
	r := &recipe.Recipe{
		Schema:   "bloc/v1",
		Metadata: recipe.Metadata{Name: "test"},
		Engine:   recipe.Engine{Name: "llama-cpp"},
	}
	_, err := engine.Resolve(r)
	// Resolve must produce an error (no llamacpp engine imported in this test binary).
	// The important property: the error message must NOT contain "llama-cpp" (the alias),
	// because the engine package normalizes it to "llama.cpp" before lookups.
	if err == nil {
		// llamacpp was imported and is registered — alias resolved correctly.
		return
	}
	if strings.Contains(err.Error(), `"llama-cpp"`) {
		t.Errorf("error should use the normalized canonical name 'llama.cpp', not the alias 'llama-cpp'.\nGot: %v", err)
	}
}


// TestRegistry_RegisterPanicsOnDuplicate verifies that calling Register() with
// the same name twice panics — this protects against accidental double-import.
func TestRegistry_RegisterPanicsOnDuplicate(t *testing.T) {
	// We register under a unique name scoped to this test run to avoid collision
	// with other tests that may run in the same binary.
	testName := "test-engine-for-duplicate-check"
	engine.Register(testName, func(_ *recipe.Recipe) (engine.Engine, error) {
		return &fakeEngine{name: testName}, nil
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate Register(), got none")
		}
	}()
	// This second registration should panic.
	engine.Register(testName, func(_ *recipe.Recipe) (engine.Engine, error) {
		return &fakeEngine{name: testName}, nil
	})
}

// TestRegistry_ConcurrentResolveSafe verifies that concurrent calls to Resolve()
// do not race. Run with -race.
func TestRegistry_ConcurrentResolveSafe(t *testing.T) {
	r := &recipe.Recipe{
		Schema:   "bloc/v1",
		Metadata: recipe.Metadata{Name: "test"},
		Engine:   recipe.Engine{Name: "unknown-for-concurrent-test"},
	}
	var wg sync.WaitGroup
	const goroutines = 20
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			engine.Resolve(r) //nolint:errcheck — we're testing for races, not errors
		}()
	}
	wg.Wait()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func assertSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("slice length: got %d, want %d\n  got:  %v\n  want: %v", len(got), len(want), got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("slice[%d]: got %q, want %q\n  got:  %v\n  want: %v", i, got[i], want[i], got, want)
		}
	}
}
