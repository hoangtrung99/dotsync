package components

import (
	"testing"

	"dotsync/internal/models"
)

func TestNewFileList(t *testing.T) {
	list := NewFileList()

	if list == nil {
		t.Fatal("NewFileList should return a FileList")
	}
	if len(list.Files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(list.Files))
	}
	if list.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", list.Cursor)
	}
	if list.Focused {
		t.Error("Expected Focused to be false")
	}
	if list.Title == "" {
		t.Error("Expected Title to be set")
	}
}

func TestFileList_SetFiles(t *testing.T) {
	list := NewFileList()
	files := []models.File{
		{Name: "file1.txt"},
		{Name: "file2.txt"},
	}

	list.SetFiles(files, "TestApp")

	if len(list.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(list.Files))
	}
	if list.AppName != "TestApp" {
		t.Errorf("Expected AppName 'TestApp', got %s", list.AppName)
	}
	if list.Cursor != 0 {
		t.Errorf("Expected cursor reset to 0, got %d", list.Cursor)
	}
}

func TestFileList_Clear(t *testing.T) {
	list := NewFileList()
	list.SetFiles([]models.File{{Name: "file.txt"}}, "App")
	list.Cursor = 5

	list.Clear()

	if len(list.Files) != 0 {
		t.Errorf("Expected 0 files after clear, got %d", len(list.Files))
	}
	if list.AppName != "" {
		t.Errorf("Expected empty AppName, got %s", list.AppName)
	}
	if list.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", list.Cursor)
	}
}

func TestFileList_MoveUp(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt"},
		{Name: "file2.txt"},
		{Name: "file3.txt"},
	}
	list.Cursor = 2

	list.MoveUp()
	if list.Cursor != 1 {
		t.Errorf("Expected cursor at 1, got %d", list.Cursor)
	}

	list.MoveUp()
	if list.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", list.Cursor)
	}

	// Should not go below 0
	list.MoveUp()
	if list.Cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", list.Cursor)
	}
}

func TestFileList_MoveDown(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt"},
		{Name: "file2.txt"},
		{Name: "file3.txt"},
	}

	list.MoveDown()
	if list.Cursor != 1 {
		t.Errorf("Expected cursor at 1, got %d", list.Cursor)
	}

	list.MoveDown()
	if list.Cursor != 2 {
		t.Errorf("Expected cursor at 2, got %d", list.Cursor)
	}

	// Should not go beyond last item
	list.MoveDown()
	if list.Cursor != 2 {
		t.Errorf("Expected cursor to stay at 2, got %d", list.Cursor)
	}
}

func TestFileList_Toggle(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: false},
		{Name: "file2.txt", Selected: true},
	}

	list.Toggle()
	if !list.Files[0].Selected {
		t.Error("First file should be selected after toggle")
	}

	list.Cursor = 1
	list.Toggle()
	if list.Files[1].Selected {
		t.Error("Second file should be deselected after toggle")
	}
}

func TestFileList_SelectAll(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: false},
		{Name: "file2.txt", Selected: false},
		{Name: "file3.txt", Selected: true},
	}

	list.SelectAll()

	for i, file := range list.Files {
		if !file.Selected {
			t.Errorf("File %d should be selected", i)
		}
	}
}

func TestFileList_DeselectAll(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: true},
		{Name: "file2.txt", Selected: true},
		{Name: "file3.txt", Selected: false},
	}

	list.DeselectAll()

	for i, file := range list.Files {
		if file.Selected {
			t.Errorf("File %d should not be selected", i)
		}
	}
}

func TestFileList_Current(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt"},
		{Name: "file2.txt"},
	}

	current := list.Current()
	if current == nil {
		t.Fatal("Current should return a file")
	}
	if current.Name != "file1.txt" {
		t.Errorf("Expected file1.txt, got %s", current.Name)
	}

	list.Cursor = 1
	current = list.Current()
	if current.Name != "file2.txt" {
		t.Errorf("Expected file2.txt, got %s", current.Name)
	}
}

func TestFileList_Current_Empty(t *testing.T) {
	list := NewFileList()

	current := list.Current()
	if current != nil {
		t.Error("Current should return nil for empty list")
	}
}

func TestFileList_SelectedFiles(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: true},
		{Name: "file2.txt", Selected: false},
		{Name: "file3.txt", Selected: true},
	}

	selected := list.SelectedFiles()
	if len(selected) != 2 {
		t.Errorf("Expected 2 selected, got %d", len(selected))
	}
}

