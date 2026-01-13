package components

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"dotsync/internal/models"
	"dotsync/internal/ui"
)

// TreeNode represents a node in the file tree
type TreeNode struct {
	Name       string
	Path       string
	IsDir      bool
	Expanded   bool
	File       *models.File // nil for directories
	Children   []*TreeNode
	Depth      int
	Parent     *TreeNode
	VisibleIdx int // Index in flattened visible list
}

// FileList is a list component for files with tree view
type FileList struct {
	Files   []models.File
	Cursor  int
	Width   int
	Height  int
	Focused bool
	Title   string
	AppName string

	// Tree structure
	root         *TreeNode
	visibleNodes []*TreeNode // Flattened list of visible nodes
}

// NewFileList creates a new file list
func NewFileList() *FileList {
	return &FileList{
		Files:   []models.File{},
		Cursor:  0,
		Width:   40,
		Height:  15,
		Focused: false,
		Title:   "📄 Files",
	}
}

// SetFiles updates the files list and builds tree
func (l *FileList) SetFiles(files []models.File, appName string) {
	l.Files = files
	l.AppName = appName
	l.Cursor = 0
	l.buildTree()
}

// Clear clears the file list
func (l *FileList) Clear() {
	l.Files = []models.File{}
	l.AppName = ""
	l.Cursor = 0
	l.root = nil
	l.visibleNodes = nil
}

// buildTree constructs the tree from files
func (l *FileList) buildTree() {
	if len(l.Files) == 0 {
		l.root = nil
		l.visibleNodes = nil
		return
	}

	// Create root node
	l.root = &TreeNode{
		Name:     l.AppName,
		IsDir:    true,
		Expanded: true,
		Children: []*TreeNode{},
		Depth:    -1, // Root is hidden
	}

	// Map to track existing nodes by path
	nodeMap := make(map[string]*TreeNode)

	// First pass: add all directories from Files list
	for i := range l.Files {
		file := &l.Files[i]
		if !file.IsDir {
			continue
		}

		relPath := file.RelPath
		if relPath == "" {
			relPath = file.Name
		}

		// Create node for this directory
		node := l.getOrCreateNode(nodeMap, relPath, file)
		node.File = file
		node.IsDir = true
		node.Expanded = true
	}

	// Second pass: add all files
	for i := range l.Files {
		file := &l.Files[i]
		if file.IsDir {
			continue
		}

		relPath := file.RelPath
		if relPath == "" {
			relPath = file.Name
		}

		// Get or create parent directory
		dir := filepath.Dir(relPath)
		var parentNode *TreeNode
		if dir == "." || dir == "" {
			parentNode = l.root
		} else {
			parentNode = l.getOrCreateNode(nodeMap, dir, nil)
		}

		// Create file node
		fileNode := &TreeNode{
			Name:   filepath.Base(relPath),
			Path:   file.Path,
			IsDir:  false,
			File:   file,
			Parent: parentNode,
			Depth:  parentNode.Depth + 1,
		}

		parentNode.Children = append(parentNode.Children, fileNode)
	}

	// Sort children at each level
	l.sortChildren(l.root)

	// Build visible nodes list
	l.rebuildVisibleNodes()
}

// getOrCreateNode gets an existing node or creates directory nodes as needed
func (l *FileList) getOrCreateNode(nodeMap map[string]*TreeNode, path string, file *models.File) *TreeNode {
	if node, exists := nodeMap[path]; exists {
		return node
	}

	// Get parent directory
	parentPath := filepath.Dir(path)
	var parentNode *TreeNode

	if parentPath == "." || parentPath == "" {
		parentNode = l.root
	} else {
		parentNode = l.getOrCreateNode(nodeMap, parentPath, nil)
	}

	// Create new node
	node := &TreeNode{
		Name:     filepath.Base(path),
		Path:     path,
		IsDir:    true,
		Expanded: true,
		Children: []*TreeNode{},
		Parent:   parentNode,
		Depth:    parentNode.Depth + 1,
		File:     file,
	}

	parentNode.Children = append(parentNode.Children, node)
	nodeMap[path] = node

	return node
}

