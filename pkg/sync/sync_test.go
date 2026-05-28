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

// TestSyncFolder_TargetSymlinkRejected is the regression for the VC-53816
// root-symlink variant: when the target directory itself (e.g. `vendored`)
// is a pre-planted symlink to an attacker-chosen location, SyncFolder must
// refuse rather than letting Cleanup/MkdirAll/rsync dereference it.
func TestSyncFolder_TargetSymlinkRejected(t *testing.T) {
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
