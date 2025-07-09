package config

import (
	"fmt"
	"os/exec"
	"strings"
)

// PathDetector handles detection of NetHack system paths
type PathDetector struct{}

// NewPathDetector creates a new PathDetector
func NewPathDetector() *PathDetector {
	return &PathDetector{}
}

// DetectNetHackPaths detects NetHack paths using the --showpaths flag
func (pd *PathDetector) DetectNetHackPaths() (*NetHackSystemPaths, error) {
	cmd := exec.Command("nethack", "--showpaths")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run nethack --showpaths: %w", err)
	}

	paths := &NetHackSystemPaths{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse variable playground locations
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			if strings.Contains(line, "hackdir") {
				paths.HackDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "leveldir") {
				paths.LevelDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "savedir") {
				paths.SaveDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "bonesdir") {
				paths.BonesDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "datadir") {
				paths.DataDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "scoredir") {
				paths.ScoreDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "lockdir") {
				paths.LockDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "configdir") {
				paths.ConfigDir = pd.parsePathValue(line)
			} else if strings.Contains(line, "troubledir") {
				paths.TroubleDir = pd.parsePathValue(line)
			}
		}

		// Parse fixed system paths
		if strings.Contains(line, "system configuration file") {
			if i := strings.Index(line, `"`); i != -1 {
				if j := strings.LastIndex(line, `"`); j > i {
					paths.SysConfFile = line[i+1 : j]
				}
			}
		} else if strings.Contains(line, "loadable symbols file") {
			if i := strings.Index(line, `"`); i != -1 {
				if j := strings.LastIndex(line, `"`); j > i {
					paths.SymbolsFile = line[i+1 : j]
				}
			}
		} else if strings.Contains(line, "Basic data files") {
			if i := strings.Index(line, `"`); i != -1 {
				if j := strings.LastIndex(line, `"`); j > i {
					paths.DataFile = line[i+1 : j]
				}
			}
		} else if strings.Contains(line, "personal configuration file") {
			if i := strings.Index(line, `"`); i != -1 {
				if j := strings.LastIndex(line, `"`); j > i {
					paths.UserConfig = line[i+1 : j]
				}
			}
		}
	}

	return paths, nil
}

// parsePathValue extracts path from format: [pathtype]="value" or [pathtype]="not set"
func (pd *PathDetector) parsePathValue(line string) string {
	if i := strings.Index(line, `="`); i != -1 {
		if j := strings.LastIndex(line, `"`); j > i {
			value := line[i+2 : j]
			if value == "not set" {
				return ""
			}
			return value
		}
	}
	return ""
}