// sortChildren recursively sorts children (directories first, then alphabetically)
func (l *FileList) sortChildren(node *TreeNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		// Directories first
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		// Then alphabetically
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	// Recursively sort children
	for _, child := range node.Children {
		l.sortChildren(child)
	}
}

// rebuildVisibleNodes flattens the tree into visible nodes
func (l *FileList) rebuildVisibleNodes() {
	l.visibleNodes = nil
	if l.root == nil {
		return
	}
	l.flattenNode(l.root)

	// Update visible indices
	for i, node := range l.visibleNodes {
		node.VisibleIdx = i
	}
}

// flattenNode recursively adds visible nodes to the list
func (l *FileList) flattenNode(node *TreeNode) {
	if node == nil {
		return
	}

	// Don't add root node itself
	if node.Depth >= 0 {
		l.visibleNodes = append(l.visibleNodes, node)
	}

	// Add children if expanded
	if node.Expanded {
		for _, child := range node.Children {
			l.flattenNode(child)
		}
	}
}

// MoveUp moves cursor up
func (l *FileList) MoveUp() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

// MoveDown moves cursor down
func (l *FileList) MoveDown() {
	maxIdx := len(l.visibleNodes) - 1
	if len(l.visibleNodes) == 0 {
		maxIdx = len(l.Files) - 1
	}
	if l.Cursor < maxIdx {
		l.Cursor++
	}
}

// PageUp moves cursor up by a page
func (l *FileList) PageUp() {
	pageSize := l.Height - 3
	if pageSize < 1 {
		pageSize = 10
	}
	l.Cursor -= pageSize
	if l.Cursor < 0 {
		l.Cursor = 0
	}
}

// PageDown moves cursor down by a page
func (l *FileList) PageDown() {
	pageSize := l.Height - 3
	if pageSize < 1 {
		pageSize = 10
	}
	maxIdx := len(l.visibleNodes) - 1
	if len(l.visibleNodes) == 0 {
		maxIdx = len(l.Files) - 1
	}
	l.Cursor += pageSize
	if l.Cursor > maxIdx {
		l.Cursor = max(0, maxIdx)
	}
}

// GoToFirst moves cursor to the first item
func (l *FileList) GoToFirst() {
	l.Cursor = 0
}

// GoToLast moves cursor to the last item
func (l *FileList) GoToLast() {
	if len(l.visibleNodes) > 0 {
		l.Cursor = len(l.visibleNodes) - 1
	} else if len(l.Files) > 0 {
		l.Cursor = len(l.Files) - 1
	}
}

// Toggle toggles selection of current file or all files in directory
func (l *FileList) Toggle() {
	if len(l.visibleNodes) > 0 && l.Cursor < len(l.visibleNodes) {
		node := l.visibleNodes[l.Cursor]
		if node.IsDir {
			// Directory - toggle selection of all children
			l.toggleDirSelection(node)
		} else if node.File != nil {
			// File - toggle selection
			node.File.ToggleSelected()
		}
	} else if len(l.Files) > 0 && l.Cursor < len(l.Files) {
		l.Files[l.Cursor].ToggleSelected()
	}
}

// toggleDirSelection toggles selection of all files in a directory
func (l *FileList) toggleDirSelection(node *TreeNode) {
	// Check if all children are selected
	allSelected := l.areAllChildrenSelected(node)

	// Toggle: if all selected, deselect all; otherwise select all
	l.setChildrenSelection(node, !allSelected)
}

// areAllChildrenSelected checks if all files in a directory are selected
func (l *FileList) areAllChildrenSelected(node *TreeNode) bool {
	if node.File != nil && !node.File.IsDir {
		return node.File.Selected
	}

	if len(node.Children) == 0 {
		return false
	}

	for _, child := range node.Children {
		if !l.areAllChildrenSelected(child) {
			return false
		}
	}
	return true
}

