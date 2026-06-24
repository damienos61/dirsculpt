# DirSculpt 📊

**Ultra-lightweight Disk Space Analysis & Predictive Simulation Tool**

## ✨ Features

- ⚡ Optimized for low-resource systems (Dual-Core, 4GB RAM, HDD)
- 🔍 Real-time async directory scanning with goroutines
- 🎯 Intelligent file type detection via magic numbers (MIME)
- 📈 Predictive space compression simulation without disk writes
- 👻 **Ghost Mode**: Virtual deletion simulation before committing changes
- 🚨 **Redundancy Detection**: Highlights cache/build directories
- 🔧 **Action Scripts**: Auto-generates safe bash/PowerShell scripts
- 🖥️ **Cross-Platform**: Linux, macOS, Windows support
- 💪 **60 FPS TUI**: Powered by Bubbletea framework

## 🚀 Quick Start

### Build from Source

```bash
# Clone repository
git clone https://github.com/damienos61/dirsculpt.git
cd dirsculpt

# Install dependencies
go mod download

# Build
make build

# Run
./dirsculpt
```

### Build for Multiple Platforms

```bash
make build-all
# Produces: dirsculpt-linux, dirsculpt-macos, dirsculpt.exe
```

## ⌨️ Controls

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate directory tree |
| `g` | Toggle Ghost Mode (simulation) |
| `z` | Toggle Compression Simulation |
| `Ctrl+X` | Generate action script |
| `Esc` | Quit |

## 🎯 Core Features

### Ghost Mode 👻
Virtually mark files/directories for deletion. The interface recalculates space as if they were removed without actually modifying anything.

**Use Case**: "What would my disk look like if I deleted node_modules and .cache?"

### Compression Simulation 📦
- **Text files** (.log, .txt, .csv): ~75% reduction via DEFLATE
- **Images** (.bmp): Estimates PNG/JPEG conversion savings
- **Already compressed** (.zip, .mp4, .png): Marked "Optimized"

### Redundancy Detection 🚨
Auto-highlights cache/build directories safe to delete:
```
.cache, node_modules, __pycache__
.git, venv, vendor, build, dist, target
.gradle, .m2, tmp, temp, .DS_Store
```

### Action Scripts 🔧
Press `Ctrl+X` to generate platform-specific scripts:
- **Linux/macOS**: Bash script with safety prompts
- **Windows**: PowerShell script with confirmations
- Automatic backups before any operations
- Safe to review before execution

## 📊 Architecture

```
main.go           - App initialization & lifecycle
scanner.go        - Async directory scanning with context
analyzer.go       - MIME detection & compression simulation
tui.go            - Bubbletea UI rendering (60 FPS)
events.go         - Keyboard event handling
action_generator  - Script generation (bash/PowerShell)
```

## 🔧 Technical Details

### Memory Optimization
- Max 2 concurrent goroutines (dual-core target)
- Bounded channels (256 scan, 256 analysis)
- Stream-based processing (no full tree in memory)

### Performance Metrics
- Scan speed: 10,000+ files/second
- Memory footprint: ~15-20 MB typical
- UI responsiveness: 60 FPS maintained
- HDD-optimized (semaphore-limited I/O)

### File Type Detection
Uses magic bytes (first 8 bytes) for accurate MIME detection:
```
ZIP:   50 4B 03 04
PNG:   89 50 4E 47
JPEG:  FF D8 FF
BMP:   42 4D
GIF:   47 49 46
GZIP:  1F 8B
```

## 📝 Usage Examples

### Analyze home directory
```bash
./dirsculpt ~
```

### Analyze project directory
```bash
./dirsculpt /path/to/project
```

### Generate cleanup script
```bash
# 1. Run DirSculpt
./dirsculpt .

# 2. Enable Ghost Mode (g)
# 3. Enable Compression (z)
# 4. Generate script (Ctrl+X)
# 5. Review dirsculpt_actions_*.sh (or .ps1)
# 6. Execute: bash dirsculpt_actions_*.sh
```

## ⚠️ Safety

- ✅ No actual file modifications during analysis
- ✅ Ghost Mode is 100% safe (simulation only)
- ✅ Scripts are previewed before execution
- ✅ Automatic backups created in generated scripts
- ✅ Requires explicit confirmation to run scripts

## 🔨 Building Custom Binaries

```bash
# Smallest binary (strip symbols)
go build -ldflags="-s -w" -o dirsculpt

# With version info
VERSION=$(git describe --tags)
go build -ldflags="-X main.Version=$VERSION" -o dirsculpt

# Static binary (musl libc for maximum compatibility)
CGO_ENABLED=0 GOOS=linux go build -o dirsculpt-static
```

## 📦 Dependencies

- `github.com/charmbracelet/bubbletea`: TUI framework
- `github.com/charmbracelet/lipgloss`: Terminal styling

## 🐛 Troubleshooting

### High memory usage
- Try smaller directories first
- Use Ctrl+C to stop scanning
- Check for symbolic link loops

### UI not responsive
- Ensure terminal supports 256 colors
- Try increasing terminal window size
- Restart application

### Scripts not executable
- On Linux/macOS: `chmod +x dirsculpt_actions_*.sh`
- On Windows: Run PowerShell as Administrator

## 📄 License

MIT License - See LICENSE file

## 👤 Author

Created for ultra-low-resource systems with ⚡ optimization

---

**⭐ Star this project if it helps you!**
