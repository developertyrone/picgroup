# PicGroup

PicGroup is a high-performance file organizer that sorts images and other media files based on their creation date from EXIF metadata.

## Features

- **Smart Organization**: Automatically detects creation dates from EXIF metadata
- **Flexible Folder Structure**: Organize by Year-Month-Day or Year-Month formats
- **Performance Options**: Run in sequential or concurrent mode for optimal performance
- **File Handling**: Choose between copying or moving files
- **Verbose Logging**: View detailed operation logs when needed

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/developertyrone/picgroup.git

# Build the application
cd picgroup
go build -o picgroup ./cmd/main.go
```

### Using Go Install

```bash
go install github.com/developertyrone/picgroup@latest
```

### Cross-Platform Builds

Go supports cross-compilation for different operating systems and architectures. Use the following commands to build for various platforms:

#### Windows
```bash
GOOS=windows GOARCH=amd64 go build -o picgroup.exe ./cmd/main.go
```

#### macOS
```bash
GOOS=darwin GOARCH=amd64 go build -o picgroup ./cmd/main.go
```

#### Linux
```bash
GOOS=linux GOARCH=amd64 go build -o picgroup ./cmd/main.go
```

## Usage

Basic usage:

```bash
./picgroup -d /path/to/photos
```

### Examples

Copy files with concurrent processing:
```bash
./picgroup -d /path/to/photos -m con -v 1
```

Move files and organize by year-month:
```bash
./picgroup -d /path/to/photos -g move -f ym -m con -v 1
```

Move files and organize by year-month-day:
```bash
./picgroup -d /path/to/photos -g move -f ymd -m con -v 1
```

## Command Line Options

| Flag | Description | Default | Options |
|------|-------------|---------|---------|
| `-d` | Source directory containing media files | Required | Valid directory path |
| `-f` | Folder format | `ymd` | `ymd` (Year-Month-Day), `ym` (Year-Month) |
| `-g` | Group mode | `copy` | `copy`, `move` |
| `-m` | Processing mode | `seq` | `seq` (Sequential), `con` (Concurrent) |
| `-v` | Verbose output | `0` | `0` (Disabled), `1` (Enabled) |

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific tests
go test -v -run TestGenFolder github.com/developertyrone/picgroup/pkg/organizer

# Run benchmarks
go test -bench=. github.com/developertyrone/picgroup/pkg/organizer
```

### Performance

The concurrent mode (`-m con`) significantly improves performance when organizing large collections of files by utilizing available CPU cores.

## License

[MIT License](LICENSE)