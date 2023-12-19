package mod

import (
	"sort"
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
	sort.Slice(items, func(i, j int) bool {
		return items[i].Less(items[j])
	})

	// Verify the sorting order
	expectedOrder := []string{"Folder A", "Folder B", "Folder C"}
	for i, item := range items {
		if item.FolderName != expectedOrder[i] {
			t.Errorf("Expected item at index %d to have folder name %s, but got %s", i, expectedOrder[i], item.FolderName)
		}
	}
}