// setChildrenSelection sets selection state for all files in a directory
func (l *FileList) setChildrenSelection(node *TreeNode, selected bool) {
	if node.File != nil {
		node.File.Selected = selected
	}

	for _, child := range node.Children {
		l.setChildrenSelection(child, selected)
	}
}

// areSomeChildrenSelected checks if some (but not all) files in a directory are selected
func (l *FileList) areSomeChildrenSelected(node *TreeNode) bool {
	if node.File != nil && !node.File.IsDir {
		return node.File.Selected
	}

	hasSelected := false
	hasUnselected := false

	l.checkChildrenSelection(node, &hasSelected, &hasUnselected)

	return hasSelected && hasUnselected
}

// checkChildrenSelection recursively checks selection state
func (l *FileList) checkChildrenSelection(node *TreeNode, hasSelected, hasUnselected *bool) {
	if node.File != nil && !node.File.IsDir {
		if node.File.Selected {
			*hasSelected = true
		} else {
			*hasUnselected = true
		}
		return
	}

	for _, child := range node.Children {
		l.checkChildrenSelection(child, hasSelected, hasUnselected)
		// Early exit if we found both
		if *hasSelected && *hasUnselected {
			return
		}
	}
}

// ToggleExpand toggles expand/collapse for directory at cursor
func (l *FileList) ToggleExpand() {
	if len(l.visibleNodes) > 0 && l.Cursor < len(l.visibleNodes) {
		node := l.visibleNodes[l.Cursor]
		if node.IsDir {
			node.Expanded = !node.Expanded
			l.rebuildVisibleNodes()
		}
	}
}

// SelectAll selects all files
func (l *FileList) SelectAll() {
	for i := range l.Files {
		l.Files[i].Selected = true
	}
}

// DeselectAll deselects all files
func (l *FileList) DeselectAll() {
	for i := range l.Files {
		l.Files[i].Selected = false
	}
}

// Current returns the currently selected file
func (l *FileList) Current() *models.File {
	if len(l.visibleNodes) > 0 && l.Cursor < len(l.visibleNodes) {
		return l.visibleNodes[l.Cursor].File
	}
	if len(l.Files) > 0 && l.Cursor < len(l.Files) {
		return &l.Files[l.Cursor]
	}
	return nil
}

// CurrentNode returns the current tree node at cursor
func (l *FileList) CurrentNode() *TreeNode {
	if len(l.visibleNodes) > 0 && l.Cursor < len(l.visibleNodes) {
		return l.visibleNodes[l.Cursor]
	}
	return nil
}

// SelectedFiles returns all selected files
func (l *FileList) SelectedFiles() []models.File {
	var selected []models.File
	for _, f := range l.Files {
		if f.Selected {
			selected = append(selected, f)
		}
	}
	return selected
}

// View renders the file list with tree structure
func (l *FileList) View() string {
	var b strings.Builder

	// Title with app name and counts
	selectedCount := 0
	for _, f := range l.Files {
		if f.Selected {
			selectedCount++
		}
	}

	title := l.Title
	if l.AppName != "" {
		if selectedCount > 0 {
			title = fmt.Sprintf("📄 %s (%d/%d)", l.AppName, selectedCount, len(l.Files))
		} else if len(l.Files) > 0 {
			title = fmt.Sprintf("📄 %s (%d)", l.AppName, len(l.Files))
		} else {
			title = fmt.Sprintf("📄 %s", l.AppName)
		}
	}
	b.WriteString(ui.PanelTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(ui.DividerStyle.Render(strings.Repeat("─", l.Width-2)))
	b.WriteString("\n")

	if len(l.Files) == 0 {
		b.WriteString(ui.ItemStyle.Render("Select an app to see files"))
		return l.wrapInPanel(b.String())
	}

	// Use tree view if available
	if len(l.visibleNodes) > 0 {
		return l.renderTreeView(&b)
	}

	// Fallback to flat view
	return l.renderFlatView(&b)
}

// renderTreeView renders the tree structure
func (l *FileList) renderTreeView(b *strings.Builder) string {
	// Calculate visible range
	visibleHeight := l.Height - 3
	startIdx := 0
	if l.Cursor >= visibleHeight {
		startIdx = l.Cursor - visibleHeight + 1
	}
	endIdx := min(startIdx+visibleHeight, len(l.visibleNodes))

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(MutedStyle.Render("  ↑ more"))
		b.WriteString("\n")
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		node := l.visibleNodes[i]
		line := l.renderTreeNode(node, i == l.Cursor)
		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Show scroll indicator at bottom
	if endIdx < len(l.visibleNodes) {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("  ↓ more"))
	}

	// Add position indicator when scrolling
	if len(l.visibleNodes) > visibleHeight {
		position := fmt.Sprintf(" %d/%d ", l.Cursor+1, len(l.visibleNodes))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(strings.Repeat(" ", (l.Width-len(position)-4)/2) + position))
	}

	return l.wrapInPanel(b.String())
}

