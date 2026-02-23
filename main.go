package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Config holds values parsed from config.toml.
type Config struct {
	NASIP           string
	NASUser         string
	MountHome       string
	MountDocs       string
	MountPics       string
	DefaultSyncDirs []string
	DocsSubfolder   string
}

// --- Colors ---

const (
	green  = "\033[0;32m"
	yellow = "\033[1;33m"
	red    = "\033[0;31m"
	reset  = "\033[0m"
)

func info(msg string)  { fmt.Printf("%s[+]%s %s\n", green, reset, msg) }
func warn(msg string)  { fmt.Printf("%s[!]%s %s\n", yellow, reset, msg) }
func fail(msg string)  { fmt.Printf("%s[x]%s %s\n", red, reset, msg) }
func fatal(msg string) { fail(msg); os.Exit(1) }

// --- Config parsing ---

func findConfig() string {
	// Check working directory first, then executable directory.
	if abs, err := filepath.Abs("config.toml"); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	if exePath, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exePath), "config.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func loadConfig() Config {
	path := findConfig()
	if path == "" {
		fatal("Cannot find config.toml in working directory or next to the binary.\nCopy config.toml.example to config.toml and fill in your values.")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fatal("Cannot read " + path + ": " + err.Error())
	}

	kv := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		kv[key] = val
	}

	return Config{
		NASIP:           kv["nas_ip"],
		NASUser:         kv["nas_user"],
		MountHome:       kv["mount_home"],
		MountDocs:       kv["mount_docs"],
		MountPics:       kv["mount_pics"],
		DefaultSyncDirs: parseTOMLArray(kv["default_sync_dirs"]),
		DocsSubfolder:   kv["docs_subfolder"],
	}
}

func parseTOMLArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, "\"")
		if item != "" {
			result = append(result, expandHome(item))
		}
	}
	return result
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// --- Command execution ---

func run(name string, args ...string) error {
	fmt.Printf("%s[run]%s %s %s\n", yellow, reset, name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

// --- Prompts ---

func prompt(label string) string {
	fmt.Printf("%s: ", label)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptSecret(label string) string {
	fmt.Printf("%s: ", label)

	// Disable echo
	fd := syscall.Stdin
	var oldState syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&oldState)), 0, 0, 0); err == 0 {
		newState := oldState
		newState.Lflag &^= syscall.ECHO
		syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
			uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&newState)), 0, 0, 0)
		defer func() {
			syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
				uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&oldState)), 0, 0, 0)
			fmt.Println()
		}()
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

// --- NAS checks ---

func isNASReachable(cfg Config) bool {
	return runSilent("ping", "-c", "1", "-W", "2", cfg.NASIP) == nil
}

func isMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}

// --- Wifi ---

func connectWifi(cfg Config) {
	if isNASReachable(cfg) {
		info("NAS already reachable at " + cfg.NASIP)
		return
	}

	warn("NAS not reachable. Need to connect wifi to .1 subnet.")
	fmt.Println("Available networks:")
	run("nmcli", "dev", "wifi", "list")
	fmt.Println()

	ssid := prompt("Enter SSID for NAS network")
	pass := promptSecret("Enter password for '" + ssid + "'")

	info("Connecting wlan1 to '" + ssid + "'...")
	if err := run("nmcli", "dev", "wifi", "connect", ssid, "password", pass, "ifname", "wlan1"); err != nil {
		fatal("Failed to connect wifi: " + err.Error())
	}

	// Give the connection a moment to establish
	time.Sleep(2 * time.Second)

	if !isNASReachable(cfg) {
		fatal("Still cannot reach NAS after connecting wifi")
	}
	info("NAS reachable at " + cfg.NASIP)
}

// --- Mount / Unmount ---

