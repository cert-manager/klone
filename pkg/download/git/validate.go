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
	"fmt"
	"strings"
)

// validateRepoURL rejects repo_url values that could be re-interpreted by git
// as a command-line option or as a helper transport (e.g. ext::sh -c …).
func validateRepoURL(repoURL string) error {
	if repoURL == "" {
		return fmt.Errorf("repo_url is empty")
	}
	if strings.HasPrefix(repoURL, "-") {
		return fmt.Errorf("repo_url %q starts with '-', refusing to pass it to git", repoURL)
	}
	if strings.Contains(repoURL, "::") {
		return fmt.Errorf("repo_url %q uses a helper transport ('::'), which is not allowed", repoURL)
	}
	switch {
	case strings.HasPrefix(repoURL, "https://"),
		strings.HasPrefix(repoURL, "http://"),
		strings.HasPrefix(repoURL, "ssh://"),
		strings.HasPrefix(repoURL, "git://"),
		strings.HasPrefix(repoURL, "file://"),
		strings.HasPrefix(repoURL, "/"):
		return nil
	}
	if at := strings.Index(repoURL, "@"); at > 0 {
		if colon := strings.Index(repoURL, ":"); colon > at {
			return nil
		}
	}
	return fmt.Errorf("repo_url %q does not use an allowed scheme (https/http/ssh/git/file/local path/scp-like)", repoURL)
}
