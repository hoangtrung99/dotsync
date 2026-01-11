package components

import (
	"testing"

	"dotsync/internal/models"
)

func TestNewAppList(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Name: "App 1"},
		{ID: "app2", Name: "App 2"},
	}

	list := NewAppList(apps)

	if list == nil {
		t.Fatal("NewAppList should return an AppList")
	}
	if len(list.Apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(list.Apps))
	}
	if list.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", list.Cursor)
	}
	if !list.Focused {
		t.Error("Expected Focused to be true")
	}
	if list.Title == "" {
		t.Error("Expected Title to be set")
	}
}

func TestAppList_SetApps(t *testing.T) {
	list := NewAppList([]*models.App{})
	list.Cursor = 5 // Set cursor beyond new list

	newApps := []*models.App{
		{ID: "app1", Name: "App 1"},
		{ID: "app2", Name: "App 2"},
	}
	list.SetApps(newApps)

	if len(list.Apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(list.Apps))
	}
	if list.Cursor >= len(newApps) {
		t.Errorf("Cursor should be adjusted to valid range")
	}
}

func TestAppList_MoveUp(t *testing.T) {
	apps := []*models.App{
		{ID: "app1"}, {ID: "app2"}, {ID: "app3"},
	}
	list := NewAppList(apps)
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

func TestAppList_MoveDown(t *testing.T) {
	apps := []*models.App{
		{ID: "app1"}, {ID: "app2"}, {ID: "app3"},
	}
	list := NewAppList(apps)

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

func TestAppList_Toggle(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Selected: false},
		{ID: "app2", Selected: true},
	}
	list := NewAppList(apps)

	list.Toggle()
	if !apps[0].Selected {
		t.Error("First app should be selected after toggle")
	}

	list.Cursor = 1
	list.Toggle()
	if apps[1].Selected {
		t.Error("Second app should be deselected after toggle")
	}
}

func TestAppList_SelectAll(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Selected: false},
		{ID: "app2", Selected: false},
		{ID: "app3", Selected: true},
	}
	list := NewAppList(apps)

	list.SelectAll()

	for i, app := range apps {
		if !app.Selected {
			t.Errorf("App %d should be selected", i)
		}
	}
}

func TestAppList_DeselectAll(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Selected: true},
		{ID: "app2", Selected: true},
		{ID: "app3", Selected: false},
	}
	list := NewAppList(apps)

	list.DeselectAll()

	for i, app := range apps {
		if app.Selected {
			t.Errorf("App %d should not be selected", i)
		}
	}
}

func TestAppList_Current(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Name: "App 1"},
		{ID: "app2", Name: "App 2"},
	}
	list := NewAppList(apps)

	current := list.Current()
	if current == nil {
		t.Fatal("Current should return an app")
	}
	if current.ID != "app1" {
		t.Errorf("Expected app1, got %s", current.ID)
	}

	list.Cursor = 1
	current = list.Current()
	if current.ID != "app2" {
		t.Errorf("Expected app2, got %s", current.ID)
	}
}

func TestAppList_Current_Empty(t *testing.T) {
	list := NewAppList([]*models.App{})

	current := list.Current()
	if current != nil {
		t.Error("Current should return nil for empty list")
	}
}

func TestAppList_SelectedApps(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Selected: true},
		{ID: "app2", Selected: false},
		{ID: "app3", Selected: true},
	}
	list := NewAppList(apps)

	selected := list.SelectedApps()
	if len(selected) != 2 {
		t.Errorf("Expected 2 selected, got %d", len(selected))
	}
}

func TestAppList_View(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Name: "App 1", Icon: "📱", Installed: true},
		{ID: "app2", Name: "App 2", Icon: "📦", Installed: true},
	}
	list := NewAppList(apps)
	list.Width = 30
	list.Height = 10

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string")
	}
}

