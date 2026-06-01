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

package cache

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

func TestAssertNoSymlinkInSubpath(t *testing.T) {
	skipIfNoSymlinks(t)
	root := t.TempDir()

	// Layout:
	//   root/a               (regular dir)
	//   root/a/b             (regular dir)
	//   root/sym -> /tmp     (symlink)
	//   root/a/blink -> /tmp (symlink as final component)
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	linkTarget := filepath.Join(root, "a", "b")
	if err := os.Symlink(linkTarget, filepath.Join(root, "sym")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := os.Symlink(linkTarget, filepath.Join(root, "a", "blink")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tests := []struct {
		name     string
		subpath  string
		wantErr  bool
		errMatch string
	}{
		{name: "empty subpath", subpath: "", wantErr: false},
		{name: "dot subpath", subpath: ".", wantErr: false},
		{name: "all regular dirs", subpath: "a/b", wantErr: false},
		{name: "non-existent leaf", subpath: "a/b/new", wantErr: false},
		{name: "non-existent intermediate", subpath: "fresh/leaf", wantErr: false},

		// VC-53816 cases.
		{name: "symlink at root", subpath: "sym", wantErr: true, errMatch: "symlink"},
		{name: "symlink as intermediate", subpath: "sym/anything", wantErr: true, errMatch: "symlink"},
		{name: "symlink as final", subpath: "a/blink", wantErr: true, errMatch: "symlink"},

		// path.Clean canonicalises inner traversal, so these resolve to a
		// safe path under root and must be accepted.
		{name: "inner traversal canonicalises", subpath: "a/../etc", wantErr: false},
		{name: "inner traversal after non-existent", subpath: "fresh/../etc", wantErr: false},
		{name: "empty segment canonicalises", subpath: "fresh//etc", wantErr: false},
		{name: "backslash inner traversal", subpath: `a\..\etc`, wantErr: false},

		// Net upward escapes must be rejected — Clean preserves a leading "..".
		{name: "lone dotdot", subpath: "..", wantErr: true, errMatch: "escapes root"},
		{name: "leading dotdot", subpath: "../etc", wantErr: true, errMatch: "escapes root"},
		{name: "net escape via inner dotdot", subpath: "a/../../etc", wantErr: true, errMatch: "escapes root"},
		{name: "backslash net escape", subpath: `a\..\..\etc`, wantErr: true, errMatch: "escapes root"},

		{name: "absolute unix path", subpath: "/etc/passwd", wantErr: true, errMatch: "relative"},
		// UNC normalises to "//server/share/x", which IsAbs catches as
		// absolute on POSIX and VolumeName catches on Windows.
		{name: "unc path", subpath: `\\server\share\x`, wantErr: true, errMatch: "invalid subpath"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AssertNoSymlinkInSubpath(root, tt.subpath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("AssertNoSymlinkInSubpath(root, %q) = nil, want error", tt.subpath)
					return
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}
			if err != nil {
				t.Errorf("AssertNoSymlinkInSubpath(root, %q) returned unexpected error: %v", tt.subpath, err)
			}
		})
	}
}
