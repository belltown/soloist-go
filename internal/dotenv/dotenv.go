// Package dotenv loads simple KEY=VALUE environment files, without
// overriding variables already set in the process environment.
package dotenv

import (
	"bufio"
	"os"
	"strings"
)

// Load reads the file at path and calls os.Setenv for each KEY=VALUE line,
// skipping blank lines, "#" comments, and keys already present in the
// environment. It returns nil if path does not exist.
func Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		os.Setenv(key, value)
	}
	return scanner.Err()
}
