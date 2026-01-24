package collision_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// CollisionInfo holds parsed collision details
type CollisionInfo struct {
	Message        string
	Field          string
	ExistingOrigin string
	ExistingEmpath string
	NewOrigin      string
	NewEmpath      string
}

// parseCollisions extracts collision information from protoc output
func parseCollisions(output string) []CollisionInfo {
	var collisions []CollisionInfo

	// Pattern: field collision detected {"message": "X", "field": "Y", ...}
	re := regexp.MustCompile(`field collision detected\s*\{[^}]*"message":\s*"([^"]+)"[^}]*"field":\s*"([^"]+)"[^}]*"existing_origin":\s*"([^"]+)"[^}]*"existing_empath":\s*"([^"]*)"[^}]*"new_origin":\s*"([^"]+)"[^}]*"new_empath":\s*"([^"]*)"`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) >= 7 {
			collisions = append(collisions, CollisionInfo{
				Message:        match[1],
				Field:          match[2],
				ExistingOrigin: match[3],
				ExistingEmpath: match[4],
				NewOrigin:      match[5],
				NewEmpath:      match[6],
			})
		}
	}

	return collisions
}

// TestCollisionDetection verifies that protoc-gen-go-plain correctly detects
// and reports field name collisions for various scenarios.
func TestCollisionDetection(t *testing.T) {
	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		t.Fatalf("Failed to find workspace root: %v", err)
	}

	pluginPath := filepath.Join(workspaceRoot, "bin", "protoc-gen-go-plain")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Fatalf("Plugin not found at %s. Run 'make build' first.", pluginPath)
	}

	testCases := []struct {
		name          string
		protoFile     string
		expectedError string
		description   string
	}{
		{
			name:          "EmbedCollision",
			protoFile:     "embed_collision.proto",
			expectedError: "collision",
			description:   "Two embedded messages with same field name",
		},
		{
			name:          "VirtualCollision",
			protoFile:     "virtual_collision.proto",
			expectedError: "collision",
			description:   "Virtual field conflicts with real field",
		},
		{
			name:          "PrefixCollision",
			protoFile:     "prefix_collision.proto",
			expectedError: "collision",
			description:   "Two embedded messages with same field name",
		},
		{
			name:          "DirectCollision",
			protoFile:     "direct_collision.proto",
			expectedError: "collision",
			description:   "Embedded field conflicts with direct field",
		},
		{
			name:          "RecursiveCollision",
			protoFile:     "recursive_collision.proto",
			expectedError: "collision",
			description:   "Recursive embed creates duplicate fields at different levels",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			protoPath := filepath.Join(workspaceRoot, "test", "collision", tc.protoFile)

			// Create temp directory for output
			tmpDir, err := os.MkdirTemp("", "collision_test_*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Run protoc with our plugin
			cmd := exec.Command("protoc",
				"--plugin=protoc-gen-go-plain="+pluginPath,
				"--go_out="+tmpDir,
				"--go_opt=paths=source_relative",
				"--go-plain_out="+tmpDir,
				"--go-plain_opt=paths=source_relative,json_jx=true",
				"--proto_path="+workspaceRoot,
				protoPath,
			)

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// We expect protoc to fail
			if err == nil {
				t.Errorf("%s: expected protoc to fail with collision error, but it succeeded.\nDescription: %s",
					tc.name, tc.description)
				return
			}

			// Verify the error message contains expected text
			if !strings.Contains(strings.ToLower(outputStr), tc.expectedError) {
				t.Errorf("%s: expected error containing %q, got:\n%s\nDescription: %s",
					tc.name, tc.expectedError, outputStr, tc.description)
				return
			}

			// Parse and log collision details
			collisions := parseCollisions(outputStr)
			t.Logf("=== %s ===", tc.name)
			t.Logf("Description: %s", tc.description)
			t.Logf("Proto file: %s", tc.protoFile)
			t.Logf("Collisions detected: %d", len(collisions))

			for i, c := range collisions {
				t.Logf("  [%d] Message: %s", i+1, c.Message)
				t.Logf("      Field: %q", c.Field)
				t.Logf("      Existing: origin=%s, empath=%q", c.ExistingOrigin, c.ExistingEmpath)
				t.Logf("      New:      origin=%s, empath=%q", c.NewOrigin, c.NewEmpath)
			}

			if len(collisions) == 0 {
				// Fallback: show raw output if parsing failed
				t.Logf("Raw error output:\n%s", outputStr)
			}
		})
	}
}

// findWorkspaceRoot finds the root of the workspace by looking for go.mod
func findWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
