package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time" // NEW: Required for cmdSetup

	"github.com/charmbracelet/huh"
)

type BuiltinAlias struct {
	Name     string // without leading ':'
	Desc     string // for printHelp
	Template string // alias RHS; use {{BIN}} where COLONSH_BIN should go
}

// Order matters: simpler/common commands first usually looks better
var builtinAliases = []BuiltinAlias{
	// --- Core / Meta ---
	{Name: "help", Desc: "Show this help menu", Template: "{{BIN}}"}, // Handled specially as ::
	{Name: "init", Desc: "Emit shell integration code (stdout)", Template: ""},
	{Name: "setup", Desc: "Modify profile to auto-load colonsh", Template: ""}, // NEW: Added setup command
	{Name: "config", Desc: "Open colonsh config file", Template: "{{BIN}} config"},

	// --- Project Navigation ---
	{Name: "pd", Desc: "Select a project directory", Template: `cd "$({{BIN}} pd)"`},
	{Name: "cd", Desc: "Select subdirectory in CWD", Template: `cd "$({{BIN}} cd)"`},
	{Name: "po", Desc: "Open project in IDE", Template: "{{BIN}} po"},
	{Name: "pa", Desc: "Run actions for project", Template: "{{BIN}} pa"},

	// --- Git Helpers (Subcommands) ---
	{Name: "gb", Desc: "Select a git branch", Template: "{{BIN}} gb"},
	{Name: "gnb", Desc: "Create a new branch", Template: "{{BIN}} gnb"},
	{Name: "gdb", Desc: "Delete a branch", Template: "{{BIN}} gdb"},
	{Name: "gc", Desc: "git commit -m <msg>", Template: "{{BIN}} gc"},
	{Name: "gca", Desc: "git commit --amend", Template: "{{BIN}} gca"},
	{Name: "gcam", Desc: "git commit --amend -m <msg>", Template: "{{BIN}} gcam"},
	{Name: "prs", Desc: "Open Pull Requests URL", Template: "{{BIN}} prs"},

	// --- Pure Shell Aliases (No colonsh subcommand counterpart) ---
	{Name: "main", Desc: "Switch to main branch", Template: "git checkout main"},
	{Name: "master", Desc: "Switch to master branch", Template: "git checkout master"},
	{Name: "gs", Desc: "git status", Template: "git status"},
	{Name: "ll", Desc: "git pull", Template: "git pull"},
	{Name: "gaa", Desc: "git add .", Template: "git add ."},
	{Name: "gp", Desc: "git push", Template: "git push"},
	{Name: "gpf", Desc: "git push --force", Template: "git push --force"},
	{Name: "gl", Desc: "git log --oneline --graph", Template: "git log --oneline --graph --decorate"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "colonsh:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath, err := colonConfigPath()
	if err != nil {
		return err
	}

	cfg, err := loadOrInitConfig(cfgPath)
	if err != nil {
		return err
	}

	args := os.Args[1:]
	if len(args) == 0 {
		printHelp(cfg)
		return nil
	}

	switch args[0] {
	case "init":
		shellArg := "zsh"
		if len(args) > 1 {
			shellArg = args[1]
		}
		return cmdInit(shellArg, cfg)
	case "setup":
		return cmdSetup(cfg)
	case "config":
		return cmdConfig(cfgPath)
	case "pd":
		return cmdPD(cfg)
	case "po":
		return cmdPO(cfg)
	case "pa":
		return cmdPA(cfg)
	case "gb":
		return cmdGB()
	case "gnb":
		return cmdGNB(args[1:])
	case "gdb":
		return cmdGDB()
	case "gc":
		return cmdGC(args[1:])
	case "gca":
		return cmdGCA()
	case "gcam":
		return cmdGCAM(args[1:])
	case "prs":
		return cmdPRS()
	case "cd":
		return cmdCD()
	default:
		printHelp(cfg)
		return nil
	}
}

func printHelp(cfg *Config) {
	fmt.Printf("Welcome to colonsh! Your config file is at ~/%s\n", configFileName)

	// 1. Calculate padding width
	maxNameLen := 0
	for _, ba := range builtinAliases {
		if len(ba.Name) > maxNameLen {
			maxNameLen = len(ba.Name)
		}
	}
	// Check custom aliases for length too
	if cfg != nil {
		for _, a := range cfg.Aliases {
			// +1 for the leading colon in visual representation logic below
			if len(a.Name)+1 > maxNameLen {
				maxNameLen = len(a.Name) + 1
			}
		}
	}

	// 2. Print Built-in Aliases (Everything except special init cases)
	fmt.Println("\nBuilt-in :aliases:")
	// Add the root alias manually for clarity
	fmt.Printf("  %-*s  %s\n", maxNameLen, "::", "Show this help menu")

	for _, ba := range builtinAliases {
		// Skip commands without a template (init, setup)
		if ba.Template == "" || ba.Name == "help" {
			continue
		}
		name := ":" + ba.Name
		fmt.Printf("  %-*s  %s\n", maxNameLen, name, ba.Desc)
	}

	// 3. Custom aliases from config
	if cfg != nil && len(cfg.Aliases) > 0 {
		fmt.Println("\nCustom aliases (from config):")
		for _, a := range cfg.Aliases {
			if a.Name == "" || a.Cmd == "" {
				continue
			}
			name := ":" + a.Name
			fmt.Printf("  %-*s  %s\n", maxNameLen, name, a.Cmd)
		}
	}
	fmt.Println()
}

func shellQuoteSingle(s string) string {
	// Escapes single quotes by closing the string, adding an escaped quote, and reopening.
	// ' -> '\''
	return strings.ReplaceAll(s, `'`, `'\''`)
}

// -----------------------------------------------------------------------------
// init – emit shell integration code (to stdout)
// -----------------------------------------------------------------------------
func cmdInit(shellArg string, cfg *Config) error {
	// If the user didn't specify the shell, use detection logic
	if shellArg != "zsh" && shellArg != "bash" && shellArg != "powershell" {
		shellArg = detectShell()
	}

	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "colonsh"
	}

	var buf bytes.Buffer

	// --- PowerShell Output ---
	if shellArg == "powershell" {
		fmt.Fprintf(&buf, `# colonsh PowerShell Integration
# Paste the output of 'colonsh init' into your $PROFILE file.

# Binary path (using full path for reliability)
$COLONSH_BIN='%s'

# Root alias (::)
Function Global:colonsh { & $COLONSH_BIN @args }
Set-Alias -Name '::' -Value colonsh

# --- Built-in Aliases (PowerShell) ---
`, exe)
		// NOTE: Complex aliases like :pd='cd "$(colonsh pd)"' require PowerShell functions
		// instead of simple Set-Alias, which is too complex for this init output.
		// Sticking to simple aliases for now, warning user about limitations.
		for _, ba := range builtinAliases {
			if ba.Template == "" || ba.Name == "help" || ba.Name == "pd" || ba.Name == "cd" {
				continue
			}
			cmd := strings.ReplaceAll(ba.Template, "{{BIN}}", "$COLONSH_BIN")
			// Simple replacement, might fail for complex aliases involving sub-shells/eval
			buf.WriteString(fmt.Sprintf("Set-Alias -Name ':%s' -Value '%s'\n", ba.Name, strings.ReplaceAll(cmd, "$COLONSH_BIN", exe)))
		}

	} else {
		// --- UNIX Shell Output (bash/zsh) ---
		fmt.Fprintf(&buf, `# colonsh shell integration
# Generated by: %s init %s

export COLONSH_BIN=%q

# Root help / entrypoint
alias ::='$COLONSH_BIN'

# --- Built-in Aliases (UNIX) ---
`, filepath.Base(exe), shellArg, exe)

		// Dynamically generate aliases from builtinAliases
		for _, ba := range builtinAliases {
			// Skip commands that don't have a template (like 'init', 'setup')
			if ba.Template == "" || ba.Name == "help" {
				continue
			}

			// Replace placeholder with env var
			cmd := strings.ReplaceAll(ba.Template, "{{BIN}}", "$COLONSH_BIN")

			// Use shellQuoteSingle to ensure the command inside the alias is safe
			buf.WriteString(fmt.Sprintf("alias :%s='%s'\n", ba.Name, shellQuoteSingle(cmd)))
		}
	}

	// --- Custom Aliases from Config (Appended to both) ---
	if cfg != nil && len(cfg.Aliases) > 0 {
		buf.WriteString("\n# --- Custom aliases from colonsh.json ---\n")
		for _, a := range cfg.Aliases {
			if a.Name == "" || a.Cmd == "" {
				continue
			}
			if shellArg == "powershell" {
				buf.WriteString(fmt.Sprintf("Set-Alias -Name ':%s' -Value '%s'\n", a.Name, a.Cmd))
			} else {
				buf.WriteString(fmt.Sprintf("alias :%s='%s'\n", a.Name, shellQuoteSingle(a.Cmd)))
			}
		}
	}

	fmt.Print(buf.String())
	return nil
}