func TestFileList_View(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: true},
		{Name: "file2.txt", Selected: false},
	}
	list.Width = 40
	list.Height = 10
	list.AppName = "TestApp"

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string")
	}
}

func TestFileList_View_Empty(t *testing.T) {
	list := NewFileList()
	list.Width = 40
	list.Height = 10

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string even for empty list")
	}
}

func TestFileList_View_WithScrolling(t *testing.T) {
	list := NewFileList()
	files := make([]models.File, 20)
	for i := 0; i < 20; i++ {
		files[i] = models.File{Name: "file.txt", Selected: true}
	}
	list.Files = files
	list.Width = 40
	list.Height = 5
	list.Cursor = 15 // Trigger scrolling

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string with scrolling")
	}
}

func TestFileList_View_WithSyncStatus(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: true, SyncStatus: models.StatusSynced},
		{Name: "file2.txt", Selected: false, SyncStatus: models.StatusModified},
		{Name: "file3.txt", Selected: true, SyncStatus: models.StatusOutdated},
		{Name: "file4.txt", Selected: false, SyncStatus: models.StatusNew},
	}
	list.Width = 50
	list.Height = 10
	list.AppName = "TestApp"
	list.Focused = true

	view := list.View()
	if view == "" {
		t.Error("View should render files with sync status")
	}
}

func TestFileList_View_Unfocused(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: true},
	}
	list.Width = 40
	list.Height = 10
	list.Focused = false

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string when unfocused")
	}
}

func TestFileList_View_LongFileName(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "this_is_a_very_long_file_name_that_should_be_truncated.txt", Selected: true},
	}
	list.Width = 30 // Small width to trigger truncation
	list.Height = 10

	view := list.View()
	if view == "" {
		t.Error("View should handle long file names")
	}
}

func TestFileList_View_AllSyncStatuses(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "synced.txt", SyncStatus: models.StatusSynced},
		{Name: "modified.txt", SyncStatus: models.StatusModified},
		{Name: "outdated.txt", SyncStatus: models.StatusOutdated},
		{Name: "new.txt", SyncStatus: models.StatusNew},
		{Name: "missing.txt", SyncStatus: models.StatusMissing},
		{Name: "unknown.txt", SyncStatus: models.StatusUnknown},
	}
	list.Width = 50
	list.Height = 15
	list.AppName = "TestApp"
	list.Focused = true

	view := list.View()
	if view == "" {
		t.Error("View should render all sync statuses")
	}
}

func TestFileList_View_CursorAtDifferentPositions(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file1.txt", Selected: true},
		{Name: "file2.txt", Selected: false},
		{Name: "file3.txt", Selected: true},
	}
	list.Width = 40
	list.Height = 10
	list.Focused = true

	// Test cursor at first position
	list.Cursor = 0
	view1 := list.View()
	if view1 == "" {
		t.Error("View should render with cursor at first position")
	}

	// Test cursor at middle
	list.Cursor = 1
	view2 := list.View()
	if view2 == "" {
		t.Error("View should render with cursor at middle")
	}

	// Test cursor at last position
	list.Cursor = 2
	view3 := list.View()
	if view3 == "" {
		t.Error("View should render with cursor at last position")
	}
}

func TestFileList_View_EncryptedFiles(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "secret.txt", Encrypted: true, Selected: true},
		{Name: "normal.txt", Encrypted: false, Selected: false},
	}
	list.Width = 40
	list.Height = 10

	view := list.View()
	if view == "" {
		t.Error("View should render encrypted files")
	}
}

func TestFileList_View_NormalWidth(t *testing.T) {
	list := NewFileList()
	list.Files = []models.File{
		{Name: "file.txt"},
	}
	list.Width = 40 // Normal width
	list.Height = 10

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty")
	}
}

func TestFileList_Toggle_EmptyList(t *testing.T) {
	list := NewFileList()
	// Should not panic on empty list
	list.Toggle()
}

func TestFileList_MoveUp_EmptyList(t *testing.T) {
	list := NewFileList()
	// Should not panic on empty list
	list.MoveUp()
	if list.Cursor != 0 {
		t.Errorf("Cursor should stay at 0")
	}
}

func TestFileList_MoveDown_EmptyList(t *testing.T) {
	list := NewFileList()
	// Should not panic on empty list
	list.MoveDown()
	if list.Cursor != 0 {
		t.Errorf("Cursor should stay at 0")
	}
}
