package perf

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type startupLog struct {
	TS          string             `json:"ts"`
	TTY         string             `json:"tty"`
	PID         int                `json:"pid"`
	TermProgram string             `json:"term_program"`
	Bypasses    bypassInfo         `json:"bypasses"`
	MS          timingInfo         `json:"ms"`
	Tree        []treeEntry        `json:"tree"`
	Deferred    []treeEntry        `json:"deferred"`
	Sections    map[string]float64 `json:"sections"`
	Syssnap     *string            `json:"syssnap"`
	Zprof       *string            `json:"zprof"`
}

type bypassInfo struct {
	PathHelperCached bool `json:"path_helper_cached"`
	LocaleBypassed   bool `json:"locale_bypassed"`
}

type timingInfo struct {
	PreZshrc    float64 `json:"pre_zshrc"`
	SystemZsh   float64 `json:"system_zsh"`
	ZshenvSelf  float64 `json:"zshenv_self"`
	TimePrompt  float64 `json:"time_prompt"`
	FirstPrecmd float64 `json:"first_precmd"`
	TimeReady   float64 `json:"time_ready"`
}

type treeEntry struct {
	Depth int     `json:"depth"`
	Label string  `json:"label"`
	MS    float64 `json:"ms"`
	Tag   string  `json:"tag,omitempty"`
}

type startupRecord struct {
	Path    string
	ModTime time.Time
	Raw     []byte
	Log     startupLog
}

type zprofEntry struct {
	Name  string
	Total float64
	Self  float64
	Calls string
}

var zprofLinePattern = regexp.MustCompile(`^\s+[0-9]+\)`)

func Run(args []string) error {
	if len(args) == 0 {
		return runSummary(args)
	}

	switch args[0] {
	case "log":
		return runLog(args[1:])
	case "history":
		return runHistory(args[1:])
	case "arm-zprof":
		return runArmZprof(args[1:])
	case "rebuild-path-cache":
		return runRebuildPathCache(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		if strings.HasPrefix(args[0], "-") {
			return runSummary(args)
		}
		return fmt.Errorf("unknown perf command: %s", args[0])
	}
}

func runSummary(args []string) error {
	fs := flag.NewFlagSet("perf", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	useGlobal := fs.Bool("global", false, "use the newest log across all ttys")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse perf flags: %w", err)
	}

	record, err := selectRecord(*useGlobal)
	if err != nil {
		return err
	}

	var out strings.Builder
	fmt.Fprintf(&out, "shell startup: %.0f ms (time-to-prompt)\n", record.Log.MS.TimePrompt)
	out.WriteString(renderTree(record.Log.Tree, ""))

	zprofEntries := parseZprof(record.Log.Zprof)
	if len(zprofEntries) > 0 {
		out.WriteString(fmt.Sprintf("    └── [zprof: %d functions]\n", len(zprofEntries)))
		for idx, entry := range zprofEntries {
			branch := "├──"
			if idx == len(zprofEntries)-1 {
				branch = "└──"
			}
			callSuffix := "s"
			if entry.Calls == "1" {
				callSuffix = ""
			}
			out.WriteString(fmt.Sprintf("        %s %-28s %6.2f ms self   %6.2f ms total   (%s call%s)\n",
				branch, entry.Name, entry.Self, entry.Total, entry.Calls, callSuffix))
		}
	}

	if len(record.Log.Deferred) > 0 {
		out.WriteString("\ndeferred:\n")
		out.WriteString(renderTree(record.Log.Deferred, ""))
	}

	precmdGap := record.Log.MS.FirstPrecmd - record.Log.MS.TimePrompt
	ready := record.Log.MS.TimeReady
	if ready <= 0 {
		ready = record.Log.MS.FirstPrecmd
	}
	deferredTime := ready - record.Log.MS.FirstPrecmd
	if deferredTime < 0 {
		deferredTime = 0
	}
	out.WriteString("\ntimeline:\n")
	out.WriteString(fmt.Sprintf("  prompt visible:      %6.0f ms  (end of .zshrc)\n", record.Log.MS.TimePrompt))
	out.WriteString(fmt.Sprintf("  first precmd:        %6.0f ms  (+%.0f ms zsh hook overhead)\n", record.Log.MS.FirstPrecmd, precmdGap))
	out.WriteString(fmt.Sprintf("  shell interactive:   %6.0f ms  (+%.0f ms deferred tiers)\n", ready, deferredTime))

	fmt.Print(out.String())
	return nil
}

func runLog(args []string) error {
	fs := flag.NewFlagSet("perf log", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	useGlobal := fs.Bool("global", false, "use the newest log across all ttys")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse perf log flags: %w", err)
	}

	record, err := selectRecord(*useGlobal)
	if err != nil {
		return err
	}

	var obj any
	if err := json.Unmarshal(record.Raw, &obj); err != nil {
		return fmt.Errorf("decode startup log: %w", err)
	}
	pretty, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("format startup log: %w", err)
	}
	fmt.Println(string(pretty))
	return nil
}

