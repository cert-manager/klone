/*
Copyright 2026 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skipIfNoSymlinks probes whether the current process/OS can create a
// symlink and skips the test if it cannot. Windows without Developer Mode
// (or sandboxed CI without the symlink privilege) returns EPERM/ENOTSUP
// from os.Symlink, which would otherwise mask the real assertion under a
// setup failure.
func skipIfNoSymlinks(t *testing.T) {
	t.Helper()
	probe := t.TempDir()
	if err := os.Symlink(probe, filepath.Join(probe, "probe")); err != nil {
		t.Skipf("skip: symlinks unsupported in this environment: %v", err)
	}
}

// TestSyncFolder_TargetSymlinkRejected is the regression for the VC-53816
// root-symlink variant: when the target directory itself (e.g. `vendored`)
// is a pre-planted symlink to an attacker-chosen location, SyncFolder must
// refuse rather than letting Cleanup/MkdirAll/rsync dereference it.
func TestSyncFolder_TargetSymlinkRejected(t *testing.T) {
	skipIfNoSymlinks(t)
	sb := t.TempDir()
	workDir := filepath.Join(sb, "project")
	decoy := filepath.Join(sb, "decoy")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("setup workDir: %v", err)
	}
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatalf("setup decoy: %v", err)
	}
	sentinel := filepath.Join(decoy, "important")
	if err := os.WriteFile(sentinel, []byte("VICTIM"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	// Hostile pre-state: workDir/vendored is a symlink to the decoy dir.
	if err := os.Symlink(decoy, filepath.Join(workDir, "vendored")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	manifest := `targets:
  vendored:
    - folder_name: subdir
      repo_url: /nonexistent-klone-poc-repo
      repo_ref: main
      repo_hash: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
      repo_path: .
`
	if err := os.WriteFile(filepath.Join(workDir, "klone.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("KLONE_CACHE_DIR", filepath.Join(sb, "cache"))
	err := SyncFolder(t.Context(), workDir, false)
	if err == nil {
		t.Fatalf("SyncFolder returned nil, want symlink-refusal error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("SyncFolder error = %q, want substring 'symlink'", err.Error())
	}

	// Sentinel inside the decoy must be untouched — Cleanup must not have
	// walked through the symlink.
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("VC-53816 root-symlink escape: decoy sentinel was removed: %v", err)
	}
}

// TestSplitFolderName covers the manifest-input parser that replaces the
// buggy filepath.SplitList call (VC-53818). The original split-on-PATH-sep
// behaviour turned "..:..:.." into three ".." segments; this parser
// produces a single literal segment for the same input and rejects every
// traversal/absolute/volume-prefixed shape.
func TestSplitFolderName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     []string
		wantErr  bool
		errMatch string
	}{
		// Valid shapes.
		{name: "single segment", input: "a", want: []string{"a"}},
		{name: "nested slash", input: "a/b/c", want: []string{"a", "b", "c"}},
		{name: "nested backslash", input: `a\b\c`, want: []string{"a", "b", "c"}},
		{name: "mixed separators", input: `a\b/c`, want: []string{"a", "b", "c"}},
		// VC-53818: the disclosed payload is now a literal single-segment
		// name, not three ".." traversal segments. Unusual but not a
		// traversal vector.
		{name: "colon payload kept as literal", input: "..:..:..", want: []string{"..:..:.."}},

		// Rejections.
		{name: "empty", input: "", wantErr: true, errMatch: "empty"},
		{name: "lone dot", input: ".", wantErr: true, errMatch: "traversal segment"},
		{name: "lone dotdot", input: "..", wantErr: true, errMatch: "traversal segment"},
		{name: "leading dotdot", input: "../etc", wantErr: true, errMatch: "traversal segment"},
		{name: "inner dotdot", input: "a/../etc", wantErr: true, errMatch: "traversal segment"},
		{name: "empty segment", input: "a//b", wantErr: true, errMatch: "empty or traversal"},
		{name: "absolute unix", input: "/etc/passwd", wantErr: true, errMatch: "absolute"},
		{name: "windows drive upper", input: `C:\tmp`, wantErr: true, errMatch: "Windows volume"},
		{name: "windows drive lower", input: "c:/tmp", wantErr: true, errMatch: "Windows volume"},
		// UNC paths: backslash normalisation turns `\\srv\share\x` into
		// `//srv/share/x`, which IsAbs catches as absolute on POSIX and
		// VolumeName catches on Windows.
		{name: "unc path", input: `\\server\share\x`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitFolderName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitFolderName(%q) = %v, nil; want error", tt.input, got)
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("splitFolderName(%q) error = %q, want substring %q", tt.input, err.Error(), tt.errMatch)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitFolderName(%q) returned unexpected error: %v", tt.input, err)
			}
			if !equalSegments(got, tt.want) {
				t.Errorf("splitFolderName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestTreeNodeCleanup_PartialTreeIsNoOp pins the regression that surfaced
// alongside the VC-53818 splitter fix: once folder_name parses into a
// multi-segment tree, Cleanup recurses into intermediates that do not
// exist on first sync, and ReadDir errors with ENOENT. The fix turns
// ENOENT into a no-op; this test fails if that handling regresses.
func TestTreeNodeCleanup_PartialTreeIsNoOp(t *testing.T) {
	root := t.TempDir()
	tn := newTreeNode()
	tn.Add("a", "b", "c")
	if err := tn.Cleanup(root); err != nil {
		t.Errorf("Cleanup on tree with missing intermediates returned error: %v", err)
	}
}

func equalSegments(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSyncFolder_ColonPayloadDoesNotEscape is the end-to-end regression
// for VC-53818. Pre-fix, "..:..:.." would split into three ".." segments
// and Cleanup would RemoveAll directories above the working directory.
// Post-fix, the colon payload is a valid single literal segment, so
// validation passes and SyncFolder proceeds to a git fetch that fails on
// the bogus repo — but no escape occurs.
func TestSyncFolder_ColonPayloadDoesNotEscape(t *testing.T) {
	sb := t.TempDir()
	victim := filepath.Join(sb, "buffer", "L3", "L2", "L1", "victim")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	sentinels := []string{
		filepath.Join(sb, "buffer", "L3", "L2", "L1", "SENTINEL-L1"),
		filepath.Join(sb, "buffer", "L3", "L2", "SENTINEL-L2"),
		filepath.Join(sb, "buffer", "L3", "SENTINEL-L3"),
	}
	for _, p := range sentinels {
		if err := os.WriteFile(p, []byte("sentinel"), 0o644); err != nil {
			t.Fatalf("setup sentinel %s: %v", p, err)
		}
	}

	manifest := `targets:
  vendored:
    - folder_name: "..:..:.."
      repo_url: /nonexistent-klone-poc-repo
      repo_ref: main
      repo_hash: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
      repo_path: .
`
	if err := os.WriteFile(filepath.Join(victim, "klone.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("KLONE_CACHE_DIR", filepath.Join(sb, "cache"))
	// Bogus repo means SyncFolder must report a non-nil error. The real
	// CVE proof is that the sentinels above the working dir survive.
	if err := SyncFolder(t.Context(), victim, false); err == nil {
		t.Fatalf("SyncFolder returned nil for bogus manifest, want error")
	}

	for _, p := range sentinels {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("VC-53818 escape: sentinel %s was wrongfully removed: %v", p, err)
		}
	}
}
