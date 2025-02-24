./picgroup -d <path> -m con -v 1
./picgroup -d /volume1/photo/ASUSCamera -g move -f ym -m con -v 1
./picgroup -d /volume1/photo/new -g move -f ymd -m con -v 1

# Run a single test function by name
go test -v -run TestGenFolder

# Run tests that match a pattern
go test -v -run "Test.*Folder"


# Navigate to the organizer package directory first
cd pkg/organizer

# Then run the tests
go test

# For verbose output, add -v flag
go test -v

# Run all benchmarks
go test -bench=.

# Run specific benchmark
go test -bench=BenchmarkFileOrganizerSequential

# Show coverage percentage
go test -cover

# Generate coverage profile
go test -coverprofile=coverage.out

# View coverage in browser
go tool cover -html=coverage.out

# Run all tests in all packages
go test ./...

# Run tests in specific package from anywhere
go test github.com/yourusername/fileorganizer/pkg/organizer
# or if your module is named 'fileorganizer'
go test fileorganizer/pkg/organizer