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

package mod

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rogpeppe/go-internal/lockedfile"
	"gopkg.in/yaml.v3"
)

const kloneFileName = "klone.yaml"

type WorkDir string

type kloneFile struct {
	Targets map[string]KloneFolder `yaml:"targets"`
}

func (f *kloneFile) canonicalize() {
	newModTargets := make(map[string]KloneFolder, len(f.Targets))
	for target, srcs := range f.Targets {
		// Deduplicate sources based on cleaned relative path
		uniqueSrcs := make(map[string]KloneItem, len(srcs))
		for _, src := range srcs {
			src.FolderName = cleanRelativePath(src.FolderName)
			uniqueSrcs[src.FolderName] = src
		}

		// Rebuild array of sources, now without duplicates
		srcs := make(KloneFolder, 0, len(uniqueSrcs))
		for _, src := range uniqueSrcs {
			srcs = append(srcs, src)
		}

		// Sort sources by folder name
		slices.SortFunc(srcs, func(a, b KloneItem) int {
			return a.Compare(b)
		})

		newModTargets[cleanRelativePath(target)] = srcs
	}

	f.Targets = newModTargets
}

type KloneFolder []KloneItem

type KloneItem struct {
	FolderName  string `yaml:"folder_name"`
	KloneSource `yaml:",inline"`
}

func (i KloneItem) Compare(other KloneItem) int {
	return strings.Compare(i.FolderName, other.FolderName)
}

type KloneSource struct {
	RepoURL  string `yaml:"repo_url"`
	RepoRef  string `yaml:"repo_ref"`
	RepoHash string `yaml:"repo_hash"`
	RepoPath string `yaml:"repo_path"`
}

func (w WorkDir) editKloneFile(fn func(*kloneFile) error) error {
	kloneFilePath := filepath.Join(string(w), kloneFileName)

	// exclusively open or create index file
	file, err := lockedfile.Edit(kloneFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	index := kloneFile{}

	// decode current contents of index file
	if err := yaml.NewDecoder(file).Decode(&index); err != nil && err != io.EOF {
		return err
	}

	// canonicalize index
	index.canonicalize()

	// update index
	if err := fn(&index); err != nil {
		return err
	}

	// canonicalize index
	index.canonicalize()

	var topComments string

	{
		// go back to the beginning of the file
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}

		comments := strings.Builder{}

		// read lines until the first non-comment line
		reader := bufio.NewReader(file)
		for {
			line, isPrefix, err := reader.ReadLine()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			if !isPrefix && (len(line) > 0 && line[0] != '#') {
				break
			}

			if _, err := comments.Write(line); err != nil {
				return err
			}

			if !isPrefix {
				if _, err := comments.WriteRune('\n'); err != nil {
					return err
				}
			}
		}

		topComments = comments.String()
	}

	// truncate file
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}

	// write comments
	if _, err := file.WriteString(topComments); err != nil {
		return err
	}

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	if err := encoder.Encode(index); err != nil {
		return err
	}

	return nil
}

func (w WorkDir) Init() error {
	return w.editKloneFile(func(kf *kloneFile) error {
		return nil
	})
}

func (w WorkDir) AddTarget(target string, folderName string, dep KloneSource) error {
	return w.editKloneFile(func(kf *kloneFile) error {
		for targetFolder, src := range kf.Targets[target] {
			if src.FolderName == folderName {
				src.KloneSource = dep
				kf.Targets[target][targetFolder] = src
				return nil
			}
		}

		kf.Targets[target] = append(kf.Targets[target], KloneItem{
			FolderName:  folderName,
			KloneSource: dep,
		})

		return nil
	})
}

func cleanRelativePath(src string) string {
	return filepath.Join(".", filepath.Clean(filepath.Join("/", src)))
}

// ValidateRepoURL rejects repo_url values that could be re-interpreted by git
// as a command-line option or as a request to spin up an arbitrary helper
// transport (e.g. ext::sh -c …). It is intentionally strict — only schemes
// klone actually needs to clone from are accepted. See VC-53817.
func ValidateRepoURL(repoURL string) error {
	if repoURL == "" {
		return fmt.Errorf("repo_url is empty")
	}
	// A leading '-' would be parsed by git as an option even with `--` in
	// the argv (some git subcommands look at the first non-option as
	// `--upload-pack=…` etc. when other flags appear earlier). Reject
	// outright — no legitimate URL starts with '-'.
	if strings.HasPrefix(repoURL, "-") {
		return fmt.Errorf("repo_url %q starts with '-', refusing to pass it to git", repoURL)
	}
	// Reject any helper transport other than the well-known network/local
	// schemes. `ext::`, `transport_helper::`, etc. all match
	// `<word>::<command>` — disallow the `::` shape entirely.
	if strings.Contains(repoURL, "::") {
		return fmt.Errorf("repo_url %q uses a helper transport ('::'), which is not allowed", repoURL)
	}
	// Allow-list the schemes we expect. We accept:
	//   - https://, http://, ssh://, git://, file://     (URL form)
	//   - user@host:path                                 (scp-like ssh form)
	//   - /absolute/local/path                           (local clone path)
	switch {
	case strings.HasPrefix(repoURL, "https://"),
		strings.HasPrefix(repoURL, "http://"),
		strings.HasPrefix(repoURL, "ssh://"),
		strings.HasPrefix(repoURL, "git://"),
		strings.HasPrefix(repoURL, "file://"),
		strings.HasPrefix(repoURL, "/"):
		return nil
	}
	// scp-like form: <user>@<host>:<path>. Require an '@' before the first ':'.
	if at := strings.Index(repoURL, "@"); at > 0 {
		if colon := strings.Index(repoURL, ":"); colon > at {
			return nil
		}
	}
	return fmt.Errorf("repo_url %q does not use an allowed scheme (https/http/ssh/git/file/local path/scp-like)", repoURL)
}

func (w WorkDir) FetchTargets(
	cleanFn func(string, string, *KloneSource) error,
	fetchFn func(target string, srcs KloneFolder) error,
) error {
	return w.editKloneFile(func(kf *kloneFile) error {
		for target, srcs := range kf.Targets {
			for i, src := range srcs {
				if err := cleanFn(target, src.FolderName, &src.KloneSource); err != nil {
					return err
				}
				srcs[i] = src
			}

			if err := fetchFn(target, srcs); err != nil {
				return err
			}

			kf.Targets[target] = srcs
		}

		return nil
	})
}
