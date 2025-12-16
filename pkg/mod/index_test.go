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
	"os"
	"path"
	"slices"
	"testing"
)

func TestKloneItemSorting(t *testing.T) {
	// Create some sample KloneItems
	items := []KloneItem{
		{FolderName: "Folder C", KloneSource: KloneSource{}},
		{FolderName: "Folder A", KloneSource: KloneSource{}},
		{FolderName: "Folder B", KloneSource: KloneSource{}},
	}

	// Sort the items
	slices.SortFunc(items, func(a, b KloneItem) int {
		return a.Compare(b)
	})

	// Verify the sorting order
	expectedOrder := []string{"Folder A", "Folder B", "Folder C"}
	for i, item := range items {
		if item.FolderName != expectedOrder[i] {
			t.Errorf("Expected item at index %d to have folder name %s, but got %s", i, expectedOrder[i], item.FolderName)
		}
	}
}

func Test_editKloneFile(t *testing.T) {
	tests := []struct {
		name      string
		initial   string
		modifyFn  func(*kloneFile) error
		expected  string
		expectErr bool
	}{
		{
			name: "Preserve comments",
			initial: `# Test comment1
# Test comment2
targets:
  target1:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
`,
			modifyFn: func(kf *kloneFile) error {
				return nil
			},
			expected: `# Test comment1
# Test comment2
targets:
  target1:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
`,
			expectErr: false,
		},
		{
			name: "Sort targets (level 1)",
			initial: `# Test comment1
# Test comment2
targets:
  target2:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
  target1:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
`,
			modifyFn: func(kf *kloneFile) error {
				return nil
			},
			expected: `# Test comment1
# Test comment2
targets:
  target1:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
  target2:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
`,
			expectErr: false,
		},
		{
			name: "Sort targets (level 2)",
			initial: `# Test comment1
# Test comment2
targets:
  target1:
    - folder_name: Folder B
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
`,
			modifyFn: func(kf *kloneFile) error {
				return nil
			},
			expected: `# Test comment1
# Test comment2
targets:
  target1:
    - folder_name: Folder A
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
    - folder_name: Folder B
      repo_url: https://github.com/repo1
      repo_ref: main
      repo_hash: abc123
      repo_path: path/to/repo1
`,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDirPath := t.TempDir()

			// Create the file and write the initial contents
			{
				tempFile, err := os.Create(path.Join(tempDirPath, kloneFileName))
				if err != nil {
					t.Fatalf("Failed to create temporary file: %v", err)
				}

				// Write the initial content to the temporary file
				_, err = tempFile.WriteString(tt.initial)
				if err != nil {
					t.Fatalf("Failed to write initial content to temporary file: %v", err)
				}

				// Close the temporary file
				err = tempFile.Close()
				if err != nil {
					t.Fatalf("Failed to close temporary file: %v", err)
				}
			}

			// Create a WorkDir instance with the path to the temporary file
			workDir := WorkDir(tempDirPath)

			// Call the editKloneFile function with the modifyFn
			err := workDir.editKloneFile(tt.modifyFn)

			// Check if an error is expected
			if tt.expectErr && err == nil {
				t.Errorf("Expected an error, but got nil")
			} else if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			// Read the new file contents and compare with the expected contents
			{
				// Read the content of the modified file
				modifiedContent, err := os.ReadFile(path.Join(tempDirPath, kloneFileName))
				if err != nil {
					t.Fatalf("Failed to read modified content: %v", err)
				}

				// Compare the modified content with the expected content
				if string(modifiedContent) != tt.expected {
					t.Errorf("Expected modified content:\n%s\n\nBut got:\n%s", tt.expected, modifiedContent)
				}
			}
		})
	}
}
