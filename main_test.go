package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestRandomString tests the randomString generator function
func TestRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"zero length", 0},
		{"short string", 4},
		{"medium string", 10},
		{"long string", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := randomString(tt.length)

			// Check length is correct
			if len(result) != tt.length {
				t.Errorf("randomString(%d) length = %d, want %d", tt.length, len(result), tt.length)
			}

			// Check all characters are valid (lowercase letters and digits only)
			const validChars = "abcdefghijklmnopqrstuvwxyz0123456789"
			for _, char := range result {
				if !strings.ContainsRune(validChars, char) {
					t.Errorf("randomString(%d) contains invalid character: %c", tt.length, char)
				}
			}
		})
	}
}

// TestRandomStringUniqueness verifies that multiple calls produce different results
func TestRandomStringUniqueness(t *testing.T) {
	length := 10
	iterations := 100
	results := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		result := randomString(length)
		results[result] = true
	}

	// We expect high uniqueness (at least 95% unique out of 100 calls)
	uniqueCount := len(results)
	if uniqueCount < 95 {
		t.Errorf("randomString uniqueness too low: got %d unique strings out of %d calls", uniqueCount, iterations)
	}
}

// TestParseVMNames tests VM name extraction from multipass list output
func TestParseVMNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty output",
			input:    "",
			expected: []string{},
		},
		{
			name: "single VM",
			input: `Name                    State             IPv4             Image
vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS`,
			expected: []string{"vm1"},
		},
		{
			name: "multiple VMs with different states",
			input: `Name                    State             IPv4             Image
vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS
vm2                     Stopped           --               Ubuntu 20.04 LTS
vm3                     Suspended         192.168.64.3     Ubuntu 24.04 LTS`,
			expected: []string{"vm1", "vm2", "vm3"},
		},
		{
			name: "with header and separator",
			input: `Name                    State             IPv4             Image
-----------------------------------------------------------
vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS`,
			expected: []string{"vm1"},
		},
		{
			name: "empty lines and whitespace",
			input: `Name                    State             IPv4             Image

vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS

vm2                     Stopped           --               Ubuntu 20.04 LTS
`,
			expected: []string{"vm1", "vm2"},
		},
		{
			name: "only header no VMs",
			input: `Name                    State             IPv4             Image`,
			expected: []string{},
		},
		{
			name: "malformed line with insufficient fields",
			input: `Name                    State             IPv4             Image
vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS
incomplete-line
vm2                     Stopped           --               Ubuntu 20.04 LTS`,
			expected: []string{"vm1", "vm2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVMNames(tt.input)

			// Check length matches
			if len(result) != len(tt.expected) {
				t.Errorf("parseVMNames() returned %d VMs, expected %d\nGot: %v\nWant: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}

			// Check each VM name matches
			for i, vmName := range result {
				if vmName != tt.expected[i] {
					t.Errorf("parseVMNames() VM[%d] = %q, want %q", i, vmName, tt.expected[i])
				}
			}
		})
	}
}

