package hardware

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// SystemInfo holds detected hardware specs.
type SystemInfo struct {
	OS         string
	Arch       string
	TotalRAMMB int64
	GPUs       []GPUInfo
	Platform   string // "cuda" | "metal" | "rocm" | "cpu"
}

type GPUInfo struct {
	Name    string
	VRAMMB  int64
	IsApple bool // Apple Silicon unified memory
}

// P-11: Cache hardware probe result for the process lifetime.
// Hardware doesn't change mid-run, and system_profiler can take 500ms–2s on macOS.
var (
	cachedSystemInfo    *SystemInfo
	cachedSystemInfoErr error
	systemInfoOnce      sync.Once
)

// Probe detects available hardware. Result is cached after first call (P-11).
func Probe() (*SystemInfo, error) {
	systemInfoOnce.Do(func() {
		cachedSystemInfo, cachedSystemInfoErr = probe()
	})
	return cachedSystemInfo, cachedSystemInfoErr
}

func probe() (*SystemInfo, error) {
	info := &SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "darwin":
		return probeMacOS(info)
	case "linux":
		return probeLinux(info)
	default:
		return info, nil
	}
}

func probeMacOS(info *SystemInfo) (*SystemInfo, error) {
	// F-13: Use absolute path for sysctl — not susceptible to PATH hijack.
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output()
	if err == nil {
		val := strings.TrimSpace(string(out))
		if bytes, err := strconv.ParseInt(val, 10, 64); err == nil {
			info.TotalRAMMB = bytes / 1024 / 1024
		}
	}

	// Apple Silicon: unified memory is both RAM and VRAM.
	// F-13: Use absolute path for system_profiler.
	ioregOut, err := exec.Command("/usr/bin/system_profiler", "SPDisplaysDataType", "-json").Output()
	if err == nil {
		// Simple regex extraction
		nameRe := regexp.MustCompile(`"spdisplays_vendor"\s*:\s*"([^"]+)"`)
		vramRe := regexp.MustCompile(`"spdisplays_vram"\s*:\s*"([^"]+)"`)

		names := nameRe.FindAllStringSubmatch(string(ioregOut), -1)
		vrams := vramRe.FindAllStringSubmatch(string(ioregOut), -1)

		isAppleSilicon := info.Arch == "arm64"

		if isAppleSilicon {
			// Unified memory: VRAM = total RAM
			info.Platform = "metal"
			gpu := GPUInfo{
				Name:    "Apple Silicon (Unified Memory)",
				VRAMMB:  info.TotalRAMMB,
				IsApple: true,
			}
			if len(names) > 0 {
				gpu.Name = names[0][1]
			}
			info.GPUs = append(info.GPUs, gpu)
		} else {
			info.Platform = "metal"
			for i, nameMatch := range names {
				gpu := GPUInfo{Name: nameMatch[1]}
				if i < len(vrams) {
					gpu.VRAMMB = parseVRAMString(vrams[i][1])
				}
				info.GPUs = append(info.GPUs, gpu)
			}
		}
	}

	return info, nil
}

func probeLinux(info *SystemInfo) (*SystemInfo, error) {
	// F-13: Use absolute paths for well-known system binaries.
	// grep and /proc/meminfo are standard on all Linux distros.
	memOut, err := exec.Command("/bin/grep", "MemTotal", "/proc/meminfo").Output()
	if err == nil {
		re := regexp.MustCompile(`(\d+)\s+kB`)
		if m := re.FindStringSubmatch(string(memOut)); len(m) > 1 {
			kb, _ := strconv.ParseInt(m[1], 10, 64)
			info.TotalRAMMB = kb / 1024
		}
	}

	// NVIDIA GPUs via nvidia-smi.
	// F-13: Use exec.LookPath to find and verify the binary before executing.
	if nvPath, err := exec.LookPath("nvidia-smi"); err == nil {
		nvOut, err := exec.Command(nvPath, "--query-gpu=name,memory.total", "--format=csv,noheader,nounits").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(nvOut)), "\n")
			for _, line := range lines {
				parts := strings.Split(line, ", ")
				if len(parts) == 2 {
					mb, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
					info.GPUs = append(info.GPUs, GPUInfo{
						Name:   strings.TrimSpace(parts[0]),
						VRAMMB: mb,
					})
				}
			}
			if len(info.GPUs) > 0 {
				info.Platform = "cuda"
			}
		}
	}

	// AMD/ROCm fallback via rocm-smi.
	// F-13: Same LookPath verification.
	if len(info.GPUs) == 0 {
		if rocmPath, err := exec.LookPath("rocm-smi"); err == nil {
			rocmOut, err := exec.Command(rocmPath, "--showmeminfo", "vram", "--json").Output()
			if err == nil && len(rocmOut) > 0 {
				info.Platform = "rocm"
				info.GPUs = append(info.GPUs, GPUInfo{Name: "AMD GPU (ROCm)", VRAMMB: parseROCmVRAM(string(rocmOut))})
			}
		}
	}

	// CPU-only fallback
	if len(info.GPUs) == 0 {
		info.Platform = "cpu"
	}

	return info, nil
}

// TotalVRAMMB returns the combined VRAM of all detected GPUs.
func (s *SystemInfo) TotalVRAMMB() int64 {
	var total int64
	for _, g := range s.GPUs {
		total += g.VRAMMB
	}
	return total
}

// CheckVRAMRequirement validates the recipe's min_vram against detected VRAM.
// Returns (ok, detectedGB, requiredGB).
func (s *SystemInfo) CheckVRAMRequirement(minVRAM string) (bool, float64, float64) {
	required := parseVRAMString(minVRAM)
	detected := s.TotalVRAMMB()
	return detected >= required,
		float64(detected) / 1024,
		float64(required) / 1024
}

// Summary returns a human-readable one-liner for display.
func (s *SystemInfo) Summary() string {
	if len(s.GPUs) == 0 {
		return fmt.Sprintf("%s/%s · %d GB RAM · No GPU detected (CPU-only mode)",
			s.OS, s.Arch, s.TotalRAMMB/1024)
	}
	gpuNames := make([]string, len(s.GPUs))
	for i, g := range s.GPUs {
		gpuNames[i] = fmt.Sprintf("%s (%.0f GB)", g.Name, float64(g.VRAMMB)/1024)
	}
	return fmt.Sprintf("%s/%s · %d GB RAM · %s",
		s.OS, s.Arch, s.TotalRAMMB/1024, strings.Join(gpuNames, " + "))
}

// parseVRAMString converts strings like "8GB", "8 GB", "8192" (MB) to MB.
func parseVRAMString(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(GB|MB|G|M)?`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	val, _ := strconv.ParseFloat(m[1], 64)
	switch m[2] {
	case "GB", "G":
		return int64(val * 1024)
	case "MB", "M":
		return int64(val)
	default:
		// bare number: assume MB
		return int64(val)
	}
}

// parseROCmVRAM is a simplified rocm-smi JSON parser.
func parseROCmVRAM(jsonStr string) int64 {
	re := regexp.MustCompile(`"VRAM Total Memory \(B\)"\s*:\s*"(\d+)"`)
	m := re.FindStringSubmatch(jsonStr)
	if len(m) < 2 {
		return 0
	}
	bytes, _ := strconv.ParseInt(m[1], 10, 64)
	return bytes / 1024 / 1024
}
