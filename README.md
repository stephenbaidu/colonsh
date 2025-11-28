## :: `colonsh`
> **A simple, customizable command-line utility for managing projects, executing repo-specific actions, and creating shell aliases.**

[![Go Build Status](https://github.com/colonsh/colonsh/actions/workflows/release.yml/badge.svg)](https://github.com/colonsh/colonsh/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Homebrew Version](https://img.shields.io/badge/Homebrew-v0.0.1-brightgreen)](https://github.com/colonsh/homebrew-tap)

***

## What is `colonsh`?

`colonsh` is a modern CLI tool written in **Go** that acts as a customizable command dispatch system for your shell.

It solves the problem of having scattered scripts and long `cd` commands by allowing you to define project root directories, custom aliases (`:pd`, `:po`), and specific build/deploy actions (`:pa`) that are accessible from anywhere in your terminal.

## Key Features

* **Project Navigation (`:pd`)**: Quickly jump to any defined project directory using an interactive fuzzy finder.
* **Project Actions (`:pa`)**: Run customizable scripts (e.g., `make deploy`, `docker build`) specific to the repository you are currently in.
* **Repo Opener (`:po`)**: Open the current repository in your preferred IDE (e.g., VS Code) or browser with a single command.
* **Git Helpers (`:gb`, `:gnb`)**: Built-in commands for common Git actions like branch switching (`:gb`) and new branch creation (`:gnb`).
* **Custom Aliases**: Define your own simple shell aliases right inside your `colonsh.json` config file.

***

## Installation

### A. Homebrew (OSX and Linux)

Since you have set up a Homebrew Tap, this is the easiest way to install `colonsh` on macOS or Linux.

```bash
# 1. Tap the repository
brew tap colonsh/tap

# 2. Install the application
brew install colonsh
```

### B. Chocolatey (Windows)

Chocolatey is the recommended package manager for Windows users.

```bash
# 1. Install the application
choco install colonsh
```

### C. Download Binary (All Platforms)

You can download the pre-compiled binary directly from the GitHub Releases page and manually add it to your PATH.

1. **Download:** Go to the [GitHub Releases page](https://github.com/colonsh/colonsh/releases) and download the appropriate file for your operating system and architecture (e.g., `colonsh_v0.0.1_macos_arm64.zip`).
2. **Extract:** Unzip the file and find the `colonsh` binary.
3. **Install:** Move the binary to a directory included in your system's PATH (e.g., `/usr/local/bin` on macOS/Linux, or a custom bin folder on Windows).

## Setup and Usage (The Crucial Step!)

To activate the powerful `:` aliases, you must integrate `colonsh` with your shell profile (`.zshrc`, `.bashrc`, etc.).

### 1. Run the Setup Command

This command automatically detects your shell and adds the necessary initialization block to your profile file.

```bash
colonsh setup
```

### 2. Reload Your Shell

You must `source` your profile file for the aliases to take effect in your current session:

```bash
source ~/.zshrc  # Use your specific profile file
```

### 3. Start Using Aliases
| Alias | Command | Description |
| :--- | :--- | :--- |
| `:pd` | `colonsh pd` | Select and **P**roject **D**irectory to `cd` into. |
| `:po` | `colonsh po` | **P**roject **O**pen: Open the current directory in your IDE. |
| `:pa` | `colonsh pa` | **P**roject **A**ctions: Run a configured action for the current repo. |
| `:help` | `colonsh` | Show the help menu. |

***

## Configuration

`colonsh` is driven entirely by a single JSON configuration file: **`~/colonsh.json`**.

Run the following command to open the file for editing:

```bash
colonsh config
```

### ⚙️ Complete `colonsh.json` Structure

This is the comprehensive configuration file used by `colonsh` to define global aliases, project locations, and repository-specific actions.

```json
{
  "aliases": [
    { 
      "name": "config", 
      "cmd": "code ~/colonsh.json" 
    },
    { 
      "name": "source", 
      "cmd": "source ~/.zshrc" 
    }
  ],
  "project_dirs": [
    { 
      "path": "~/Code/Work", 
      "exclude": ["archived"] 
    },
    { 
      "path": "~/Code/Personal" 
    }
  ],
  "git_repos": [
    {
      "slug": "colonsh/colonsh",
      "actions": [
        { 
          "name": "Deploy locally", 
          "cmd": "go build . && mv colonsh $HOME/bin/colonsh" 
        },
        { 
          "name": "Run tests", 
          "cmd": "go test ./..." 
        }
      ]
    }
  ]
}
```

### 📖 Configuration Sections Explained

### 1. `aliases`

The **`aliases`** array defines simple custom commands accessible from anywhere in your shell via the `:` prefix (e.g., `:config`, `:source`). These are simple command substitutions that run shell commands.

| Key | Description |
| :--- | :--- |
| **`name`** | The specific alias name to be used after the colon (e.g., `:config`). |
| **`cmd`** | The raw shell command that `colonsh` executes when the alias is called. |

### 2. `project_dirs`

The **`project_dirs`** array instructs `colonsh` where to scan for Git repositories on your system. This data is used by the `:pd` command to provide a searchable, quick-jump list of all your projects.

| Key | Description |
| :--- | :--- |
| **`path`** | The root directory path where `colonsh` should recursively look for Git repositories. Tilde (`~`) expansion is supported. |
| **`exclude`** | *(Optional)* A list of subdirectory names to ignore during the scan (e.g., excluding an `archived` folder within a large work directory). |

### 3. `git_repos`

The **`git_repos`** array defines specific actions and behaviors for individual Git repositories. This is the most powerful section, enabling context-aware actions via the `:pa` command.

| Key | Description |
| :--- | :--- |
| **`slug`** | The unique identifier for the repository, typically in the format `organization/repo-name` (e.g., `colonsh/colonsh`). |
| **`actions`** | A list of structured commands that only become available via `:pa` when your current working directory is inside this specific repository. |
| **`actions.name`** | The descriptive name displayed in the interactive list when running `:pa`. |
| **`actions.cmd`** | The shell command to be executed when this action is selected. |

***

## Development

### Building from Source

```bash
# Build the binary in the current directory
go build . 

# Run the compiled binary
./colonsh init zsh
```

### Testing

```bash
go test ./...
```

## License

`colonsh` is distributed under the **MIT License**. See the [LICENSE](LICENSE) file for details.