// renderTreeNode renders a single tree node
func (l *FileList) renderTreeNode(node *TreeNode, isCursor bool) string {
	// Build tree prefix
	indent := strings.Repeat("  ", node.Depth)

	// Tree connector
	connector := "├─"
	if node.Parent != nil && len(node.Parent.Children) > 0 {
		lastChild := node.Parent.Children[len(node.Parent.Children)-1]
		if lastChild == node {
			connector = "└─"
		}
	}

	// Icon and expand indicator
	var icon string
	expandIndicator := ""
	if node.IsDir {
		if node.Expanded {
			icon = "📂"
			expandIndicator = "▼"
		} else {
			icon = "📁"
			expandIndicator = "▶"
		}
	} else if node.File != nil {
		icon = node.File.Icon()
	} else {
		icon = "📄"
	}

	// Checkbox for files and directories
	checkbox := "  "
	if node.IsDir {
		// Directory - show selection state of children
		allSelected := l.areAllChildrenSelected(node)
		someSelected := l.areSomeChildrenSelected(node)
		if allSelected {
			checkbox = ui.RenderCheckbox(true) // [x]
		} else if someSelected {
			checkbox = "[-]" // Partial selection
		} else {
			checkbox = ui.RenderCheckbox(false) // [ ]
		}
	} else if node.File != nil {
		checkbox = ui.RenderCheckbox(node.File.Selected)
	}

	// File/dir name with expand indicator for directories
	name := node.Name
	if node.IsDir && expandIndicator != "" {
		name = expandIndicator + " " + name
	}
	maxNameLen := l.Width - 18 - (node.Depth * 2)
	if maxNameLen < 10 {
		maxNameLen = 10
	}
	if len(name) > maxNameLen {
		name = name[:maxNameLen-3] + "..."
	}

	// Status and suffix for files
	suffix := ""
	statusIcon := ""
	var statusStyle = ui.SyncedStyle

	if node.File != nil {
		// Add encrypted indicator
		if node.File.Encrypted {
			suffix = " " + ui.EncryptedStyle.Render("🔒")
		}

		// Status based on conflict type
		statusIcon = node.File.ConflictType.ConflictIcon()
		switch node.File.ConflictType {
		case models.ConflictLocalModified, models.ConflictLocalNew:
			statusStyle = ui.ModifiedStyle
		case models.ConflictDotfilesModified, models.ConflictDotfilesNew:
			statusStyle = ui.OutdatedStyle
		case models.ConflictBothModified:
			statusStyle = ui.ConflictStyle
		case models.ConflictLocalDeleted, models.ConflictDotfilesDeleted:
			statusStyle = ui.MissingStyle
		case models.ConflictNone:
			statusStyle = ui.SyncedStyle
		default:
			statusIcon = node.File.SyncStatus.StatusIcon()
			switch node.File.SyncStatus {
			case models.StatusModified:
				statusStyle = ui.ModifiedStyle
			case models.StatusNew:
				statusStyle = ui.NewStyle
			case models.StatusMissing:
				statusStyle = ui.MissingStyle
			}
		}
	}

	// Build content
	var content string
	if node.Depth == 0 {
		// Top level - no connector
		content = fmt.Sprintf("%s%s %s%s",
			checkbox,
			icon,
			ui.FileNameStyle.Render(name),
			suffix,
		)
	} else {
		content = fmt.Sprintf("%s%s %s%s %s%s",
			indent,
			MutedStyle.Render(connector),
			checkbox,
			icon,
			ui.FileNameStyle.Render(name),
			suffix,
		)
	}

	// Add status for files
	if statusIcon != "" {
		content += " " + statusStyle.Render(statusIcon)
	}

	if isCursor && l.Focused {
		return ui.SelectedItemStyle.Width(l.Width - 4).Render(content)
	}
	return ui.ItemStyle.Render(content)
}