// TestParseSnapshots tests snapshot parsing from multipass list --snapshots output
func TestParseSnapshots(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []SnapshotInfo
	}{
		{
			name:     "empty output",
			input:    "",
			expected: []SnapshotInfo{},
		},
		{
			name: "single snapshot without parent or comment",
			input: `Instance    Snapshot    Parent    Comment
vm1         snap1       --        --`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "snap1", Parent: "", Comment: ""},
			},
		},
		{
			name: "single snapshot with parent and comment",
			input: `Instance    Snapshot    Parent    Comment
vm1         snap2       snap1     after-update`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "snap2", Parent: "snap1", Comment: "after-update"},
			},
		},
		{
			name: "multiple snapshots for same VM",
			input: `Instance    Snapshot    Parent    Comment
vm1         base        --        initial-state
vm1         snap1       base      after-install
vm1         snap2       snap1     final`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "base", Parent: "", Comment: "initial-state"},
				{Instance: "vm1", Name: "snap1", Parent: "base", Comment: "after-install"},
				{Instance: "vm1", Name: "snap2", Parent: "snap1", Comment: "final"},
			},
		},
		{
			name: "snapshots for multiple VMs",
			input: `Instance    Snapshot    Parent    Comment
vm1         snap1       --        test
vm2         backup      --        before-upgrade
vm2         snap2       backup    after-upgrade`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "snap1", Parent: "", Comment: "test"},
				{Instance: "vm2", Name: "backup", Parent: "", Comment: "before-upgrade"},
				{Instance: "vm2", Name: "snap2", Parent: "backup", Comment: "after-upgrade"},
			},
		},
		{
			name: "only header line",
			input: `Instance    Snapshot    Parent    Comment`,
			expected: []SnapshotInfo{},
		},
		{
			name: "with empty lines",
			input: `Instance    Snapshot    Parent    Comment

vm1         snap1       --        --

vm2         snap2       --        test
`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "snap1", Parent: "", Comment: ""},
				{Instance: "vm2", Name: "snap2", Parent: "", Comment: "test"},
			},
		},
		{
			name: "partial data (only instance and snapshot name)",
			input: `Instance    Snapshot    Parent    Comment
vm1         snap1`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "snap1", Parent: "", Comment: ""},
			},
		},
		{
			name: "malformed line with insufficient fields",
			input: `Instance    Snapshot    Parent    Comment
vm1         snap1       --        comment1
invalid
vm2         snap2       --        comment2`,
			expected: []SnapshotInfo{
				{Instance: "vm1", Name: "snap1", Parent: "", Comment: "comment1"},
				{Instance: "vm2", Name: "snap2", Parent: "", Comment: "comment2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSnapshots(tt.input)

			// Check length matches
			if len(result) != len(tt.expected) {
				t.Errorf("parseSnapshots() returned %d snapshots, expected %d\nGot: %+v\nWant: %+v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}

			// Check each snapshot matches
			for i, snapshot := range result {
				expected := tt.expected[i]
				if snapshot.Instance != expected.Instance {
					t.Errorf("Snapshot[%d].Instance = %q, want %q", i, snapshot.Instance, expected.Instance)
				}
				if snapshot.Name != expected.Name {
					t.Errorf("Snapshot[%d].Name = %q, want %q", i, snapshot.Name, expected.Name)
				}
				if snapshot.Parent != expected.Parent {
					t.Errorf("Snapshot[%d].Parent = %q, want %q", i, snapshot.Parent, expected.Parent)
				}
				if snapshot.Comment != expected.Comment {
					t.Errorf("Snapshot[%d].Comment = %q, want %q", i, snapshot.Comment, expected.Comment)
				}
			}
		})
	}
}

// BenchmarkRandomString measures performance of random string generation
func BenchmarkRandomString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		randomString(10)
	}
}

// BenchmarkParseVMNames measures parsing performance
func BenchmarkParseVMNames(b *testing.B) {
	input := `Name                    State             IPv4             Image
vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS
vm2                     Stopped           --               Ubuntu 20.04 LTS
vm3                     Suspended         192.168.64.3     Ubuntu 24.04 LTS`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseVMNames(input)
	}
}

// BenchmarkParseSnapshots measures snapshot parsing performance
func BenchmarkParseSnapshots(b *testing.B) {
	input := `Instance    Snapshot    Parent    Comment
vm1         snap1       --        initial
vm1         snap2       snap1     updated
vm2         backup      --        test`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseSnapshots(input)
	}
}

// TestParseVMNamesStressTest tests parsing with many VMs
func TestParseVMNamesStressTest(t *testing.T) {
	// Build input with 100 VMs
	var builder strings.Builder
	builder.WriteString("Name                    State             IPv4             Image\n")

	for i := 0; i < 100; i++ {
		builder.WriteString(fmt.Sprintf("vm%d                    Running           192.168.64.%d     Ubuntu 22.04 LTS\n", i, i))
	}

	result := parseVMNames(builder.String())

	if len(result) != 100 {
		t.Errorf("parseVMNames() stress test: got %d VMs, want 100", len(result))
	}

	// Verify first and last
	if result[0] != "vm0" {
		t.Errorf("First VM = %q, want vm0", result[0])
	}
	if result[99] != "vm99" {
		t.Errorf("Last VM = %q, want vm99", result[99])
	}
}

// TestParseSnapshotsComplexHierarchy tests complex parent-child relationships
func TestParseSnapshotsComplexHierarchy(t *testing.T) {
	input := `Instance    Snapshot    Parent    Comment
vm1         base        --        initial
vm1         dev1        base      development
vm1         dev2        dev1      feature-x
vm1         dev3        dev2      feature-y
vm1         prod1       base      production
vm1         prod2       prod1     hotfix`

	result := parseSnapshots(input)

	// Should have 6 snapshots total
	if len(result) != 6 {
		t.Fatalf("Expected 6 snapshots, got %d", len(result))
	}

	// Verify hierarchy
	snapMap := make(map[string]SnapshotInfo)
	for _, snap := range result {
		snapMap[snap.Name] = snap
	}

	// Check base has no parent
	if snapMap["base"].Parent != "" {
		t.Errorf("base.Parent = %q, want empty", snapMap["base"].Parent)
	}

	// Check dev chain
	if snapMap["dev1"].Parent != "base" {
		t.Errorf("dev1.Parent = %q, want base", snapMap["dev1"].Parent)
	}
	if snapMap["dev2"].Parent != "dev1" {
		t.Errorf("dev2.Parent = %q, want dev1", snapMap["dev2"].Parent)
	}
	if snapMap["dev3"].Parent != "dev2" {
		t.Errorf("dev3.Parent = %q, want dev2", snapMap["dev3"].Parent)
	}

	// Check prod chain
	if snapMap["prod1"].Parent != "base" {
		t.Errorf("prod1.Parent = %q, want base", snapMap["prod1"].Parent)
	}
	if snapMap["prod2"].Parent != "prod1" {
		t.Errorf("prod2.Parent = %q, want prod1", snapMap["prod2"].Parent)
	}
}