func runHistory(args []string) error {
	fs := flag.NewFlagSet("perf history", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	slowOnly := fs.Bool("slow", false, "show only slow startups")
	showAll := fs.Bool("all", false, "show all startup logs")
	jsonOut := fs.Bool("json", false, "print logs as json")
	last := fs.Int("last", 50, "number of logs to show")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse perf history flags: %w", err)
	}

	records, err := loadRecords()
	if err != nil {
		return err
	}
	total := len(records)
	if !*showAll && *last > 0 && len(records) > *last {
		records = records[:*last]
	}

	if *slowOnly {
		filtered := make([]startupRecord, 0, len(records))
		for _, record := range records {
			if record.Log.MS.PreZshrc > 100 || record.Log.MS.TimePrompt > 300 {
				filtered = append(filtered, record)
			}
		}
		records = filtered
	}

	if *jsonOut {
		items := make([]json.RawMessage, 0, len(records))
		for _, record := range records {
			items = append(items, json.RawMessage(record.Raw))
		}
		pretty, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("encode history json: %w", err)
		}
		fmt.Println(string(pretty))
		return nil
	}

	fmt.Printf("%-18s %-10s %8s %8s %8s %8s %6s %6s  %s %s\n",
		"timestamp", "tty", "pre-rc", "prompt", "precmd", "ready", "ph", "locale", "by", "flags")
	fmt.Printf("%-18s %-10s %8s %8s %8s %8s %6s %6s\n",
		"---------", "---", "------", "------", "------", "-----", "--", "------")
	for _, record := range records {
		pathHelperMS := findTreeMS(record.Log.Tree, "path_helper", "path_helper_fork")
		localeMS := findTreeMS(record.Log.Tree, "combining/locale")
		flags := ""
		if record.Log.MS.PreZshrc > 100 {
			flags += "SLOW-PRE "
		}
		if record.Log.MS.TimePrompt > 300 {
			flags += "SLOW-PROMPT "
		}
		phFlag := "-"
		if record.Log.Bypasses.PathHelperCached {
			phFlag = "P"
		}
		locFlag := "-"
		if record.Log.Bypasses.LocaleBypassed {
			locFlag = "L"
		}
		by := phFlag + locFlag
		ts := record.Log.TS
		if len(ts) > 16 {
			ts = ts[:16]
		}
		fmt.Printf("%-18s %-10s %8.0f %8.0f %8.0f %8.0f %6.0f %6.0f  %s %s\n",
			ts,
			record.Log.TTY,
			record.Log.MS.PreZshrc,
			record.Log.MS.TimePrompt,
			record.Log.MS.FirstPrecmd,
			record.Log.MS.TimeReady,
			pathHelperMS,
			localeMS,
			by,
			flags,
		)
	}

	fmt.Printf("\n%d logs shown (of %d total).  --slow  --all  --last=N  --json\n", len(records), total)
	fmt.Println("by: P=path_helper cached  L=locale bypassed")
	return nil
}

