package suggestions

import (
	"testing"

	"dotsync/internal/modes"
)

func TestSuggestionTypeString(t *testing.T) {
	tests := []struct {
		typ      SuggestionType
		expected string
	}{
		{TypeAllSynced, "all_synced"},
		{TypeLocalModified, "local_modified"},
		{TypeRemoteUpdated, "remote_updated"},
		{TypeConflicts, "conflicts"},
		{TypeFirstRun, "first_run"},
		{SuggestionType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.expected {
			t.Errorf("SuggestionType(%d).String() = %s, want %s", tt.typ, got, tt.expected)
		}
	}
}

func TestSuggestionIcon(t *testing.T) {
	tests := []struct {
		typ      SuggestionType
		expected string
	}{
		{TypeAllSynced, "[OK]"},
		{TypeLocalModified, "[UP]"},
		{TypeRemoteUpdated, "[DN]"},
		{TypeConflicts, "[!!]"},
		{TypeFirstRun, "[HI]"},
	}

	for _, tt := range tests {
		s := &Suggestion{Type: tt.typ}
		if got := s.Icon(); got != tt.expected {
			t.Errorf("Suggestion{Type: %v}.Icon() = %s, want %s", tt.typ, got, tt.expected)
		}
	}
}

func TestSuggestionIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		s        *Suggestion
		expected bool
	}{
		{"all synced no files", &Suggestion{Type: TypeAllSynced, Files: nil}, true},
		{"all synced with files", &Suggestion{Type: TypeAllSynced, Files: []string{"a.txt"}}, false},
		{"local modified", &Suggestion{Type: TypeLocalModified, Files: []string{"a.txt"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnalyzerFirstRun(t *testing.T) {
	analyzer := NewAnalyzer(modes.Default())

	state := &SyncState{
		IsFirstRun: true,
	}

	suggestion := analyzer.AnalyzeState(state)

	if suggestion.Type != TypeFirstRun {
		t.Errorf("expected TypeFirstRun, got %v", suggestion.Type)
	}
	if len(suggestion.Actions) == 0 {
		t.Error("expected actions for first run")
	}
}

func TestAnalyzerConflicts(t *testing.T) {
	analyzer := NewAnalyzer(modes.Default())

	state := &SyncState{
		Files: []FileState{
			{Path: "a.txt", HasConflict: true},
			{Path: "b.txt", LocalModified: true},
		},
	}

	suggestion := analyzer.AnalyzeState(state)

	if suggestion.Type != TypeConflicts {
		t.Errorf("expected TypeConflicts, got %v", suggestion.Type)
	}
	if suggestion.Count != 1 {
		t.Errorf("expected count 1, got %d", suggestion.Count)
	}
}

func TestAnalyzerLocalModified(t *testing.T) {
	analyzer := NewAnalyzer(modes.Default())

	state := &SyncState{
		Files: []FileState{
			{Path: "a.txt", LocalModified: true},
			{Path: "b.txt", LocalModified: true},
			{Path: "c.txt", RemoteModified: true},
		},
	}

	suggestion := analyzer.AnalyzeState(state)

	if suggestion.Type != TypeLocalModified {
		t.Errorf("expected TypeLocalModified, got %v", suggestion.Type)
	}
	if suggestion.Count != 2 {
		t.Errorf("expected count 2, got %d", suggestion.Count)
	}
}

func TestAnalyzerRemoteUpdated(t *testing.T) {
	analyzer := NewAnalyzer(modes.Default())

	state := &SyncState{
		Files: []FileState{
			{Path: "a.txt", RemoteModified: true},
		},
	}

	suggestion := analyzer.AnalyzeState(state)

	if suggestion.Type != TypeRemoteUpdated {
		t.Errorf("expected TypeRemoteUpdated, got %v", suggestion.Type)
	}
	if suggestion.Count != 1 {
		t.Errorf("expected count 1, got %d", suggestion.Count)
	}
}

func TestAnalyzerAllSynced(t *testing.T) {
	analyzer := NewAnalyzer(modes.Default())

	state := &SyncState{
		Files: []FileState{
			{Path: "a.txt"}, // no modifications
		},
	}

	suggestion := analyzer.AnalyzeState(state)

	if suggestion.Type != TypeAllSynced {
		t.Errorf("expected TypeAllSynced, got %v", suggestion.Type)
	}
}

func TestQuickAnalyze(t *testing.T) {
	tests := []struct {
		name          string
		local, remote int
		conflicts     int
		expectedType  SuggestionType
	}{
		{"conflicts priority", 5, 3, 2, TypeConflicts},
		{"local modified priority", 5, 3, 0, TypeLocalModified},
		{"remote updated", 0, 3, 0, TypeRemoteUpdated},
		{"all synced", 0, 0, 0, TypeAllSynced},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := QuickAnalyze(tt.local, tt.remote, tt.conflicts)
			if s.Type != tt.expectedType {
				t.Errorf("QuickAnalyze(%d, %d, %d) = %v, want %v",
					tt.local, tt.remote, tt.conflicts, s.Type, tt.expectedType)
			}
		})
	}
}

func TestQuickAnalyzeSingularMessages(t *testing.T) {
	// Test singular messages
	s := QuickAnalyze(1, 0, 0)
	if s.Message != "1 file modified locally" {
		t.Errorf("expected singular message, got %s", s.Message)
	}

	s = QuickAnalyze(0, 1, 0)
	if s.Message != "1 update available" {
		t.Errorf("expected singular message, got %s", s.Message)
	}

	s = QuickAnalyze(0, 0, 1)
	if s.Message != "1 conflict detected" {
		t.Errorf("expected singular message, got %s", s.Message)
	}
}
