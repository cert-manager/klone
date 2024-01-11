package mod

import (
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

	var rawDocument yaml.Node

	// decode current contents of index file
	if err := yaml.NewDecoder(file).Decode(&rawDocument); err != nil && err != io.EOF {
		return err
	}

	index := kloneFile{}

	err = rawDocument.Decode(&index)
	if err != nil {
		return err
	}

	index.canonicalize()

	// update index
	if err := fn(&index); err != nil {
		return err
	}

	index.canonicalize()

	// truncate file
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}

	trimmedHeadComment := strings.TrimSpace(rawDocument.HeadComment)
	if len(trimmedHeadComment) > 0 {
		_, _ = file.WriteString(trimmedHeadComment + "\n\n")
	}

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	if err := encoder.Encode(index); err != nil {
		return err
	}

	trimmedFootComment := strings.TrimSpace(rawDocument.FootComment)
	if len(trimmedFootComment) > 0 {
		_, _ = file.WriteString("\n\n" + trimmedFootComment)
	}

	return nil
}

func (w WorkDir) readKloneFile() (*kloneFile, error) {
	kloneFilePath := filepath.Join(string(w), kloneFileName)

	// exclusively open or create index file
	file, err := lockedfile.Open(kloneFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	index := kloneFile{}

	// decode current contents of index file
	if err := yaml.NewDecoder(file).Decode(&index); err != nil && err != io.EOF {
		return nil, err
	}

	// canonicalize index
	index.canonicalize()

	return &index, nil
}

func (w WorkDir) Init() error {
	return w.editKloneFile(func(kf *kloneFile) error {
		return nil
	})
}

func (w WorkDir) AddTarget(target string, folderName string, dep KloneSource) error {
	return w.editKloneFile(func(kf *kloneFile) error {
		for _, src := range kf.Targets[target] {
			if src.FolderName == folderName {
				src.KloneSource = dep
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