// renderFlatView renders the flat file list (fallback)
func (l *FileList) renderFlatView(b *strings.Builder) string {
	visibleHeight := l.Height - 3
	startIdx := 0
	if l.Cursor >= visibleHeight {
		startIdx = l.Cursor - visibleHeight + 1
	}
	endIdx := min(startIdx+visibleHeight, len(l.Files))

	if startIdx > 0 {
		b.WriteString(MutedStyle.Render("  ↑ more"))
		b.WriteString("\n")
	}

	for i := startIdx; i < endIdx; i++ {
		file := l.Files[i]
		line := l.renderItem(&file, i == l.Cursor)
		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	if endIdx < len(l.Files) {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("  ↓ more"))
	}

	if len(l.Files) > visibleHeight {
		position := fmt.Sprintf(" %d/%d ", l.Cursor+1, len(l.Files))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(strings.Repeat(" ", (l.Width-len(position)-4)/2) + position))
	}

	return l.wrapInPanel(b.String())
}

// renderItem renders a single file item (for flat view)
func (l *FileList) renderItem(file *models.File, isCursor bool) string {
	checkbox := ui.RenderCheckbox(file.Selected)
	icon := file.Icon()

	name := file.RelPath
	if name == "" {
		name = file.Name
	}
	maxNameLen := l.Width - 15
	if len(name) > maxNameLen {
		name = "..." + name[len(name)-maxNameLen+3:]
	}

	suffix := ""
	if file.Encrypted {
		suffix = " " + ui.EncryptedStyle.Render("🔒")
	}

	statusIcon := file.ConflictType.ConflictIcon()
	var statusStyle = ui.SyncedStyle
	switch file.ConflictType {
	case models.ConflictLocalModified, models.ConflictLocalNew:
		statusStyle = ui.ModifiedStyle
	case models.ConflictDotfilesModified, models.ConflictDotfilesNew:
		statusStyle = ui.OutdatedStyle
	case models.ConflictBothModified:
		statusStyle = ui.ConflictStyle
	case models.ConflictLocalDeleted, models.ConflictDotfilesDeleted:
		statusStyle = ui.MissingStyle
	case models.ConflictNone:
		statusStyle = ui.SyncedStyle
	default:
		statusIcon = file.SyncStatus.StatusIcon()
		switch file.SyncStatus {
		case models.StatusModified:
			statusStyle = ui.ModifiedStyle
		case models.StatusNew:
			statusStyle = ui.NewStyle
		case models.StatusMissing:
			statusStyle = ui.MissingStyle
		}
	}

	content := fmt.Sprintf("%s %s %s%s %s",
		checkbox,
		icon,
		ui.FileNameStyle.Render(name),
		suffix,
		statusStyle.Render(statusIcon),
	)

	if isCursor && l.Focused {
		return ui.SelectedItemStyle.Width(l.Width - 4).Render(content)
	}
	return ui.ItemStyle.Render(content)
}

// wrapInPanel wraps content in a panel border
func (l *FileList) wrapInPanel(content string) string {
	style := ui.PanelStyle
	if l.Focused {
		style = ui.ActivePanelStyle
	}
	return style.Width(l.Width).Height(l.Height).Render(content)
}
