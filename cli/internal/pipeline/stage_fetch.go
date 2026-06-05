package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bloc-org/bloc/internal/config"
	"github.com/bloc-org/bloc/internal/recipe"
)

// recipeIDRe is the allowlist for author and recipe name path segments.
// F-09: Prevents path traversal via crafted IDs like "../../etc/passwd/foo".
// Compiled once at package init — not per-call.
var recipeIDRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,99}$`)

// FetchRecipeStage resolves the RecipeID to a parsed *recipe.Recipe.
// Handles two cases:
//  1. Local file (.yaml/.yml or explicit path with separator)
//  2. Remote Hub recipe (author/recipe-name)
//
// Sets: state.Recipe, state.IsLocal
type FetchRecipeStage struct {
	// APIClient is the HTTP client used for remote fetches.
	// If nil, a default client with a 30-second timeout is used.
	APIClient *http.Client
}

func (s *FetchRecipeStage) Name() string { return "Fetching recipe" }

func (s *FetchRecipeStage) Run(_ context.Context, state *RunState) error {
	if isLocalRecipePath(state.RecipeID) {
		return s.loadLocal(state)
	}
	return s.fetchRemote(state)
}

func (s *FetchRecipeStage) loadLocal(state *RunState) error {
	r, err := recipe.ParseFileLocal(state.RecipeID)
	if err != nil {
		return fmt.Errorf("cannot parse local recipe: %w", err)
	}
	state.Recipe = r
	state.IsLocal = true
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Local file loaded: %s (%s)\n", state.RecipeID, r.Metadata.Name)
	return nil
}

func (s *FetchRecipeStage) fetchRemote(state *RunState) error {
	parts := strings.SplitN(state.RecipeID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid recipe ID %q — expected author/recipe-name or local path (.yaml)", state.RecipeID)
	}
	author, name := parts[0], parts[1]

	// F-09: Validate both path segments before interpolating into the URL.
	if !recipeIDRe.MatchString(author) {
		return fmt.Errorf("invalid author name %q — only alphanumeric, dash, dot and underscore allowed", author)
	}
	if !recipeIDRe.MatchString(name) {
		return fmt.Errorf("invalid recipe name %q — only alphanumeric, dash, dot and underscore allowed", name)
	}

	apiURL := fmt.Sprintf("%s/recipes/%s/%s",
		state.APIBase,
		url.PathEscape(author),
		url.PathEscape(name),
	)

	client := s.APIClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/yaml, application/json")
	req.Header.Set("User-Agent", "bloc-cli")

	if auth, authErr := config.LoadAuth(); authErr == nil && auth != nil && auth.Token != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("recipe %q not found — check spelling or visit https://bloc-theta.vercel.app/registry", state.RecipeID)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	// The Hub API may wrap the YAML in a JSON envelope {yaml_content: "..."}.
	var envelope struct {
		YAMLContent string `json:"yaml_content"`
	}
	var r *recipe.Recipe
	if json.Unmarshal(body, &envelope) == nil && envelope.YAMLContent != "" {
		r, err = recipe.Parse([]byte(envelope.YAMLContent))
	} else {
		r, err = recipe.Parse(body)
	}
	if err != nil {
		return fmt.Errorf("cannot parse recipe: %w", err)
	}

	state.Recipe = r
	state.IsLocal = false
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  %s — %s\n", r.Metadata.Name, shortDesc(r.Metadata.Description, 72))
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// isLocalRecipePath returns true when the ID looks like a local file path.
// SEC-10: only treats as local if it has a YAML extension or is an explicit
// path with a separator AND the file exists.
func isLocalRecipePath(path string) bool {
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return true
	}
	if strings.Contains(path, "/") || strings.Contains(path, "\\") {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// shortDesc truncates a description to maxLen characters, appending "…" if needed.
func shortDesc(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
