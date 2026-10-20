package db

import "testing"

func TestBranchConversationCopiesHistoryAndIsolatesFutureTurns(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer d.Close()

	parent, err := d.CreateConversationInMode("reverse", ConversationModeReverse, "反编译样本", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteConversation(parent.ID)
	if _, err := d.AppendConvActivity(parent.ID, Activity{Worker: "reverse", Kind: "user", Summary: "检查入口"}); err != nil {
		t.Fatal(err)
	}

	branch, err := d.BranchConversation(parent.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteConversation(branch.ID)
	if branch.ParentID == nil || *branch.ParentID != parent.ID {
		t.Fatalf("branch parent = %v, want %d", branch.ParentID, parent.ID)
	}
	if branch.Mode != ConversationModeReverse || branch.AgentKey != "reverse" {
		t.Fatalf("branch mode/agent changed: %+v", branch)
	}
	if branch.Title != parent.Title+"（分支）" {
		t.Fatalf("branch title = %q", branch.Title)
	}

	parentHistory, _, err := d.ConvActivityList(parent.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	branchHistory, _, err := d.ConvActivityList(branch.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(parentHistory) != 1 || len(branchHistory) != 1 || branchHistory[0].Summary != parentHistory[0].Summary {
		t.Fatalf("branch did not inherit history: parent=%+v branch=%+v", parentHistory, branchHistory)
	}
	if _, err := d.AppendConvActivity(branch.ID, Activity{Worker: "reverse", Kind: "user", Summary: "只写入分支"}); err != nil {
		t.Fatal(err)
	}
	parentHistory, _, err = d.ConvActivityList(parent.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	branchHistory, _, err = d.ConvActivityList(branch.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(parentHistory) != 1 || len(branchHistory) != 2 {
		t.Fatalf("future turns are not isolated: parent=%d branch=%d", len(parentHistory), len(branchHistory))
	}
}
