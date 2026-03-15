package node

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// NodeInfo describes a mesh node's identity and hardware capabilities.
type NodeInfo struct {
	ID             string
	Name           string
	Host           string
	Port           int
	Platform       string
	Arch           string
	CPUCores       int
	RAMTotalMB     int
	StorageTotalGB int
	IsHub          bool
	Status         string
}

// CapabilityScore returns a 0-60 score based on hardware resources.
// RAM: 0-40 points (linear scale, 40 for 16GB+)
// Storage: 0-20 points (linear, 20 for 500GB+)
func CapabilityScore(info NodeInfo) int {
	// RAM score: linear 0-40, capped at 16384 MB.
	ramScore := info.RAMTotalMB * 40 / 16384
	if ramScore > 40 {
		ramScore = 40
	}
	if ramScore < 0 {
		ramScore = 0
	}

	// Storage score: linear 0-20, capped at 500 GB.
	storageScore := info.StorageTotalGB * 20 / 500
	if storageScore > 20 {
		storageScore = 20
	}
	if storageScore < 0 {
		storageScore = 0
	}

	return ramScore + storageScore
}

// LoadOrCreateID reads a node UUID from path, or generates and persists a new one.
func LoadOrCreateID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	id := uuid.New().String()
	if err := os.WriteFile(path, []byte(id+"\n"), 0600); err != nil {
		return "", fmt.Errorf("mesh: write node id: %w", err)
	}
	return id, nil
}

// SystemSnapshot gathers current system information and returns a NodeInfo.
func SystemSnapshot() (*NodeInfo, error) {
	info := &NodeInfo{
		Platform: runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
		Status:   "online",
	}

	// Read RAM from /proc/meminfo.
	ramMB, err := readMemTotalMB()
	if err == nil {
		info.RAMTotalMB = ramMB
	}

	// Read disk space from df.
	storageGB, err := readStorageTotalGB()
	if err == nil {
		info.StorageTotalGB = storageGB
	}

	hostname, err := os.Hostname()
	if err == nil {
		info.Name = hostname
	}

	return info, nil
}

// readMemTotalMB reads MemTotal from /proc/meminfo.
func readMemTotalMB() (int, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.Atoi(fields[1])
				if err != nil {
					return 0, err
				}
				return kb / 1024, nil
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// readStorageTotalGB reads total disk space from df /.
func readStorageTotalGB() (int, error) {
	out, err := exec.Command("df", "--output=size", "-BG", "/").Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected df output")
	}
	sizeStr := strings.TrimSpace(lines[1])
	sizeStr = strings.TrimSuffix(sizeStr, "G")
	gb, err := strconv.Atoi(sizeStr)
	if err != nil {
		return 0, err
	}
	return gb, nil
}
