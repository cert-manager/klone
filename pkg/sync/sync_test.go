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
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSplitFolderName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "single segment", input: "a", want: []string{"a"}, wantErr: false},
		{name: "nested", input: "a/b", want: []string{"a", "b"}, wantErr: false},
		{name: "deeply nested", input: "a/b/c/d", want: []string{"a", "b", "c", "d"}, wantErr: false},

		// Backslashes are treated as separators on every GOOS.
		{name: "backslash separator", input: `a\b`, want: []string{"a", "b"}, wantErr: false},
		{name: "mixed separators", input: `a\b/c`, want: []string{"a", "b", "c"}, wantErr: false},

		// VC-53818 payloads are now literal names, not path-list entries.
		{name: "colon payload kept as literal", input: "..:..:..", want: []string{"..:..:.."}, wantErr: false},
		{name: "semicolon payload kept as literal", input: "..;..;..", want: []string{"..;..;.."}, wantErr: false},

		{name: "leading traversal", input: "../etc", wantErr: true},
		{name: "embedded traversal", input: "a/../b", wantErr: true},
		{name: "trailing traversal", input: "a/..", wantErr: true},
		{name: "current dir", input: "./a", wantErr: true},
		{name: "lone dot", input: ".", wantErr: true},
		{name: "lone dotdot", input: "..", wantErr: true},

		{name: "empty input", input: "", wantErr: true},
		{name: "leading slash", input: "/a", wantErr: true},
		{name: "trailing slash", input: "a/", wantErr: true},
		{name: "double slash", input: "a//b", wantErr: true},

		// Absolute and drive-qualified paths have no use in folder_name.
		{name: "windows absolute backslash", input: `C:\tmp`, wantErr: true},
		{name: "windows absolute forward", input: "C:/tmp", wantErr: true},
		{name: "windows volume only", input: "C:", wantErr: true},
		{name: "unc path", input: `\\server\share\x`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitFolderName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("splitFolderName(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("splitFolderName(%q) returned unexpected error: %v", tt.input, err)
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("splitFolderName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestSyncFolder_ColonPayloadDoesNotEscape is the end-to-end VC-53818 check.
func TestSyncFolder_ColonPayloadDoesNotEscape(t *testing.T) {
	sb := t.TempDir()
	victim := filepath.Join(sb, "buffer", "L3", "L2", "L1", "victim")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	mustTouch(t, filepath.Join(sb, "buffer", "L3", "L2", "L1", "SENTINEL-L1"))
	mustTouch(t, filepath.Join(sb, "buffer", "L3", "L2", "SENTINEL-L2"))
	mustTouch(t, filepath.Join(sb, "buffer", "L3", "SENTINEL-L3"))

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
	_ = SyncFolder(t.Context(), victim, false)

	for _, p := range []string{
		filepath.Join(sb, "buffer", "L3", "L2", "L1", "SENTINEL-L1"),
		filepath.Join(sb, "buffer", "L3", "L2", "SENTINEL-L2"),
		filepath.Join(sb, "buffer", "L3", "SENTINEL-L3"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("VC-53818 escape: sentinel %s was wrongfully removed: %v", p, err)
		}
	}
}

// Nested folder names should sync idempotently with either slash style.
func TestSyncFolder_NestedFolderNameSyncs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not on PATH")
	}

	cases := []struct {
		name       string
		folderName string
	}{
		{name: "forward slashes", folderName: "deeply/nested/path"},
		{name: "backslash separator", folderName: `deeply\nested\path`},
		{name: "mixed separators", folderName: `deeply\nested/path`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sb := t.TempDir()
			srv := filepath.Join(sb, "srv", "repo.git")
			work := filepath.Join(sb, "repo-work")
			project := filepath.Join(sb, "project")
			cache := filepath.Join(sb, "cache")
			for _, d := range []string{filepath.Dir(srv), work, project, cache} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", d, err)
				}
			}

			runGit := func(dir string, args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(),
					"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
					"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
				)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}

			runGit(filepath.Dir(srv), "init", "-q", "--bare", srv)
			runGit(work, "init", "-q")
			if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("hello"), 0o644); err != nil {
				t.Fatalf("write f.txt: %v", err)
			}
			runGit(work, "add", ".")
			runGit(work, "commit", "-qm", "a")
			runGit(work, "push", "-q", srv, "HEAD:main")

			hashOut, err := exec.Command("git", "-C", work, "rev-parse", "HEAD").Output()
			if err != nil {
				t.Fatalf("rev-parse: %v", err)
			}
			hash := strings.TrimSpace(string(hashOut))

			manifest := "targets:\n" +
				"  vendored:\n" +
				"    - folder_name: " + tc.folderName + "\n" +
				"      repo_url: " + srv + "\n" +
				"      repo_ref: main\n" +
				"      repo_hash: " + hash + "\n" +
				"      repo_path: .\n"
			if err := os.WriteFile(filepath.Join(project, "klone.yaml"), []byte(manifest), 0o644); err != nil {
				t.Fatalf("write manifest: %v", err)
			}

			t.Setenv("KLONE_CACHE_DIR", cache)

			if err := SyncFolder(t.Context(), project, false); err != nil {
				t.Fatalf("first SyncFolder failed for folder_name %q: %v", tc.folderName, err)
			}
			nested := filepath.Join(project, "vendored", "deeply", "nested", "path", "f.txt")
			if got, err := os.ReadFile(nested); err != nil {
				t.Fatalf("first sync: nested path not materialised: %v", err)
			} else if string(got) != "hello" {
				t.Errorf("first sync content = %q, want %q", got, "hello")
			}

			if err := SyncFolder(t.Context(), project, false); err != nil {
				t.Fatalf("second SyncFolder failed for folder_name %q: %v", tc.folderName, err)
			}
			if got, err := os.ReadFile(nested); err != nil {
				t.Fatalf("second sync: nested path was removed (cleanup/sync disagreed): %v", err)
			} else if string(got) != "hello" {
				t.Errorf("second sync content = %q, want %q", got, "hello")
			}
		})
	}
}

