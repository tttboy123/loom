package api

import (
	"context"
	"testing"

	"loom-pi-rebuild/internal/app"
)

type handoffServiceStub struct{ calls []string }

func (s *handoffServiceStub) ProposeSideTask(context.Context, app.SideTaskProposalRequest) (app.SideTaskProposalResult, error) {
	s.calls = append(s.calls, "propose")
	return app.SideTaskProposalResult{SchemaVersion: 1, Status: "proposal"}, nil
}
func (s *handoffServiceStub) CreateSideTask(context.Context, app.SideTaskCreateRequest) (app.SideTaskCreateResult, error) {
	s.calls = append(s.calls, "create")
	return app.SideTaskCreateResult{SchemaVersion: 1}, nil
}
func (s *handoffServiceStub) ReadSideTask(context.Context, app.SideTaskReadRequest) (app.SideTaskReadResult, error) {
	s.calls = append(s.calls, "read")
	return app.SideTaskReadResult{SchemaVersion: 1}, nil
}
func (s *handoffServiceStub) DecideSideTask(context.Context, app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error) {
	s.calls = append(s.calls, "decide")
	return app.SideTaskDecisionResult{SchemaVersion: 1}, nil
}

func TestLocalProductHandoffAPIUsesOneApplicationService(t *testing.T) {
	stub := &handoffServiceStub{}
	api, err := NewLocalProductHandoffAPI(stub)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err = api.ProposeSideTask(ctx, app.SideTaskProposalRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err = api.CreateSideTask(ctx, app.SideTaskCreateRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err = api.ReadSideTask(ctx, app.SideTaskReadRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err = api.DecideSideTask(ctx, app.SideTaskDecisionRequest{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"propose", "create", "read", "decide"}
	for i := range want {
		if stub.calls[i] != want[i] {
			t.Fatalf("calls=%v", stub.calls)
		}
	}
}