func mountShares(cfg Config) {
	if isMounted(cfg.MountHome) && isMounted(cfg.MountDocs) && isMounted(cfg.MountPics) {
		info("Shares already mounted")
		return
	}

	uid := fmt.Sprintf("%d", os.Getuid())
	gid := fmt.Sprintf("%d", os.Getgid())

	run("sudo", "mkdir", "-p", cfg.MountHome, cfg.MountDocs, cfg.MountPics)

	if !isMounted(cfg.MountHome) {
		info("Mounting home share...")
		opts := fmt.Sprintf("username=%s,vers=2.0,uid=%s,gid=%s", cfg.NASUser, uid, gid)
		if err := run("sudo", "mount", "-t", "cifs",
			"//"+cfg.NASIP+"/home", cfg.MountHome,
			"-o", opts); err != nil {
			fatal("Failed to mount home share: " + err.Error())
		}
	}

	if !isMounted(cfg.MountDocs) {
		info("Mounting Documents share...")
		opts := fmt.Sprintf("username=%s,vers=2.0,sec=ntlmssp,uid=%s,gid=%s", cfg.NASUser, uid, gid)
		if err := run("sudo", "mount", "-t", "cifs",
			"//"+cfg.NASIP+"/Documents", cfg.MountDocs,
			"-o", opts); err != nil {
			fatal("Failed to mount Documents share: " + err.Error())
		}
	}

	if !isMounted(cfg.MountPics) {
		info("Mounting Pictures share...")
		opts := fmt.Sprintf("username=%s,vers=2.0,sec=ntlmssp,uid=%s,gid=%s", cfg.NASUser, uid, gid)
		if err := run("sudo", "mount", "-t", "cifs",
			"//"+cfg.NASIP+"/Pictures", cfg.MountPics,
			"-o", opts); err != nil {
			fatal("Failed to mount Pictures share: " + err.Error())
		}
	}

	info("Shares mounted")
}

func unmountShares(cfg Config) {
	if isMounted(cfg.MountHome) {
		info("Unmounting " + cfg.MountHome + "...")
		if err := run("sudo", "umount", cfg.MountHome); err != nil {
			fail("Failed to unmount " + cfg.MountHome + ": " + err.Error())
		}
	}
	if isMounted(cfg.MountDocs) {
		info("Unmounting " + cfg.MountDocs + "...")
		if err := run("sudo", "umount", cfg.MountDocs); err != nil {
			fail("Failed to unmount " + cfg.MountDocs + ": " + err.Error())
		}
	}
	if isMounted(cfg.MountPics) {
		info("Unmounting " + cfg.MountPics + "...")
		if err := run("sudo", "umount", cfg.MountPics); err != nil {
			fail("Failed to unmount " + cfg.MountPics + ": " + err.Error())
		}
	}
	info("Shares unmounted")
}

// --- Sync ---

func syncDirectory(src string, dst string, dryRun bool) SyncResult {
	if !dryRun {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			fail("Cannot create destination " + dst + ": " + err.Error())
			return SyncResult{Err: err}
		}
	}

	info("Syncing " + src + " -> " + dst)

	if _, err := exec.LookPath("rsync"); err == nil {
		args := []string{"-avh", "--itemize-changes", "--stats"}
		if dryRun {
			args = append(args, "--dry-run")
		}
		args = append(args, src+"/", dst+"/")

		fmt.Printf("%s[run]%s rsync %s\n", yellow, reset, strings.Join(args, " "))
		cmd := exec.Command("rsync", args...)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		// Capture stdout while also displaying to terminal
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return SyncResult{Err: err}
		}
		if err := cmd.Start(); err != nil {
			return SyncResult{Err: err}
		}

		var output strings.Builder
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			output.WriteString(line + "\n")
		}

		cmdErr := cmd.Wait()
		captured := output.String()
		files := parseRsyncFiles(captured)
		bytes := parseRsyncBytes(captured)

		info("Done: " + filepath.Base(src))

		if cmdErr != nil {
			return SyncResult{
				Files:            files,
				FilesTransferred: len(files),
				BytesTransferred: bytes,
				Err:              cmdErr,
			}
		}
		return SyncResult{
			Files:            files,
			FilesTransferred: len(files),
			BytesTransferred: bytes,
		}
	}

	// cp fallback
	if dryRun {
		warn("cp does not support dry-run — listing files that would be copied:")
		if _, err := os.Stat(dst); err != nil {
			info("Destination does not exist yet — all files would be copied:")
			run("find", src, "-type", "f")
		} else {
			run("find", src, "-newer", dst, "-type", "f")
		}
		return SyncResult{}
	}
	if err := run("cp", "-ruv", src+"/.", dst+"/"); err != nil {
		fail("cp failed: " + err.Error())
		return SyncResult{Err: err}
	}

	info("Done: " + filepath.Base(src))
	return SyncResult{}
}

