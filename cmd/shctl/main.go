package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	// Environment overrides
	envRCFile     = getenvDefault("BASM_RC_FILE", "")
	envSudoers    = getenvDefault("BASM_SUDOERS_PATH", "")
	envBackupDir  = getenvDefault("BASM_BACKUP_DIR", "/tmp")
	shellPath     = getenvDefault("SHELL", "/bin/bash")
	defaultIsZsh  = strings.HasSuffix(shellPath, "zsh")
	defaultRCName = ".bashrc"
)

func init() {
	if defaultIsZsh {
		defaultRCName = ".zshrc"
	}
}

func main() {
	if len(os.Args) < 2 {
		usageAndExit()
	}

	cmd := os.Args[1]
	switch cmd {
	case "alias":
		handleAlias(os.Args[2:])
	case "export":
		handleExport(os.Args[2:])
	case "sudoers":
		handleSudoers(os.Args[2:])
	case "backup":
		handleBackup(os.Args[2:])
	case "restore":
		handleRestore(os.Args[2:])
	case "apply":
		handleApply()
	case "help", "--help", "-h":
		usageAndExit()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usageAndExit()
	}
}

// ----------------- Helpers: env, paths -----------------

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func rcFilePath() string {
	if envRCFile != "" {
		return envRCFile
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, defaultRCName)
}

func sudoersPath() string {
	if envSudoers != "" {
		return envSudoers
	}
	return "/etc/sudoers"
}

func backupDir() string {
	return envBackupDir
}

// ----------------- Usage -----------------

func usageAndExit() {
	fmt.Print(`cli-tool (Go)

Usage:
  cli-tool <command> [subcommand] [args...]

Commands:
  alias    add <name> <command>   : add alias
           list                    : list aliases
           remove <name>           : remove alias

  export   add <VAR> <value>      : add export
           list                    : list exports
           remove <VAR>            : remove export

  sudoers  add <entry>            : add sudoers entry (uses visudo validation)
           list                    : list non-comment sudoers lines
           remove <pattern>        : remove lines containing pattern (validates)

  backup   [--no-rc] [--no-sudoers] : backup files to backup dir
  restore  [--no-rc] [--no-sudoers] : restore from backups (sudo may be required)

  apply    : source the RC file in a shell (spawns shell - won't affect current process)

Environment overrides:
  BASM_RC_FILE        - path to rc file (default: ~/.bashrc or ~/.zshrc)
  BASM_SUDOERS_PATH   - path to sudoers (default: /etc/sudoers)
  BASM_BACKUP_DIR     - backup directory (default: /tmp)

Examples:
  cli-tool alias add ll "ls -la"
  cli-tool alias list
  cli-tool sudoers add "myuser ALL=(ALL) NOPASSWD: /usr/bin/somebinary"
`)
	os.Exit(1)
}

// ----------------- Alias commands -----------------

func handleAlias(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "alias: requires subcommand")
		usageAndExit()
	}
	action := args[0]
	switch action {
	case "add":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "alias add requires name and command")
			os.Exit(2)
		}
		name, cmd := args[1], args[2]
		if err := addAlias(name, cmd); err != nil {
			dieErr(err)
		}
		fmt.Printf("Alias '%s' added to %s\n", name, rcFilePath())
	case "list":
		if err := listAliases(); err != nil {
			dieErr(err)
		}
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "alias remove requires name")
			os.Exit(2)
		}
		if err := removeAlias(args[1]); err != nil {
			dieErr(err)
		}
		fmt.Printf("Alias '%s' removed (if present) from %s\n", args[1], rcFilePath())
	default:
		fmt.Fprintf(os.Stderr, "alias: unknown action %s\n", action)
		usageAndExit()
	}
}

func addAlias(name, command string) error {
	path := rcFilePath()
	if err := ensureFile(path); err != nil {
		return err
	}
	line := fmt.Sprintf("alias %s='%s'\n", name, command)
	return appendAtomic(path, []byte(line))
}

