package team

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ReadSystemResources reads CPU temp, memory, and load average from /proc and /sys.
func ReadSystemResources() SystemResources {
	hostname, _ := os.Hostname()
	res := SystemResources{Hostname: hostname}

	// CPU temp from thermal zone (RPi and most Linux).
	if data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		s := strings.TrimSpace(string(data))
		if milliC, err := strconv.Atoi(s); err == nil {
			res.CPUTempC = float64(milliC) / 1000.0
		}
	}

	// Memory from /proc/meminfo.
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		res.MemoryTotalMB, res.MemoryUsedMB = parseMeminfo(string(data))
	}

	// Load average from /proc/loadavg.
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		res.LoadAvg1, res.LoadAvg5, res.LoadAvg15 = parseLoadAvg(string(data))
	}

	return res
}

// parseMeminfo extracts MemTotal and computes MemUsed from /proc/meminfo content.
func parseMeminfo(content string) (totalMB, usedMB int) {
	var memTotalKB, memAvailableKB int64
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotalKB = val
		case "MemAvailable:":
			memAvailableKB = val
		}
	}
	if memTotalKB > 0 {
		totalMB = int(memTotalKB / 1024)
		usedMB = int((memTotalKB - memAvailableKB) / 1024)
	}
	return totalMB, usedMB
}

// parseLoadAvg extracts 1, 5, 15 minute load averages from /proc/loadavg content.
func parseLoadAvg(content string) (avg1, avg5, avg15 float64) {
	fields := strings.Fields(strings.TrimSpace(content))
	if len(fields) >= 3 {
		avg1, _ = strconv.ParseFloat(fields[0], 64)
		avg5, _ = strconv.ParseFloat(fields[1], 64)
		avg15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return
}

// ReadTokenUsage reads 7-day aggregated per-agent token spend from guardian.db.
// Returns nil (not error) if the database doesn't exist or has no data.
func ReadTokenUsage() []AgentTokenUsage {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dbPath := filepath.Join(home, ".soul", "guardian.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT agent,
		       COALESCE(SUM(total_input), 0) + COALESCE(SUM(total_output), 0) AS tokens,
		       COALESCE(SUM(total_cost), 0) AS cost
		FROM daily_spend
		WHERE date >= date('now', '-7 days')
		GROUP BY agent
		ORDER BY cost DESC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []AgentTokenUsage
	for rows.Next() {
		var u AgentTokenUsage
		if err := rows.Scan(&u.Agent, &u.TokensUsed, &u.CostUSD); err != nil {
			continue
		}
		results = append(results, u)
	}

	if err := rows.Err(); err != nil {
		return nil
	}

	return results
}

// ListTmuxPanes returns a set of pane IDs that are alive in the given tmux session.
// Returns an empty map (not error) if tmux is not available or the session doesn't exist.
func ListTmuxPanes(session string) map[string]bool {
	result := make(map[string]bool)
	if session == "" {
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "tmux", "list-panes", "-t", session, "-F", "#{pane_id}").Output()
	if err != nil {
		return result
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result[line] = true
		}
	}

	return result
}