// -----------------------------------------------------------------------------
// setup – automatically modify the user's shell profile file
// -----------------------------------------------------------------------------

func cmdSetup(cfg *Config) error {
	targetShell := detectShell()

	// 1. Determine the path to the user's profile file
	var profilePath string
	switch targetShell {
	case "bash":
		profilePath = "~/.bashrc"
		if runtime.GOOS == "darwin" {
			// macOS often uses .bash_profile
			if _, err := os.Stat(profilePath); os.IsNotExist(err) {
				profilePath = "~/.bash_profile"
			}
		}
	case "zsh":
		profilePath = "~/.zshrc"
	case "fish":
		if home, err := os.UserHomeDir(); err == nil {
			profilePath = filepath.Join(home, ".config/fish/config.fish")
		} else {
			return fmt.Errorf("could not determine home directory for fish profile")
		}
	case "powershell":
		fmt.Println("PowerShell requires manual setup due to dynamic profile paths and security policies.")
		fmt.Printf("1. Run: colonsh init powershell\n2. Copy the output into your $PROFILE file (e.g., C:\\Users\\...\\profile.ps1).\n")
		return nil // Success, but no automated change made
	default:
		return fmt.Errorf("unsupported shell %q for automatic setup. Please use 'colonsh init' and follow manual instructions", targetShell)
	}

	// Expand the tilde (~) path
	expandedPath, err := expandTilde(profilePath)
	if err != nil {
		return err
	}

	// 2. Check if the setup block already exists
	content, err := os.ReadFile(expandedPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read profile file %s: %w", expandedPath, err)
	}

	if bytes.Contains(content, []byte("# --- colonsh Integration ---")) {
		fmt.Printf("✅ colonsh setup block already found in %s. Nothing changed.\n", expandedPath)
		return nil
	}

	// 3. Generate the conditional loading block (UNIX style only)
	setupBlock := fmt.Sprintf(`
# --- colonsh Integration ---
# Added by 'colonsh setup' on %s
if command -v colonsh >/dev/null 2>&1; then
  # Load aliases generated by 'colonsh init'
  eval "$(colonsh init %s)"
  echo "colonsh loaded"
fi
# --- End colonsh Integration ---
`, time.Now().Format("2006-01-02"), targetShell)

	// 4. Append the block to the profile file
	f, err := os.OpenFile(expandedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", expandedPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(setupBlock); err != nil {
		return fmt.Errorf("failed to write to %s: %w", expandedPath, err)
	}

	fmt.Printf("🎉 Successfully appended colonsh setup block to %s.\n", expandedPath)
	fmt.Printf("Please run 'source %s' or restart your terminal for changes to take effect.\n", profilePath)
	return nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func detectShell() string {
	shellPath := os.Getenv("SHELL")
	if shellPath != "" {
		base := filepath.Base(shellPath)
		if strings.Contains(base, "zsh") {
			return "zsh"
		}
		if strings.Contains(base, "bash") {
			return "bash"
		}
		if strings.Contains(base, "fish") {
			return "fish"
		}
	}

	if runtime.GOOS == "windows" {
		return "powershell"
	}

	return "zsh"
}

func expandTilde(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func runShellCommand(cmdStr string, dir string) error {
	if cmdStr == "" {
		return errors.New("empty command")
	}

	// Check the user's SHELL environment variable
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Fallback if SHELL is not set, or on Windows
		if runtime.GOOS == "windows" {
			shell = "powershell"
		} else {
			shell = "bash"
		}
	}

	// The flag to execute the command in the shell might need adjustment:
	// bash/zsh often use -c, but -lc is safer for initialization files.
	cmd := exec.Command(shell, "-lc", cmdStr)

	if dir != "" {
		cmd.Dir = dir
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func inGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func gitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// getRawGitRemoteURL executes the git command to retrieve the remote.origin.url.
func getRawGitRemoteURL() (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// gitRepoSlug executes the Git command via a helper and returns the canonical repository slug
// in the format "user/repo" (e.g., "stephenbaidu/colonsh").
func gitRepoSlug() (string, error) {
	// 1. Get the raw remote URL using the helper (no exec.Command duplication)
	rawURL, err := getRawGitRemoteURL()
	if err != nil {
		return "", err
	}

	// 2. Normalize the URL (Replaces normalizeGitURL)
	s := strings.TrimSpace(rawURL)

	// a. Remove .git suffix
	s = strings.TrimSuffix(s, ".git")

	// b. Handle SSH format: git@github.com:user/repo
	if strings.HasPrefix(s, "git@") {
		s = strings.TrimPrefix(s, "git@")
		s = strings.Replace(s, ":", "/", 1)
	}

	// c. Handle HTTP/HTTPS
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	// Result: host/user/repo (e.g., github.com/user/repo)

	// 3. Extract the "user/repo" part
	parts := strings.SplitN(s, "/", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("could not extract slug from normalized URL: %s", s)
	}

	// The slug is the second part: user/repo
	return parts[1], nil
}

// findCurrentRepo executes the Git command to find the current repository slug
// and then performs the lookup against the provided config.
func findCurrentRepo(cfg *Config) *GitRepo {
	// Note: This function now executes external commands (Git) and should handle errors internally

	repoSlug, err := gitRepoSlug() // This function contains the Git execution logic
	if err != nil {
		// If we can't get the slug (e.g., no remote), return nil
		return nil
	}

	// Perform the lookup against the config list
	for i := range cfg.GitRepos {
		if cfg.GitRepos[i].Slug == repoSlug {
			return &cfg.GitRepos[i]
		}
	}
	return nil
}

// NOTE: You must update all callers (cmdPA, cmdPO) to use this new function and handle the case
// where it returns nil, instead of handling errors from gitRepoSlug().

// -----------------------------------------------------------------------------
// config – open config file in default editor
// -----------------------------------------------------------------------------
func cmdConfig(configPath string) error {
	// Ensure file exists
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not found at %s: %w", configPath, err)
	}

	fmt.Println("Opening config:", configPath)

	// Use the new generic function to open the local file path
	return openPath(configPath)
}

// -----------------------------------------------------------------------------
// pd – project directory selection
// -----------------------------------------------------------------------------

func cmdPD(cfg *Config) error {
	var projects []string

	for _, pd := range cfg.ProjectDirs {
		root, err := expandTilde(pd.Path)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}

		exclude := make(map[string]struct{}, len(pd.Exclude))
		for _, ex := range pd.Exclude {
			exclude[ex] = struct{}{}
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if _, skip := exclude[name]; skip {
				continue
			}
			projects = append(projects, filepath.Join(root, name))
		}
	}

	if len(projects) == 0 {
		return errors.New("no projects found from project_dirs")
	}

	var selected string
	opts := []huh.Option[string]{}
	for _, p := range projects {
		opts = append(opts, huh.NewOption(p, p))
	}

	if err := huh.NewSelect[string]().
		Title("Select a project directory").
		Options(opts...).
		Value(&selected).
		Run(); err != nil {
		return err
	}

	if selected == "" {
		return errors.New("no project selected")
	}

	// print for alias: alias :pd='cd "$(colonsh pd)"'
	fmt.Println(selected)
	return nil
}

// -----------------------------------------------------------------------------
// po – open project (IDE / open_cmd)
// -----------------------------------------------------------------------------

func cmdPO(cfg *Config) error {
	var baseDir string

	// 1. Enforce requirement: Must be inside a Git repository.
	if !inGitRepo() {
		return errors.New("command 'po' currently expects to be run inside a git repository")
	}

	// 2. Get the root directory of the repository.
	var err error
	baseDir, err = gitRoot()
	if err != nil {
		return fmt.Errorf("failed to get git root: %w", err)
	}

	// 3. Find the specific repository config using the new lookup function.
	// This function handles getting the slug and finding the matching config entry.
	repo := findCurrentRepo(cfg)

	// 4. Determine the open command, prioritizing repo-specific setting.

	// Start with global default (from config or hardcoded)
	openCmd := cfg.OpenCmd
	if openCmd == "" {
		openCmd = "code ."
	}

	// Override with repo-specific setting if a matching GitRepo was found
	if repo != nil && repo.OpenCmd != "" {
		openCmd = repo.OpenCmd
	}

	// 5. Execute the command in the root directory.
	fmt.Printf("Opening project at %s with: %s\n", baseDir, openCmd)
	return runShellCommand(openCmd, baseDir)
}

// -----------------------------------------------------------------------------
// pa – run repo actions
// -----------------------------------------------------------------------------

func cmdPA(cfg *Config) error {
	if !inGitRepo() {
		return errors.New("not inside a git repository")
	}

	root, err := gitRoot()
	if err != nil {
		return err
	}

	repo := findCurrentRepo(cfg)
	if repo == nil || len(repo.Actions) == 0 {
		return errors.New("no actions found for this repository in colonsh.json")
	}

	opts := []huh.Option[string]{}
	for _, a := range repo.Actions {
		opts = append(opts, huh.NewOption(a.Name, a.Name))
	}

	var selectedName string
	if err := huh.NewSelect[string]().
		Title("Select an action").
		Options(opts...).
		Value(&selectedName).
		Run(); err != nil {
		return err
	}
	if selectedName == "" {
		fmt.Println("No action selected.")
		return nil
	}

	var action *RepoAction
	for i := range repo.Actions {
		if repo.Actions[i].Name == selectedName {
			action = &repo.Actions[i]
			break
		}
	}
	if action == nil {
		return errors.New("selected action not found")
	}

	runDir := root
	if action.Dir != "" && action.Dir != "." {
		runDir = filepath.Join(root, action.Dir)
	}

	fmt.Printf("Executing action %q in %s: %s\n", action.Name, runDir, action.Cmd)
	return runShellCommand(action.Cmd, runDir)
}

// -----------------------------------------------------------------------------
// gb – select branch & checkout
// -----------------------------------------------------------------------------

func cmdGB() error {
	branches, err := gitBranchesRaw()
	if err != nil {
		return err
	}
	if len(branches) == 0 {
		return errors.New("no branches found")
	}

	opts := []huh.Option[string]{}
	for _, b := range branches {
		opts = append(opts, huh.NewOption(b, b))
	}

	var selected string
	if err := huh.NewSelect[string]().
		Title("Select a branch").
		Options(opts...).
		Value(&selected).
		Run(); err != nil {
		return err
	}
	if selected == "" {
		fmt.Println("No branch selected.")
		return nil
	}

	fmt.Println("Switching to branch:", selected)
	cmd := exec.Command("git", "checkout", selected)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// -----------------------------------------------------------------------------
// gnb – create new branch
// -----------------------------------------------------------------------------

func cmdGNB(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: colonsh gnb <branch-name>")
	}
	branchName := strings.Join(args, "-")

	u, err := user.Current()
	username := "user"
	if err == nil && u.Username != "" {
		username = u.Username
	}
	full := fmt.Sprintf("%s/%s", username, branchName)

	fmt.Println("Creating and switching to branch:", full)
	cmd := exec.Command("git", "checkout", "-b", full)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// -----------------------------------------------------------------------------
// gdb – delete multiple branches (exclude main/master)
// -----------------------------------------------------------------------------

func cmdGDB() error {
	all, err := gitBranchesRaw()
	if err != nil {
		return err
	}
	var branches []string
	for _, b := range all {
		if b == "main" || b == "master" {
			continue
		}
		branches = append(branches, b)
	}

	if len(branches) == 0 {
		fmt.Println("No branches available for deletion (all filtered).")
		return nil
	}

	opts := []huh.Option[string]{}
	for _, b := range branches {
		opts = append(opts, huh.NewOption(b, b))
	}

	var selected []string
	if err := huh.NewMultiSelect[string]().
		Title("Select branch(es) to delete").
		Options(opts...).
		Value(&selected).
		Run(); err != nil {
		return err
	}

	if len(selected) == 0 {
		fmt.Println("No branches selected.")
		return nil
	}

	fmt.Println("Branches to delete:")
	for _, b := range selected {
		fmt.Println("  " + b)
	}

	var confirm bool
	if err := huh.NewConfirm().
		Title("Proceed with deletion?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run(); err != nil {
		return err
	}
	if !confirm {
		fmt.Println("Aborted.")
		return nil
	}

	for _, b := range selected {
		fmt.Println("Deleting branch:", b)
		cmd := exec.Command("git", "branch", "-d", b)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		_ = cmd.Run() // ignore individual failures, just print output
	}

	return nil
}

func gitBranchesRaw() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		branches = append(branches, line)
	}
	return branches, nil
}

// -----------------------------------------------------------------------------
// gc / gca / gcam – git commits
// -----------------------------------------------------------------------------

func cmdGC(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: colonsh gc <commit-message>")
	}
	msg := strings.Join(args, " ")
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func cmdGCA() error {
	cmd := exec.Command("git", "commit", "--amend")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func cmdGCAM(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: colonsh gcam <commit-message>")
	}
	msg := strings.Join(args, " ")
	cmd := exec.Command("git", "commit", "--amend", "-m", msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// -----------------------------------------------------------------------------
// prs – open PRs URL
// -----------------------------------------------------------------------------

func cmdPRS() error {
	if !inGitRepo() {
		return errors.New("this is not a git repository")
	}
	gurl, err := getRawGitRemoteURL()
	if err != nil {
		return err
	}

	// Convert SSH git@github.com:owner/repo.git → https://github.com/owner/repo/pulls
	// or just append /pulls if already https.
	var pullsURL string
	if strings.HasPrefix(gurl, "git@") {
		// git@github.com:owner/repo.git
		parts := strings.SplitN(strings.TrimPrefix(gurl, "git@"), ":", 2)
		if len(parts) == 2 {
			host := parts[0]
			path := strings.TrimSuffix(parts[1], ".git")
			pullsURL = fmt.Sprintf("https://%s/%s/pulls", host, path)
		}
	} else if strings.HasPrefix(gurl, "https://") || strings.HasPrefix(gurl, "http://") {
		pullsURL = strings.TrimSuffix(gurl, ".git") + "/pulls"
	}

	if pullsURL == "" {
		return fmt.Errorf("could not construct pulls URL from remote %q", gurl)
	}

	fmt.Println("Opening:", pullsURL)
	return openPath(pullsURL)
}

// openPath opens the given path (file or URL) using the system's default handler.
func openPath(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS: uses 'open'
		cmd = exec.Command("open", path)
	case "windows":
		// Windows: uses 'cmd /c start'
		cmd = exec.Command("cmd", "/c", "start", path)
	case "linux":
		// Linux: Try common desktop environment commands
		return runLinuxBrowserCommand(path) // Re-use the multi-command logic for Linux
	default:
		// Fallback for other POSIX-like systems
		cmd = exec.Command("xdg-open", path)
	}

	if cmd != nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return fmt.Errorf("unsupported operating system or failed to open path: %s", path)
}

// runLinuxBrowserCommand tries common commands for opening a URL on Linux.
func runLinuxBrowserCommand(url string) error {
	// Ordered list of common Linux commands for opening a URL/file
	browsers := []string{"xdg-open", "gnome-open", "kde-open", "x-www-browser", "firefox", "chromium"}

	for _, browser := range browsers {
		cmd := exec.Command(browser, url)
		// We only care if the command *can be executed*.
		// We capture output to avoid cluttering stdout/stderr if a tool fails.
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		// Use LookPath to check if the command exists before running it
		_, lookPathErr := exec.LookPath(browser)
		if lookPathErr != nil {
			continue // Command not found, try the next one
		}

		// Run the command
		if err := cmd.Run(); err == nil {
			return nil // Success!
		}
	}
	return fmt.Errorf("failed to open browser/URL using all known commands: %s", url)
}

// -----------------------------------------------------------------------------
// cd – select subdirectory in CWD (prints path)
// -----------------------------------------------------------------------------

func cmdCD() error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return err
	}

	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, name)
	}

	if len(dirs) == 0 {
		return errors.New("no subdirectories found")
	}

	opts := []huh.Option[string]{}
	for _, d := range dirs {
		opts = append(opts, huh.NewOption(d, d))
	}

	var selected string
	if err := huh.NewSelect[string]().
		Title("Select a directory").
		Options(opts...).
		Value(&selected).
		Run(); err != nil {
		return err
	}

	if selected == "" {
		fmt.Println("No directory selected.")
		return nil
	}

	// Print for alias: alias :cd='cd "$(colonsh cd)"'
	fmt.Println(selected)
	return nil
}
