package core

import "testing"

func TestBuildOperations(t *testing.T) {
	cardCom := 2
	irs1 := 3
	irs2 := 4
	req := ApplyRequest{
		Card: BasicTargetInput{Selected: true, Com: &cardCom},
		IRS: IrsTargetInput{
			Selected:   true,
			Device1Com: &irs1,
			UseDevice2: true,
			Device2Com: &irs2,
		},
		NM43: BasicTargetInput{Selected: false},
	}

	ops, err := BuildOperations(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}
	if ops[0].Target != TargetCard {
		t.Fatalf("expected card target, got %s", ops[0].Target)
	}
	if ops[1].Target != TargetIRS {
		t.Fatalf("expected irs target, got %s", ops[1].Target)
	}
}

func TestBuildOperationsNoSelection(t *testing.T) {
	_, err := BuildOperations(ApplyRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