func TestAppList_View_WithScrolling(t *testing.T) {
	// Create a list with more items than height
	apps := make([]*models.App, 20)
	for i := 0; i < 20; i++ {
		apps[i] = &models.App{ID: "app", Name: "App", Icon: "📱", Installed: true}
	}
	list := NewAppList(apps)
	list.Width = 30
	list.Height = 5
	list.Cursor = 15 // Set cursor near end to trigger scrolling

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string with scrolling")
	}
}

func TestAppList_View_WithCategories(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Name: "App 1", Icon: "📱", Category: "terminal", Installed: true},
		{ID: "app2", Name: "App 2", Icon: "📦", Category: "editor", Installed: true, Selected: true},
	}
	list := NewAppList(apps)
	list.Width = 40
	list.Height = 10
	list.Focused = true

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string with categories")
	}
}

func TestAppList_View_Unfocused(t *testing.T) {
	apps := []*models.App{
		{ID: "app1", Name: "App 1", Icon: "📱", Installed: true},
	}
	list := NewAppList(apps)
	list.Width = 30
	list.Height = 10
	list.Focused = false

	view := list.View()
	if view == "" {
		t.Error("View should return non-empty string when unfocused")
	}
}

func TestAppList_MaxMin(t *testing.T) {
	// Test max/min functions indirectly through View
	apps := []*models.App{
		{ID: "app1", Name: "App 1", Icon: "📱"},
	}
	list := NewAppList(apps)
	list.Width = 50  // Normal width
	list.Height = 10 // Normal height

	view := list.View()
	// Just ensure it returns output
	if view == "" {
		t.Error("View should return non-empty string")
	}
}

func TestAppList_PageUp(t *testing.T) {
	apps := make([]*models.App, 30)
	for i := 0; i < 30; i++ {
		apps[i] = &models.App{ID: "app", Name: "App"}
	}
	list := NewAppList(apps)
	list.Height = 13 // pageSize = 10
	list.Cursor = 20

	list.PageUp()
	if list.Cursor != 10 {
		t.Errorf("Expected cursor at 10, got %d", list.Cursor)
	}

	list.PageUp()
	if list.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", list.Cursor)
	}

	// Should not go below 0
	list.PageUp()
	if list.Cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", list.Cursor)
	}
}

func TestAppList_PageDown(t *testing.T) {
	apps := make([]*models.App, 30)
	for i := 0; i < 30; i++ {
		apps[i] = &models.App{ID: "app", Name: "App"}
	}
	list := NewAppList(apps)
	list.Height = 13 // pageSize = 10
	list.Cursor = 0

	list.PageDown()
	if list.Cursor != 10 {
		t.Errorf("Expected cursor at 10, got %d", list.Cursor)
	}

	list.PageDown()
	if list.Cursor != 20 {
		t.Errorf("Expected cursor at 20, got %d", list.Cursor)
	}

	list.PageDown()
	if list.Cursor != 29 { // Should stop at last item
		t.Errorf("Expected cursor at 29, got %d", list.Cursor)
	}
}

func TestAppList_GoToFirst(t *testing.T) {
	apps := make([]*models.App, 10)
	for i := 0; i < 10; i++ {
		apps[i] = &models.App{ID: "app", Name: "App"}
	}
	list := NewAppList(apps)
	list.Cursor = 7

	list.GoToFirst()
	if list.Cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", list.Cursor)
	}
}

func TestAppList_GoToLast(t *testing.T) {
	apps := make([]*models.App, 10)
	for i := 0; i < 10; i++ {
		apps[i] = &models.App{ID: "app", Name: "App"}
	}
	list := NewAppList(apps)
	list.Cursor = 2

	list.GoToLast()
	if list.Cursor != 9 {
		t.Errorf("Expected cursor at 9, got %d", list.Cursor)
	}
}

func TestAppList_GoToLast_EmptyList(t *testing.T) {
	list := NewAppList([]*models.App{})
	list.GoToLast()
	if list.Cursor != 0 {
		t.Errorf("Expected cursor to stay at 0 for empty list, got %d", list.Cursor)
	}
}
