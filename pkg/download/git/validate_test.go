/*
Copyright 2023 The cert-manager Authors.

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

package git

import (
	"strings"
	"testing"
)

func TestValidateRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		errMatch string // substring expected in the error, optional
	}{
		// Legitimate.
		{name: "https", input: "https://github.com/cert-manager/klone.git", wantErr: false},
		{name: "http", input: "http://example.com/repo.git", wantErr: false},
		{name: "ssh url", input: "ssh://git@github.com/cert-manager/klone.git", wantErr: false},
		{name: "git url", input: "git://example.com/repo.git", wantErr: false},
		{name: "file url", input: "file:///srv/git/repo.git", wantErr: false},
		{name: "absolute local path", input: "/srv/git/repo.git", wantErr: false},
		{name: "scp-like", input: "git@github.com:cert-manager/klone.git", wantErr: false},

		// VC-53817 vectors.
		{name: "ext transport", input: "ext::sh -c touch$IFS/tmp/klone-pwned", wantErr: true, errMatch: "helper transport"},
		{name: "upload-pack option", input: "--upload-pack=touch /tmp/klone-pwned2;true", wantErr: true, errMatch: "starts with '-'"},
		{name: "single dash option", input: "-uX", wantErr: true, errMatch: "starts with '-'"},
		{name: "transport_helper", input: "transport_helper::cmd", wantErr: true, errMatch: "helper transport"},

		// Degenerate / unsupported.
		{name: "empty", input: "", wantErr: true},
		{name: "relative path", input: "../etc", wantErr: true},
		{name: "bare hostname", input: "example.com", wantErr: true},
		{name: "scheme without slashes", input: "javascript:alert(1)", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateRepoURL(%q) = nil, want error", tt.input)
					return
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("validateRepoURL(%q) error = %q, want substring %q", tt.input, err.Error(), tt.errMatch)
				}
				return
			}
			if err != nil {
				t.Errorf("validateRepoURL(%q) returned unexpected error: %v", tt.input, err)
			}
		})
	}
}
