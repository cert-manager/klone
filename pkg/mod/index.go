package mod

import (
	"bufio"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rogpeppe/go-internal/lockedfile"
	"gopkg.in/yaml.v3"
)

const kloneFileName = "klone.yaml"

type WorkDir string

type kloneFile struct {
	Targets map[string]KloneFolder `yaml:"targets"`
}

type KloneFolder []KloneItem

type KloneItem struct {
	FolderName  string `yaml:"folder_name"`
	KloneSource `yaml:",inline"`
}

func (i KloneItem) Less(other KloneItem) bool {
	return i.FolderName < other.FolderName
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

	// update index
	if err := fn(&index); err != nil {
		return err
	}

	topComments := ""

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
				return newSrcsArray[i].Less(newSrcsArray[j])
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
