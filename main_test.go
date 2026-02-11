package main

import (
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
