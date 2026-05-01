package validation

import (
	"testing"

	"task-ledger/internal/types"
)

func TestIssueValidators(t *testing.T) {
	if err := Exists()("tl-test", nil); err == nil {
		t.Fatal("Exists should fail for nil issue")
	}
	if err := Exists()("tl-test", &types.Issue{ID: "tl-test"}); err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}

	if err := NotPinned(false)("tl-test", &types.Issue{ID: "tl-test", Status: types.StatusPinned}); err == nil {
		t.Fatal("NotPinned should fail for pinned issue without force")
	}
	if err := NotPinned(true)("tl-test", &types.Issue{ID: "tl-test", Status: types.StatusPinned}); err != nil {
		t.Fatalf("NotPinned with force returned error: %v", err)
	}

	if err := NotClosed()("tl-test", &types.Issue{ID: "tl-test", Status: types.StatusClosed}); err == nil {
		t.Fatal("NotClosed should fail for closed issue")
	}
}

func TestChain(t *testing.T) {
	chain := Chain(Exists(), NotPinned(false))
	if err := chain("tl-test", &types.Issue{ID: "tl-test", Status: types.StatusOpen}); err != nil {
		t.Fatalf("chain returned error: %v", err)
	}
	if err := chain("tl-test", nil); err == nil {
		t.Fatal("chain should stop on first error")
	}
}

func TestHasStatusAndType(t *testing.T) {
	issue := &types.Issue{ID: "tl-test", Status: types.StatusOpen, IssueType: types.TypeTask}
	if err := HasStatus(types.StatusOpen, types.StatusBlocked)("tl-test", issue); err != nil {
		t.Fatalf("HasStatus returned error: %v", err)
	}
	if err := HasStatus(types.StatusClosed)("tl-test", issue); err == nil {
		t.Fatal("HasStatus should fail for non-matching status")
	}
	if err := HasType(types.TypeTask, types.TypeBug)("tl-test", issue); err != nil {
		t.Fatalf("HasType returned error: %v", err)
	}
	if err := HasType(types.TypeEpic)("tl-test", issue); err == nil {
		t.Fatal("HasType should fail for non-matching type")
	}
}

func TestOperationValidatorChains(t *testing.T) {
	open := &types.Issue{ID: "tl-test", Status: types.StatusOpen}
	closed := &types.Issue{ID: "tl-test", Status: types.StatusClosed}
	pinned := &types.Issue{ID: "tl-test", Status: types.StatusPinned}

	if err := ForUpdate()("tl-test", open); err != nil {
		t.Fatalf("ForUpdate returned error: %v", err)
	}
	if err := ForUpdate()("tl-test", nil); err == nil {
		t.Fatal("ForUpdate should fail for nil issue")
	}
	if err := ForClose(false)("tl-test", pinned); err == nil {
		t.Fatal("ForClose should fail for pinned issue without force")
	}
	if err := ForClose(true)("tl-test", pinned); err != nil {
		t.Fatalf("ForClose with force returned error: %v", err)
	}
	if err := ForDelete()("tl-test", open); err != nil {
		t.Fatalf("ForDelete returned error: %v", err)
	}
	if err := ForReopen()("tl-test", closed); err != nil {
		t.Fatalf("ForReopen returned error: %v", err)
	}
	if err := ForReopen()("tl-test", open); err == nil {
		t.Fatal("ForReopen should fail for open issue")
	}
}
