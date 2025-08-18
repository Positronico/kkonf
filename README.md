# kkonf - kubectl Config Manager

[![Latest Release](https://img.shields.io/github/release/positronico/kkonf.svg)](https://github.com/positronico/kkonf/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/positronico/kkonf/total.svg)](https://github.com/positronico/kkonf/releases)

`kkonf` is an interactive CLI tool for managing kubectl configuration files with a focus on user consolidation and simplified management of clusters, users, and contexts.

## 📥 Quick Download

**→ [Download pre-built binaries from GitHub Releases](https://github.com/positronico/kkonf/releases/latest) ←**

Choose the appropriate binary for your platform (Linux, macOS, Windows) from the latest release page.

## Features

- **Interactive Menu System**: Navigate through options with arrow keys and an intuitive interface
- **CRUD Operations**: Full Create, Read, Update, Delete support for:
  - Clusters
  - Users  
  - Contexts
- **User Consolidation**: Automatically detect and merge duplicate users with identical properties
- **Config Validation**: Validate configuration integrity and check for broken references
- **Import/Export**: Import from other config files and export selected items
- **Backup Management**: Automatic backups before modifications
- **Quick Context Switching**: Fast context switching with namespace management
- **Beautiful Display**: Color-coded output with emoji indicators

## Installation

### Option 1: Download Pre-built Binaries (Recommended)

**→ [Go to Releases Page](https://github.com/positronico/kkonf/releases/latest) ←**

1. Download the appropriate binary for your platform:
   - **Linux**: `kkonf-v1.1.1-linux-amd64.tar.gz` or `kkonf-v1.1.1-linux-arm64.tar.gz`
   - **macOS**: `kkonf-v1.1.1-darwin-amd64.tar.gz` or `kkonf-v1.1.1-darwin-arm64.tar.gz`  
   - **Windows**: `kkonf-v1.1.1-windows-amd64.zip`

2. Extract and run:
   ```bash
   # Linux/macOS
   tar -xzf kkonf-v*.tar.gz
   ./kkonf
   
   # Windows
   # Extract the .zip file and run kkonf.exe
   ```

### Option 2: Install with Go
```bash
go install github.com/positronico/kkonf@latest
```

### Option 3: Build from Source
**Prerequisites:** Go 1.21 or higher

```bash
git clone https://github.com/positronico/kkonf.git
cd kkonf
make build  # or: go build -o kkonf
```

## Usage

### Basic Usage
```bash
# Use default kubeconfig (~/.kube/config)
kkonf

# Specify a different config file
kkonf -f /path/to/config

# Disable colored output
kkonf --no-color
```

## Screenshots

### Main Menu
```
⚙️ kkonf v1.1.1 - kubectl Config Manager

📁 Config file: /Users/user/.kube/config
🎯 Current context: production-cluster
✓ Status: Saved

? Select option: 
  ❯ 1. 🏢 Clusters (3)
    2. 👤 Users (5) 
    3. 🌐 Contexts (4)
    4. 🔧 Tools
    5. ⚙️ Settings
    6. 💾 Save Configuration
    0. 🚪 Exit
```

### Cluster Management
```
🏢 Cluster Management

┌────┬─────────────────────┬─────────────────────────┬────────┐
│ #  │ Name                │ Server                  │ Secure │
├────┼─────────────────────┼─────────────────────────┼────────┤
│ 1  │ production-cluster  │ https://10.0.0.1:6443   │ Yes    │
│ 2  │ staging-cluster     │ https://10.0.0.2:6443   │ Yes    │
│ 3  │ dev-cluster         │ http://localhost:8080    │ No     │
└────┴─────────────────────┴─────────────────────────┴────────┘

? Select action:
  ❯ 1. ➕ Add Cluster
    2. ✏️ Edit Cluster
    3. 🗑️ Delete Cluster
    4. 👁️ View Details
    0. ← Back
```

### User Consolidation
```
🔄 User Consolidation

Found duplicate users:

Group 1: GKE Authentication (3 duplicates)
  - gke_project1_cluster1
  - gke_project1_cluster2  
  - gke_project2_cluster1
  
  Auth Method: exec (gke-gcloud-auth-plugin)
  Used by contexts: prod-gke, staging-gke, dev-gke

? Consolidate this group? Yes
? New user name: gke-user

✓ Consolidated 3 users into 'gke-user'
✓ Updated 3 context references
```

### Tools Menu
```
🔧 Tools

? Select tool:
  ❯ 1. ✓ Validate Configuration
    2. 🔄 Consolidate Duplicate Users
    3. 📥 Import Configuration
    4. 📤 Export Configuration
    5. ⚡ Quick Context Switch
    6. 🗑️ Clean Old Backups
    0. ← Back
```

## Main Menu Options

1. **🏢 Clusters**: Manage cluster configurations
   - Add new clusters
   - Edit existing clusters (server URL, certificates, TLS settings)
   - Delete clusters (with dependency checking)
   - View cluster details

2. **👤 Users**: Manage user authentication
   - Add users with various auth methods:
     - Exec (command-based authentication)
     - Token (direct or file-based)
     - Certificate (base64 or file paths)
     - Basic authentication (username/password)
   - Edit user authentication settings
   - Delete users (with dependency checking)
   - Consolidate duplicate users

3. **🌐 Contexts**: Manage context configurations
   - Add new contexts (linking clusters and users)
   - Edit context settings
   - Delete contexts
   - Switch current context
   - Set namespace for contexts

4. **🔧 Tools**: Additional utilities
   - Validate configuration
   - Consolidate duplicate users
   - Import configurations
   - Export configurations
   - Quick context switch
   - Clean old backups

5. **⚙️ Settings**: Application settings (coming soon)

## User Consolidation

One of kkonf's key features is the ability to consolidate duplicate users. This is particularly useful when managing multiple GKE/EKS/AKS clusters that use the same authentication method.

### How it works:
1. kkonf scans all users and groups those with identical properties
2. You select which groups to consolidate
3. Choose a new name for the consolidated user
4. All context references are automatically updated

### Example:
Before consolidation:
```yaml
users:
- name: gke_project1_cluster1
  user:
    exec:
      command: gke-gcloud-auth-plugin
      ...
- name: gke_project1_cluster2
  user:
    exec:
      command: gke-gcloud-auth-plugin
      ...
- name: gke_project2_cluster1
  user:
    exec:
      command: gke-gcloud-auth-plugin
      ...
```

After consolidation:
```yaml
users:
- name: gke-user
  user:
    exec:
      command: gke-gcloud-auth-plugin
      ...
```

All contexts are automatically updated to reference `gke-user`.

## Import/Export

### Import Options:
- **Skip conflicts**: Import only non-conflicting items
- **Replace**: Overwrite existing items with imported ones
- **Rename**: Import conflicting items with new names
- **Interactive**: Decide for each conflict individually

### Export Options:
- **Selected items**: Choose specific clusters, users, and contexts
- **All**: Export entire configuration
- **Current context**: Export current context with its dependencies

## Configuration Validation

kkonf automatically validates your configuration to detect:
- Missing cluster or user references in contexts
- Invalid server URLs
- Duplicate names
- Orphaned clusters or users (not referenced by any context)
- Missing authentication methods

## Backup Management

- Automatic backup creation before any modification
- Backups are timestamped: `config.bak.YYYYMMDD-HHMMSS`
- Clean old backups through the Tools menu
- Manual restore available if needed

## Color Scheme & Icons

- **🏢 Blue**: Clusters
- **👤 Green**: Users
- **🌐 Yellow**: Contexts
- **Bold with asterisk (*)**: Current context
- **Red**: Errors
- **Orange**: Warnings

## Navigation

- **Arrow Keys**: Navigate menu options
- **Enter**: Select option
- **ESC**: Go back
- **Numbers**: Quick selection in numbered lists

## Security Considerations

- Config files are saved with `0600` permissions (read/write for owner only)
- Sensitive data (tokens, certificates) are handled carefully
- Backups maintain the same permissions as the original file
- No data is sent to external services

## Future Enhancements

The following features are planned for future releases:

- **Cloud Integration**: Direct integration with GKE/EKS/AKS for automatic cluster discovery
- **kubectl Plugin**: Install as a kubectl plugin (`kubectl kkonf`)
- **Config Templates**: Pre-defined templates for common setups
- **Multi-file Management**: Support for multiple config files and merging
- **Remote Sync**: Synchronize configurations across multiple machines
- **Live Validation**: Real-time validation using Kubernetes API
- **Config Encryption**: Encrypt sensitive configuration data
- **Audit Logging**: Track all configuration changes
- **Undo/Redo**: Ability to undo recent changes
- **Config Diff Tools**: Compare configurations and show differences
- **Settings Persistence**: User preferences and application settings
- **Namespace Discovery**: Auto-discover available namespaces from clusters

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

MIT License - See LICENSE file for details

## Support

For issues, questions, or suggestions, please open an issue on the GitHub repository.