func runArmZprof(args []string) error {
	if len(args) > 0 {
		for _, arg := range args {
			if arg == "-h" || arg == "--help" {
				fmt.Println("Usage: dots perf arm-zprof")
				return nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	path := filepath.Join(home, ".zsh_profile_next")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return fmt.Errorf("arm zprof: %w", err)
	}
	fmt.Println("zprof armed — open a new shell, then run zsh_perf to see function-level detail.")
	return nil
}

func runRebuildPathCache(args []string) error {
	if len(args) > 0 {
		for _, arg := range args {
			if arg == "-h" || arg == "--help" {
				fmt.Println("Usage: dots perf rebuild-path-cache")
				return nil
			}
		}
	}

	pathHelper := "/usr/libexec/path_helper"
	if _, err := os.Stat(pathHelper); err != nil {
		return fmt.Errorf("path_helper not found at %s", pathHelper)
	}
	output, err := exec.Command(pathHelper, "-s").CombinedOutput()
	if err != nil {
		return fmt.Errorf("run path_helper: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	cacheDir := filepath.Join(home, ".cache", "zsh_startup")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create path cache directory: %w", err)
	}
	cacheFile := filepath.Join(cacheDir, "path_cache.zsh")
	if err := os.WriteFile(cacheFile, output, 0o644); err != nil {
		return fmt.Errorf("write path cache: %w", err)
	}
	fmt.Printf("path cache rebuilt: %s\n", cacheFile)
	fmt.Printf("contents: %s\n", strings.TrimSpace(string(output)))
	return nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  dots perf")
	fmt.Println("  dots perf log [--global]")
	fmt.Println("  dots perf history [--slow] [--all] [--last=N] [--json]")
	fmt.Println("  dots perf arm-zprof")
	fmt.Println("  dots perf rebuild-path-cache")
}

func selectRecord(useGlobal bool) (startupRecord, error) {
	records, err := loadRecords()
	if err != nil {
		return startupRecord{}, err
	}
	if len(records) == 0 {
		return startupRecord{}, errors.New("no startup logs found")
	}
	if useGlobal {
		return records[0], nil
	}
	tty := currentTTYID()
	if tty == "" {
		return records[0], nil
	}
	for _, record := range records {
		if record.Log.TTY == tty {
			return record, nil
		}
	}
	return records[0], nil
}

func loadRecords() ([]startupRecord, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	pattern := filepath.Join(home, ".cache", "zsh_startup", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("list startup logs: %w", err)
	}

	records := make([]startupRecord, 0, len(paths))
	for _, path := range paths {
		if filepath.Base(path) == "latest.json" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var log startupLog
		if err := json.Unmarshal(raw, &log); err != nil {
			continue
		}
		records = append(records, startupRecord{
			Path:    path,
			ModTime: info.ModTime(),
			Raw:     raw,
			Log:     log,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].ModTime.Equal(records[j].ModTime) {
			return records[i].Path > records[j].Path
		}
		return records[i].ModTime.After(records[j].ModTime)
	})
	return records, nil
}

func currentTTYID() string {
	tty := strings.TrimSpace(os.Getenv("TTY"))
	if tty == "" {
		return ""
	}
	return strings.ReplaceAll(filepath.Base(tty), "/", "-")
}

func renderTree(entries []treeEntry, prefix string) string {
	if len(entries) == 0 {
		return ""
	}

	leftParts := make([]string, len(entries))
	maxLeft := 0

	for idx, entry := range entries {
		isLastSibling := true
		for lookIdx := idx + 1; lookIdx < len(entries); lookIdx++ {
			lookDepth := entries[lookIdx].Depth
			if lookDepth == entry.Depth {
				isLastSibling = false
				break
			}
			if lookDepth < entry.Depth {
				break
			}
		}

		var indent strings.Builder
		for ancestorDepth := 0; ancestorDepth < entry.Depth; ancestorDepth++ {
			ancestorHasMore := false
			for lookIdx := idx + 1; lookIdx < len(entries); lookIdx++ {
				lookDepth := entries[lookIdx].Depth
				if lookDepth == ancestorDepth {
					ancestorHasMore = true
					break
				}
				if lookDepth < ancestorDepth {
					break
				}
			}
			if ancestorHasMore {
				indent.WriteString("│   ")
			} else {
				indent.WriteString("    ")
			}
		}

		branch := "├──"
		if isLastSibling {
			branch = "└──"
		}
		left := prefix + indent.String() + branch + " " + entry.Label
		leftParts[idx] = left
		if width := visualWidth(left); width > maxLeft {
			maxLeft = width
		}
	}

	padTarget := maxLeft + 2
	var out strings.Builder
	for idx, entry := range entries {
		left := leftParts[idx]
		pad := padTarget - visualWidth(left)
		if pad < 1 {
			pad = 1
		}
		suffix := ""
		if entry.Tag != "" {
			suffix = fmt.Sprintf("  (%s)", entry.Tag)
		}
		out.WriteString(fmt.Sprintf("%s%s%5.1f ms%s\n", left, strings.Repeat(" ", pad), entry.MS, suffix))
	}
	return out.String()
}

func visualWidth(value string) int {
	replacer := strings.NewReplacer("│", " ", "├", " ", "└", " ", "─", " ")
	return utf8.RuneCountInString(replacer.Replace(value))
}

func parseZprof(raw *string) []zprofEntry {
	if raw == nil || *raw == "" {
		return nil
	}

	lines := strings.Split(*raw, "\n")
	sepCount := 0
	entries := make([]zprofEntry, 0)
	for _, line := range lines {
		if strings.HasPrefix(line, "--") {
			sepCount++
			if sepCount >= 2 {
				break
			}
			continue
		}
		if sepCount != 1 || !zprofLinePattern.MatchString(line) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		total, err := parseFloat(fields[2])
		if err != nil {
			continue
		}
		self, err := parseFloat(fields[5])
		if err != nil {
			continue
		}
		entries = append(entries, zprofEntry{
			Name:  strings.Join(fields[8:], " "),
			Total: total,
			Self:  self,
			Calls: fields[1],
		})
	}
	return entries
}

func parseFloat(value string) (float64, error) {
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return 0, err
	}
	return parsed, nil
}

func findTreeMS(entries []treeEntry, labels ...string) float64 {
	for _, entry := range entries {
		for _, label := range labels {
			if entry.Label == label {
				return entry.MS
			}
		}
	}
	return 0
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