// TestSnapshotInfoEquality tests snapshot comparison
func TestSnapshotInfoEquality(t *testing.T) {
	snap1 := SnapshotInfo{
		Instance: "vm1",
		Name:     "snap1",
		Parent:   "base",
		Comment:  "test",
	}

	snap2 := SnapshotInfo{
		Instance: "vm1",
		Name:     "snap1",
		Parent:   "base",
		Comment:  "test",
	}

	snap3 := SnapshotInfo{
		Instance: "vm1",
		Name:     "snap1",
		Parent:   "base",
		Comment:  "different",
	}

	if snap1 != snap2 {
		t.Error("Identical snapshots should be equal")
	}

	if snap1 == snap3 {
		t.Error("Snapshots with different comments should not be equal")
	}
}

// TestParseVMNamesWithUnicode tests handling of non-ASCII characters
func TestParseVMNamesWithUnicode(t *testing.T) {
	input := `Name                    State             IPv4             Image
vm-测试                  Running           192.168.64.2     Ubuntu 22.04 LTS
vm-café                 Stopped           --               Ubuntu 20.04 LTS
vm-🚀                   Suspended         192.168.64.3     Ubuntu 24.04 LTS`

	result := parseVMNames(input)

	expected := []string{"vm-测试", "vm-café", "vm-🚀"}

	if len(result) != len(expected) {
		t.Fatalf("Got %d VMs, want %d", len(result), len(expected))
	}

	for i, vmName := range result {
		if vmName != expected[i] {
			t.Errorf("VM[%d] = %q, want %q", i, vmName, expected[i])
		}
	}
}

// TestRandomStringDistribution tests character distribution
func TestRandomStringDistribution(t *testing.T) {
	// Generate many strings and check that all charset characters appear
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"
	charCount := make(map[rune]int)

	iterations := 1000
	length := 20

	for i := 0; i < iterations; i++ {
		s := randomString(length)
		for _, c := range s {
			charCount[c]++
		}
	}

	// Every character in charset should appear at least once in 1000 iterations
	for _, c := range charset {
		if charCount[c] == 0 {
			t.Errorf("Character %c never appeared in %d iterations", c, iterations)
		}
	}

	// Total character count should equal iterations * length
	totalChars := 0
	for _, count := range charCount {
		totalChars += count
	}

	expected := iterations * length
	if totalChars != expected {
		t.Errorf("Total characters = %d, want %d", totalChars, expected)
	}
}

// TestParseVMNamesRobustness tests parser robustness with unusual input
func TestParseVMNamesRobustness(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "very long VM name",
			input:    "Name                    State             IPv4             Image\n" +
				"this-is-a-very-long-vm-name-that-exceeds-normal-length     Running           192.168.64.2     Ubuntu 22.04 LTS",
			expected: []string{"this-is-a-very-long-vm-name-that-exceeds-normal-length"},
		},
		{
			name:     "VM name with special characters",
			input:    "Name                    State             IPv4             Image\n" +
				"vm-with-dashes          Running           192.168.64.2     Ubuntu 22.04 LTS\n" +
				"vm_with_underscores     Stopped           --               Ubuntu 20.04 LTS\n" +
				"vm.with.dots            Running           192.168.64.3     Ubuntu 24.04 LTS",
			expected: []string{"vm-with-dashes", "vm_with_underscores", "vm.with.dots"},
		},
		{
			name:     "mixed IPv4 formats",
			input:    "Name                    State             IPv4             Image\n" +
				"vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS\n" +
				"vm2                     Running           10.0.0.5,192.168.1.10  Ubuntu 20.04 LTS\n" +
				"vm3                     Running           N/A              Ubuntu 24.04 LTS",
			expected: []string{"vm1", "vm2", "vm3"},
		},
		{
			name:     "extra whitespace between columns",
			input:    "Name                    State             IPv4             Image\n" +
				"vm1                     Running           192.168.64.2     Ubuntu 22.04 LTS",
			expected: []string{"vm1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVMNames(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Got %d VMs, want %d\nGot: %v\nWant: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}

			for i, vmName := range result {
				if vmName != tt.expected[i] {
					t.Errorf("VM[%d] = %q, want %q", i, vmName, tt.expected[i])
				}
			}
		})
	}
}
