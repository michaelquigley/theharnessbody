package repo

import "testing"

func TestParseStatus(t *testing.T) {
	out := "## main...origin/main [ahead 1, behind 2]\n" +
		" M file1.go\n" +
		"A  file2.go\n" +
		"?? new.txt\n" +
		" D gone.go\n"
	s := parseStatus(out)

	if s.Branch != "main" {
		t.Errorf("branch = %q, want main", s.Branch)
	}
	if s.Ahead != 1 || s.Behind != 2 {
		t.Errorf("ahead/behind = %d/%d, want 1/2", s.Ahead, s.Behind)
	}
	if len(s.Modified) != 1 || s.Modified[0] != "file1.go" {
		t.Errorf("modified = %v", s.Modified)
	}
	if len(s.Added) != 1 || s.Added[0] != "file2.go" {
		t.Errorf("added = %v", s.Added)
	}
	if len(s.Untracked) != 1 || s.Untracked[0] != "new.txt" {
		t.Errorf("untracked = %v", s.Untracked)
	}
	if len(s.Deleted) != 1 || s.Deleted[0] != "gone.go" {
		t.Errorf("deleted = %v", s.Deleted)
	}
	if !s.HasChanges() || s.ChangeCount() != 4 {
		t.Errorf("HasChanges=%v ChangeCount=%d, want true/4", s.HasChanges(), s.ChangeCount())
	}
}

func TestParseStatusDottedBranch(t *testing.T) {
	s := parseStatus("## release/v1.2...origin/release/v1.2 [ahead 3, behind 4]\n M f.go\n")
	if s.Branch != "release/v1.2" {
		t.Errorf("branch = %q, want release/v1.2", s.Branch)
	}
	if s.Ahead != 3 || s.Behind != 4 {
		t.Errorf("ahead/behind = %d/%d, want 3/4 (lost for a dotted branch?)", s.Ahead, s.Behind)
	}
}

func TestParseStatusDottedBranchNoRemote(t *testing.T) {
	s := parseStatus("## release/v1.2\n")
	if s.Branch != "release/v1.2" {
		t.Errorf("branch = %q, want release/v1.2", s.Branch)
	}
}

func TestParseStatusClean(t *testing.T) {
	s := parseStatus("## main...origin/main\n")
	if s.Branch != "main" {
		t.Errorf("branch = %q, want main", s.Branch)
	}
	if s.HasChanges() || s.ChangeCount() != 0 {
		t.Errorf("expected clean tree, got HasChanges=%v count=%d", s.HasChanges(), s.ChangeCount())
	}
}