// Cleanup should reject unsafe children even if callers bypass parsing.
func TestTreeNodeCleanup_BoundedToRoot(t *testing.T) {
	for _, name := range []string{"..", ".", ""} {
		t.Run("child_"+name, func(t *testing.T) {
			tn := newTreeNode()
			tn.children[name] = newTreeNode()
			tn.children[name].isLeaf = true

			root := t.TempDir()
			parent := filepath.Dir(root)
			sentinel := filepath.Join(parent, "SENTINEL-CLEANUP-BOUNDED")
			if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
				t.Fatalf("plant sentinel: %v", err)
			}
			t.Cleanup(func() { _ = os.Remove(sentinel) })

			err := tn.Cleanup(root)
			if err == nil {
				t.Fatalf("Cleanup with child %q returned nil, want refusal error", name)
			}
			if !strings.Contains(err.Error(), "unsafe child name") {
				t.Errorf("Cleanup error = %q, want substring 'unsafe child name'", err.Error())
			}
			if _, statErr := os.Stat(sentinel); statErr != nil {
				t.Errorf("sentinel outside root was wrongfully removed: %v", statErr)
			}
		})
	}
}

func skipIfNoSymlinks(t *testing.T) {
	t.Helper()
	probe := t.TempDir()
	if err := os.Symlink(probe, filepath.Join(probe, "probe")); err != nil {
		t.Skipf("skip: symlinks unsupported in this environment: %v", err)
	}
}