func listAliases() error {
	path := rcFilePath()
	if err := ensureFile(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return scanAndPrintPrefix(f, "alias ")
}

func removeAlias(name string) error {
	path := rcFilePath()
	if err := ensureFile(path); err != nil {
		return err
	}
	return removeLinesContainingPrefix(path, fmt.Sprintf("alias %s=", name))
}

// ----------------- Export commands -----------------

func handleExport(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "export: requires subcommand")
		usageAndExit()
	}
	action := args[0]
	switch action {
	case "add":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "export add requires var and value")
			os.Exit(2)
		}
		if err := addExport(args[1], args[2]); err != nil {
			dieErr(err)
		}
		fmt.Printf("Export '%s' added to %s\n", args[1], rcFilePath())
	case "list":
		if err := listExports(); err != nil {
			dieErr(err)
		}
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "export remove requires var")
			os.Exit(2)
		}
		if err := removeExport(args[1]); err != nil {
			dieErr(err)
		}
		fmt.Printf("Export '%s' removed (if present) from %s\n", args[1], rcFilePath())
	default:
		fmt.Fprintf(os.Stderr, "export: unknown action %s\n", action)
		usageAndExit()
	}
}

func addExport(varName, value string) error {
	path := rcFilePath()
	if err := ensureFile(path); err != nil {
		return err
	}
	if strings.ContainsAny(value, " ") {
		value = fmt.Sprintf("\"%s\"", value)
	}
	line := fmt.Sprintf("export %s=%s\n", varName, value)
	return appendAtomic(path, []byte(line))
}

func listExports() error {
	path := rcFilePath()
	if err := ensureFile(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return scanAndPrintPrefix(f, "export ")
}

func removeExport(varName string) error {
	path := rcFilePath()
	if err := ensureFile(path); err != nil {
		return err
	}
	return removeLinesContainingPrefix(path, fmt.Sprintf("export %s=", varName))
}

// ----------------- Sudoers commands -----------------

func handleSudoers(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "sudoers: requires subcommand")
		usageAndExit()
	}
	action := args[0]
	switch action {
	case "add":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "sudoers add requires entry string (wrap it in quotes)")
			os.Exit(2)
		}
		if err := sudoersAdd(args[1]); err != nil {
			dieErr(err)
		}
	case "list":
		if err := sudoersList(); err != nil {
			dieErr(err)
		}
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "sudoers remove requires pattern")
			os.Exit(2)
		}
		if err := sudoersRemove(args[1]); err != nil {
			dieErr(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "sudoers: unknown action %s\n", action)
		usageAndExit()
	}
}

func sudoersList() error {
	path := sudoersPath()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return scanAndPrintNonComment(f)
}

// copy to temp, append entry, validate with visudo -c -f <tmp>, then apply
func sudoersAdd(entry string) error {
	orig := sudoersPath()
	tmp, err := copyToTemp(orig)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	// Append entry
	if err := appendFile(tmp, []byte("\n"+entry+"\n")); err != nil {
		return err
	}

	// Validate
	if err := visudoValidate(tmp); err != nil {
		return fmt.Errorf("visudo validation failed: %w", err)
	}

	// Apply (may need sudo if writing to /etc/sudoers)
	if err := copyBack(tmp, orig); err != nil {
		return err
	}

	fmt.Println("Sudoers entry added and applied.")
	return nil
}

func sudoersRemove(pattern string) error {
	orig := sudoersPath()
	tmp, err := copyToTemp(orig)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	// Remove lines containing pattern
	if err := removeLinesContaining(tmp, pattern); err != nil {
		return err
	}

	// Validate
	if err := visudoValidate(tmp); err != nil {
		return fmt.Errorf("visudo validation failed after removal: %w", err)
	}

	// Apply
	if err := copyBack(tmp, orig); err != nil {
		return err
	}

	fmt.Printf("Removed lines containing pattern: %s\n", pattern)
	return nil
}

// ----------------- Backup & Restore -----------------

func handleBackup(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	noRc := fs.Bool("no-rc", false, "Don't backup RC file")
	noSudo := fs.Bool("no-sudoers", false, "Don't backup sudoers")
	fs.Parse(args)

	results, err := backup(!*noRc, !*noSudo)
	if err != nil {
		dieErr(err)
	}
	for k, v := range results {
		fmt.Printf("Backed up %s -> %s\n", k, v)
	}
}

func handleRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	noRc := fs.Bool("no-rc", false, "Don't restore RC file")
	noSudo := fs.Bool("no-sudoers", false, "Don't restore sudoers")
	fs.Parse(args)

	results, err := restore(!*noRc, !*noSudo)
	if err != nil {
		dieErr(err)
	}
	for k, v := range results {
		fmt.Printf("Restored %s -> %s\n", k, v)
	}
}

