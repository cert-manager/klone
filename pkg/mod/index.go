package mod

import (
	"encoding/json"
	"io"
	"path/filepath"
	"sort"

	"github.com/rogpeppe/go-internal/lockedfile"
)

const kloneFileName = "klone.json"

type WorkDir string

type kloneFile struct {
	Targets map[string]KloneFolder `json:"targets"`
}

type KloneFolder []KloneItem

type KloneItem struct {
	FolderName  string `json:"folder_name"`
	KloneSource `json:",inline"`
}

type KloneSource struct {
	RepoURL  string `json:"repo_url"`
	RepoRef  string `json:"repo_ref"`
	RepoHash string `json:"repo_hash"`
	RepoPath string `json:"repo_path"`
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
	if err := json.NewDecoder(file).Decode(&index); err != nil && err != io.EOF {
		return err
	}

	// update index
	if err := fn(&index); err != nil {
		return err
	}

	file.Seek(0, 0)
	file.Truncate(0)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(index); err != nil {
		return err
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
	if err := json.NewDecoder(file).Decode(&index); err != nil && err != io.EOF {
		return nil, err
	}

	return &index, nil
}

func (w WorkDir) Init() error {
	return w.editKloneFile(func(kf *kloneFile) error {
		if kf.Targets == nil {
			kf.Targets = make(map[string]KloneFolder)
		}
		return nil
	})
}

func (w WorkDir) RemoveTarget(target string, dep KloneItem) error {
	return w.editKloneFile(func(kf *kloneFile) error {
		if kf.Targets == nil {
			return nil
		}

		delete(kf.Targets, target)
		return nil
	})
}

func (w WorkDir) AddTarget(target string, folderName string, dep KloneSource) error {
	return w.editKloneFile(func(kf *kloneFile) error {
		if kf.Targets == nil {
			kf.Targets = make(map[string]KloneFolder)
		}

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
		// clean target paths (only keep the last entry if they are identical)
		newModTargets := make(map[string]KloneFolder, len(kf.Targets))
		for target, srcs := range kf.Targets {
			cleanedTarget := cleanRelativePath(target)

			newSrcs := make(map[string]KloneItem, len(srcs))
			for _, src := range srcs {
				src.FolderName = cleanRelativePath(src.FolderName)

				newSrcs[src.FolderName] = src
			}

			newSrcsArray := make(KloneFolder, 0, len(newSrcs))
			for _, src := range newSrcs {
				newSrcsArray = append(newSrcsArray, src)
			}

			sort.Slice(newSrcsArray, func(i, j int) bool {
				return newSrcsArray[i].FolderName < newSrcsArray[j].FolderName ||
					newSrcsArray[i].RepoURL < newSrcsArray[j].RepoURL ||
					newSrcsArray[i].RepoRef < newSrcsArray[j].RepoRef ||
					newSrcsArray[i].RepoHash < newSrcsArray[j].RepoHash ||
					newSrcsArray[i].RepoPath < newSrcsArray[j].RepoPath
			})

			for i, src := range newSrcsArray {
				if err := cleanFn(target, src.FolderName, &src.KloneSource); err != nil {
					return err
				}

				newSrcsArray[i] = src
			}

			if err := fetchFn(target, newSrcsArray); err != nil {
				return err
			}

			newModTargets[cleanedTarget] = newSrcsArray
		}

		kf.Targets = newModTargets

		return nil
	})
}