// --- Commands ---

func cmdUp(cfg Config) {
	connectWifi(cfg)
	mountShares(cfg)
}

func cmdDown(cfg Config) {
	unmountShares(cfg)
}

func cmdSync(cfg Config, dirs []string, execute bool) {
	dryRun := !execute

	if len(dirs) == 0 {
		dirs = cfg.DefaultSyncDirs
	}
	for i, d := range dirs {
		dirs[i] = expandHome(d)
	}

	if dryRun {
		warn("DRY RUN — no files will be written. Use --execute to sync for real.")
		fmt.Println()
	}

	connectWifi(cfg)
	mountShares(cfg)

	for _, dir := range dirs {
		dir = strings.TrimRight(dir, "/")
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			warn("Skipping " + dir + " (not a directory)")
			continue
		}
		home, _ := os.UserHomeDir()
		relPath, err := filepath.Rel(home, dir)
		if err != nil {
			relPath = filepath.Base(dir)
		}
		dest := filepath.Join(cfg.MountDocs, cfg.DocsSubfolder, relPath)
		syncDirectory(dir, dest, dryRun)
	}

	fmt.Println()
	if dryRun {
		info("Dry run complete. Run with --execute to sync for real.")
	} else {
		info("All syncs complete")
	}
}

func cmdStatus(cfg Config) {
	fmt.Println("=== NAS Connection Status ===")
	fmt.Println()

	run("ping", "-c", "1", "-W", "2", cfg.NASIP)
	if isNASReachable(cfg) {
		info("NAS reachable at " + cfg.NASIP)
	} else {
		fail("NAS not reachable at " + cfg.NASIP)
	}

	fmt.Println()
	fmt.Println("Wifi:")
	run("nmcli", "dev", "show", "wlan1")

	fmt.Println()
	fmt.Println("Mounts:")
	if isMounted(cfg.MountHome) {
		info(cfg.MountHome + " mounted")
	} else {
		warn(cfg.MountHome + " not mounted")
	}
	if isMounted(cfg.MountDocs) {
		info(cfg.MountDocs + " mounted")
	} else {
		warn(cfg.MountDocs + " not mounted")
	}
	if isMounted(cfg.MountPics) {
		info(cfg.MountPics + " mounted")
	} else {
		warn(cfg.MountPics + " not mounted")
	}
}

func usage() {
	fmt.Print(`Usage: nas-sync [command]

Commands:
  up          Connect wifi to NAS subnet and mount shares
  down        Unmount shares
  sync        Dry-run: show what would be copied (local -> NAS)
  sync --execute
              Copy files from local machine to NAS
  status      Show connection and mount status
  help        Show this help

Sync always copies FROM local TO NAS. Directories are mapped by their
path relative to $HOME, under the docs_subfolder from config.toml.
  e.g. ~/repos/project -> <mount_docs>/<docs_subfolder>/repos/project

Examples:
  nas-sync sync                        Dry-run default dirs -> NAS
  nas-sync sync ~/Documents ~/repos    Dry-run specific dirs -> NAS
  nas-sync sync --execute              Copy default dirs -> NAS for real
`)
}

// --- Main ---

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "help", "-h", "--help":
		usage()
		return
	}

	cfg := loadConfig()

	switch cmd {
	case "up":
		cmdUp(cfg)
	case "down":
		cmdDown(cfg)
	case "sync":
		args := os.Args[2:]
		execute := false
		var filtered []string
		for _, a := range args {
			if a == "--execute" {
				execute = true
			} else {
				filtered = append(filtered, a)
			}
		}
		cmdSync(cfg, filtered, execute)
	case "status":
		cmdStatus(cfg)
	default:
		fail("Unknown command: " + cmd)
		usage()
		os.Exit(1)
	}
}
