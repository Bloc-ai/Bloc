package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var searchVRAM string
var searchPlatform string
var searchLimit int

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the Bloc recipe registry",
	Long: `Search for recipes on bloc-hub.com. Filter by VRAM or platform.

Examples:
  bloc search qwen3
  bloc search --vram 8GB --platform cuda
  bloc search deepseek --vram 16GB`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchVRAM, "vram", "", "Filter by minimum VRAM (e.g. 8GB, 16GB)")
	searchCmd.Flags().StringVar(&searchPlatform, "platform", "", "Filter by platform: cuda | metal | rocm | cpu")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Maximum number of results")
}

type recipeCard struct {
	Creator     string `json:"creator"`
	Name        string `json:"name"`
	Description string `json:"description"`
	BaseModel   string `json:"base_model"`
	MinVRAM     string `json:"min_vram"`
	Platform    string `json:"target_platform"`
	Stars       int    `json:"stars_count"`
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	apiURL := fmt.Sprintf("%s/recipes", hubAPIBase)
	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	if searchVRAM != "" {
		params.Set("min_vram", searchVRAM)
	}
	if searchPlatform != "" {
		params.Set("platform", searchPlatform)
	}
	params.Set("limit", fmt.Sprintf("%d", searchLimit))

	fullURL := apiURL + "?" + params.Encode()

	// P-01: Use shared package-level apiClient (see cmd/httpclient.go)
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "bloc-cli/"+Version)

	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var results []recipeCard
	if err := json.Unmarshal(body, &results); err != nil {
		return fmt.Errorf("unexpected response from API: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No recipes found. Try broadening your search.")
		fmt.Printf("Browse all recipes at https://bloc-hub.com/registry\n")
		return nil
	}

	// Print table
	fmt.Printf("\n%-40s %-12s %-8s %s\n", "RECIPE", "PLATFORM", "VRAM", "DESCRIPTION")
	fmt.Println(strings.Repeat("─", 100))
	for _, r := range results {
		id := r.Creator + "/" + r.Name
		if len(id) > 38 {
			id = id[:35] + "..."
		}
		desc := shortDesc(r.Description, 40)
		fmt.Printf("%-40s %-12s %-8s %s\n", id, r.Platform, r.MinVRAM, desc)
	}
	fmt.Printf("\n%d result(s). Deploy with: bloc deploy <author/recipe>\n", len(results))
	return nil
}