func TestTreeNodeCleanup_RefusesSymlinkDescent(t *testing.T) {
	skipIfNoSymlinks(t)

	sb := t.TempDir()
	outside := filepath.Join(sb, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	sentinel := filepath.Join(outside, "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("VICTIM"), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}

	root := filepath.Join(sb, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "deeply")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	tn := newTreeNode()
	tn.Add("deeply", "nested", "path")

	err := tn.Cleanup(root)
	if err == nil {
		t.Fatalf("Cleanup returned nil, want symlink-refusal error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("Cleanup error = %q, want substring 'symlink'", err.Error())
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("VC-53818 symlink escape: sentinel outside root was removed: %v", statErr)
	}
}

// Expected intermediate directories must not be symlinks.
func TestSyncFolder_IntermediateSymlinkRejected(t *testing.T) {
	skipIfNoSymlinks(t)

	sb := t.TempDir()
	outside := filepath.Join(sb, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	sentinel := filepath.Join(outside, "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("VICTIM"), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}

	project := filepath.Join(sb, "project")
	if err := os.MkdirAll(filepath.Join(project, "vendored"), 0o755); err != nil {
		t.Fatalf("mkdir project/vendored: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(project, "vendored", "deeply")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	manifest := `targets:
  vendored:
    - folder_name: deeply/nested/path
      repo_url: /nonexistent-klone-poc-repo
      repo_ref: main
      repo_hash: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
      repo_path: .
`
	if err := os.WriteFile(filepath.Join(project, "klone.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("KLONE_CACHE_DIR", filepath.Join(sb, "cache"))
	err := SyncFolder(t.Context(), project, false)
	if err == nil {
		t.Fatalf("SyncFolder returned nil, want symlink-refusal error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("SyncFolder error = %q, want substring 'symlink'", err.Error())
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("VC-53818 escape: sentinel outside project was removed: %v", statErr)
	}
}

// Empty targets still run cleanup, so the target path must be checked.
func TestSyncFolder_EmptySrcsTargetSymlinkRejected(t *testing.T) {
	skipIfNoSymlinks(t)

	sb := t.TempDir()
	outside := filepath.Join(sb, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	sentinel := filepath.Join(outside, "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("VICTIM"), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}

	project := filepath.Join(sb, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(project, "vendored")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	manifest := `targets:
  vendored: []
`
	if err := os.WriteFile(filepath.Join(project, "klone.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("KLONE_CACHE_DIR", filepath.Join(sb, "cache"))
	err := SyncFolder(t.Context(), project, false)
	if err == nil {
		t.Fatalf("SyncFolder returned nil, want symlink-refusal error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("SyncFolder error = %q, want substring 'symlink'", err.Error())
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("VC-53818 empty-srcs escape: sentinel was removed: %v", statErr)
	}
}

// Target paths need component-wise symlink checks too.
func TestSyncFolder_NestedTargetSymlinkRejected(t *testing.T) {
	skipIfNoSymlinks(t)

	sb := t.TempDir()
	outside := filepath.Join(sb, "outside")
	if err := os.MkdirAll(filepath.Join(outside, "bar"), 0o755); err != nil {
		t.Fatalf("mkdir outside/bar: %v", err)
	}
	sentinel := filepath.Join(outside, "bar", "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("VICTIM"), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}

	project := filepath.Join(sb, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(project, "foo")); err != nil {
		t.Fatalf("plant symlink: %v", err)
	}

	manifest := `targets:
  foo/bar: []
`
	if err := os.WriteFile(filepath.Join(project, "klone.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("KLONE_CACHE_DIR", filepath.Join(sb, "cache"))
	err := SyncFolder(t.Context(), project, false)
	if err == nil {
		t.Fatalf("SyncFolder returned nil, want symlink-refusal error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("SyncFolder error = %q, want substring 'symlink'", err.Error())
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("VC-53818 nested-target escape: sentinel was removed: %v", statErr)
	}
}

// The top-level target directory itself must not be a symlink.
func TestSyncFolder_TargetSymlinkRejected(t *testing.T) {
	skipIfNoSymlinks(t)

	sb := t.TempDir()
	outside := filepath.Join(sb, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	sentinel := filepath.Join(outside, "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("VICTIM"), 0o644); err != nil {
		t.Fatalf("plant sentinel: %v", err)
	}

	project := filepath.Join(sb, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(project, "vendored")); err != nil {
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
	if err := os.WriteFile(filepath.Join(project, "klone.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Setenv("KLONE_CACHE_DIR", filepath.Join(sb, "cache"))
	err := SyncFolder(t.Context(), project, false)
	if err == nil {
		t.Fatalf("SyncFolder returned nil, want symlink-refusal error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("SyncFolder error = %q, want substring 'symlink'", err.Error())
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Errorf("VC-53818 root-symlink escape: sentinel was removed: %v", statErr)
	}
}

func mustTouch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", p, err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("touch %s: %v", p, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", p, err)
	}
}
