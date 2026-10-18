package dbmigration

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var migrationFilename = regexp.MustCompile(`^([0-9]+)_.+\.(up|down)\.sql$`)

// Run applies the SQL migrations at path. A zero target migrates to the latest
// version; a non-zero target is intended for controlled upgrade verification.
func Run(databaseURL, path string, target uint) error {
	return RunFrom(databaseURL, path, 0, target)
}

// RunFrom limits the migration source to versions at or above minimumVersion.
// SparkOps production uses 70 because earlier application schemas historically
// came from the GORM bootstrap rather than a complete SQL migration chain.
func RunFrom(databaseURL, path string, minimumVersion, target uint) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve migrations path: %w", err)
	}
	normalizedPath, cleanup, err := normalizeSource(absolutePath, minimumVersion)
	if err != nil {
		return err
	}
	defer cleanup()
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(normalizedPath)}).String()

	runner, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		_, _ = runner.Close()
	}()

	if target == 0 {
		err = runner.Up()
	} else {
		err = runner.Migrate(target)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

type migrationFiles struct {
	up   []string
	down []string
}

// normalizeSource preserves repositories that historically assigned the same
// version to more than one migration. Up files are combined in lexical order;
// down files are combined in reverse order so rollback remains symmetric.
func normalizeSource(path string, minimumVersion uint) (string, func(), error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", func() {}, fmt.Errorf("read migrations path: %w", err)
	}
	grouped := make(map[uint64]*migrationFiles)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := migrationFilename.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		version, parseErr := strconv.ParseUint(match[1], 10, 64)
		if parseErr != nil {
			return "", func() {}, fmt.Errorf("parse migration version %q: %w", match[1], parseErr)
		}
		if version < uint64(minimumVersion) {
			continue
		}
		files := grouped[version]
		if files == nil {
			files = &migrationFiles{}
			grouped[version] = files
		}
		if match[2] == "up" {
			files.up = append(files.up, entry.Name())
		} else {
			files.down = append(files.down, entry.Name())
		}
	}
	if len(grouped) == 0 {
		return "", func() {}, fmt.Errorf("no SQL migrations found in %s", path)
	}

	normalized, err := os.MkdirTemp("", "sparkops-migrations-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create normalized migrations path: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(normalized) }
	for version, files := range grouped {
		sort.Strings(files.up)
		sort.Sort(sort.Reverse(sort.StringSlice(files.down)))
		if err := writeCombinedMigration(path, normalized, version, "up", files.up); err != nil {
			cleanup()
			return "", func() {}, err
		}
		if err := writeCombinedMigration(path, normalized, version, "down", files.down); err != nil {
			cleanup()
			return "", func() {}, err
		}
	}
	if minimumVersion > 0 {
		baseline := minimumVersion - 1
		for _, direction := range []string{"up", "down"} {
			name := fmt.Sprintf("%06d_baseline.%s.sql", baseline, direction)
			if err := os.WriteFile(
				filepath.Join(normalized, name),
				[]byte("-- SparkOps SQL migration baseline\nSELECT 1;\n"),
				0o600,
			); err != nil {
				cleanup()
				return "", func() {}, fmt.Errorf("write migration baseline %s: %w", name, err)
			}
		}
	}
	return normalized, cleanup, nil
}

func writeCombinedMigration(source, destination string, version uint64, direction string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	var combined strings.Builder
	for _, name := range files {
		contents, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		fmt.Fprintf(&combined, "-- SparkOps source: %s\n", name)
		combined.Write(contents)
		combined.WriteString("\n")
	}
	name := fmt.Sprintf("%06d_combined.%s.sql", version, direction)
	if err := os.WriteFile(filepath.Join(destination, name), []byte(combined.String()), 0o600); err != nil {
		return fmt.Errorf("write normalized migration %s: %w", name, err)
	}
	return nil
}
