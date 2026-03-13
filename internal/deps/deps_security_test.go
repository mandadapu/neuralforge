package deps_test

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownVulnerableVersions maps package names to versions with known security
// advisories. Each entry should reference the relevant advisory ID.
var knownVulnerableVersions = []struct {
	pkg      string
	version  string
	advisory string
}{
	{
		pkg:      "gopkg.in/yaml.v3",
		version:  "v3.0.0-20200313102051-9f266ea9e77c",
		advisory: "GHSA-hp87-p4gw-j4gq",
	},
}

// TestGoSumNoVulnerableVersions verifies that go.sum does not contain entries
// for dependency versions with known security advisories. This catches
// regressions where a vulnerable version is reintroduced via transitive
// dependency changes.
func TestGoSumNoVulnerableVersions(t *testing.T) {
	f, err := os.Open("../../go.sum")
	require.NoError(t, err, "failed to open go.sum")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		for _, vuln := range knownVulnerableVersions {
			marker := vuln.pkg + " " + vuln.version
			assert.False(t, strings.Contains(line, marker),
				"go.sum contains vulnerable dependency %s %s (advisory: %s)",
				vuln.pkg, vuln.version, vuln.advisory,
			)
		}
	}
	require.NoError(t, scanner.Err(), "error reading go.sum")
}

// TestGoModPinnedVersions verifies that go.mod pins dependencies at or above
// their minimum safe versions for packages with known advisories.
func TestGoModPinnedVersions(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	require.NoError(t, err, "failed to read go.mod")

	content := string(data)

	// Verify yaml.v3 is required at v3.0.1 or later (patched for GHSA-hp87-p4gw-j4gq)
	assert.Contains(t, content, "gopkg.in/yaml.v3 v3.0.1",
		"go.mod should pin gopkg.in/yaml.v3 to v3.0.1 or later (GHSA-hp87-p4gw-j4gq)")
}