func backup(rc, sudoers bool) (map[string]string, error) {
	out := map[string]string{}
	dir := backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ts := time.Now().Format("20060102_150405")
	if rc {
		src := rcFilePath()
		dst := filepath.Join(dir, filepath.Base(src)+".bak."+ts)
		if err := copyFile(src, dst); err != nil {
			return nil, err
		}
		out["rc"] = dst
	}
	if sudoers {
		src := sudoersPath()
		dst := filepath.Join(dir, filepath.Base(src)+".bak."+ts)
		if err := copyFile(src, dst); err != nil {
			return nil, err
		}
		out["sudoers"] = dst
	}
	return out, nil
}

func restore(rc, sudoers bool) (map[string]string, error) {
	out := map[string]string{}
	dir := backupDir()
	if rc {
		srcPattern := filepath.Join(dir, filepath.Base(rcFilePath())+".bak.*")
		matches, _ := filepath.Glob(srcPattern)
		if len(matches) == 0 {
			fmt.Printf("No rc backup found in %s\n", dir)
		} else {
			latest := latestFile(matches)
			if err := copyFile(latest, rcFilePath()); err != nil {
				return nil, err
			}
			out["rc"] = rcFilePath()
		}
	}
	if sudoers {
		srcPattern := filepath.Join(dir, filepath.Base(sudoersPath())+".bak.*")
		matches, _ := filepath.Glob(srcPattern)
		if len(matches) == 0 {
			fmt.Printf("No sudoers backup found in %s\n", dir)
		} else {
			latest := latestFile(matches)
			// Validate before applying
			tmp, err := copyToTemp(latest)
			if err != nil {
				return nil, err
			}
			defer os.Remove(tmp)
			if err := visudoValidate(tmp); err != nil {
				return nil, fmt.Errorf("backup sudoers failed validation: %w", err)
			}
			if err := copyBack(tmp, sudoersPath()); err != nil {
				return nil, err
			}
			out["sudoers"] = sudoersPath()
		}
	}
	return out, nil
}

// ----------------- Apply -----------------

func handleApply() {
	// spawn a shell and source file. This won't affect the parent process.
	rc := rcFilePath()
	cmd := exec.Command(shellPath, "-c", fmt.Sprintf("source %s", rc))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	fmt.Println("Sourced rc in a subshell (this does not affect the current shell session).")
}

// ----------------- File utilities -----------------

func ensureFile(path string) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		f, err := os.OpenFile(path, os.O_CREATE, 0o644)
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func appendAtomic(path string, data []byte) error {
	// open file for append
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func scanAndPrintPrefix(r io.Reader, prefix string) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			fmt.Println(line)
		}
	}
	return sc.Err()
}

func scanAndPrintNonComment(r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		fmt.Println(line)
	}
	return sc.Err()
}

func removeLinesContainingPrefix(path, prefix string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	out := []string{}
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), prefix) {
			continue
		}
		out = append(out, ln)
	}
	return atomicWriteFile(path, strings.Join(out, "\n"))
}

func removeLinesContaining(path, pattern string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	out := []string{}
	for _, ln := range lines {
		if strings.Contains(ln, pattern) {
			continue
		}
		out = append(out, ln)
	}
	return atomicWriteFile(path, strings.Join(out, "\n"))
}

func atomicWriteFile(path, content string) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, ".tmp_"+filepath.Base(path))
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ----------------- File copy / temp / validation -----------------

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func copyToTemp(src string) (string, error) {
	content, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "sudoers_*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return "", err
	}
	_ = tmp.Close()
	// preserve original permissions if possible
	fi, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(tmp.Name(), fi.Mode())
	}
	return tmp.Name(), nil
}

func copyBack(tmp, dest string) error {
	if dest == "/etc/sudoers" {
		// require sudo cp
		cmd := exec.Command("sudo", "cp", tmp, dest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// normal file copy
	return copyFile(tmp, dest)
}

func visudoValidate(path string) error {
	cmd := exec.Command("visudo", "-c", "-f", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("visudo error: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ----------------- Misc helpers -----------------

func dieErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}

func appendFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func latestFile(files []string) string {
	latest := files[0]
	var latestTime time.Time
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		t := fi.ModTime()
		if t.After(latestTime) {
			latest = f
			latestTime = t
		}
	}
	return latest
}
