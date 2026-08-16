package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/work"
)

func TestCOMP2CGovernanceConstructsRoutesInsideBundleAndRevokesThem(t *testing.T) {
	constructed := 0
	closed := 0
	slot := &productGovernanceRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-governance", nil,
		productCompatibilityConstruction{
			governanceSlot: slot,
			governanceFactory: func(context.Context) (productGovernanceRoutes, error) {
				constructed++
				routes := productGovernanceRoutesFixture()
				routes.close = func() error { closed++; return nil }
				return routes, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if constructed != 1 || !slot.Ready() {
		t.Fatalf("constructed=%d ready=%t", constructed, slot.Ready())
	}
	permission, err := slot.PermissionSnapshot(
		context.Background(), app.PermissionSnapshotRequest{},
	)
	if err != nil || permission.ViewVersion != "fixture-permission" {
		t.Fatalf("permission=%#v err=%v", permission, err)
	}
	customerRule, err := (productCustomerRuleRouteProxy{slot: slot}).Snapshot(
		context.Background(), app.CustomerRuleSnapshotRequest{},
	)
	if err != nil || customerRule.ViewVersion != "fixture-customer-rule" {
		t.Fatalf("customerRule=%#v err=%v", customerRule, err)
	}
	standingOrder, err := (productStandingOrderRouteProxy{slot: slot}).Snapshot(
		context.Background(), app.StandingOrderSnapshotRequest{},
	)
	if err != nil || standingOrder.ViewVersion != "fixture-standing-order" {
		t.Fatalf("standingOrder=%#v err=%v", standingOrder, err)
	}
	if approvals, err := slot.Approvals(); err != nil || approvals == nil {
		t.Fatalf("approvals=%#v err=%v", approvals, err)
	}
	decision, err := slot.ReadMissionDecision(
		context.Background(), app.MissionDecisionCommand{},
	)
	if err != nil || decision.Title != "fixture-decision" {
		t.Fatalf("decision=%#v err=%v", decision, err)
	}
	if _, err := slot.ListMissionDecisionCommands(
		context.Background(), app.MissionDecisionCommandQuery{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := slot.ConfigureProviderAccountPolicy(
		context.Background(), work.ProviderAccountPolicyCommand{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := slot.ConfigureProviderModelRateCard(
		context.Background(), work.ProviderModelRateCardCommand{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := slot.ConfigureRemoteToolBackendEnrollment(
		context.Background(), work.RemoteToolBackendEnrollmentCommand{},
	); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := slot.PermissionSnapshot(
		context.Background(), app.PermissionSnapshotRequest{},
	); !errors.Is(err, api.ErrInvalidLocalPermissionAPI) {
		t.Fatalf("closed Permission route err=%v", err)
	}
	if _, err := slot.Approvals(); !errors.Is(err, api.ErrInvalidLocalPermissionAPI) {
		t.Fatalf("closed approval port err=%v", err)
	}
	if _, err := slot.ListMissionDecisionCommands(
		context.Background(), app.MissionDecisionCommandQuery{},
	); !errors.Is(err, api.ErrInvalidLocalProductDecisionAPI) {
		t.Fatalf("closed decision source err=%v", err)
	}
	if _, err := slot.ConfigureProviderAccountPolicy(
		context.Background(), work.ProviderAccountPolicyCommand{},
	); !errors.Is(err, app.ErrProviderAccountPolicyUnavailable) {
		t.Fatalf("closed Provider policy port err=%v", err)
	}
	if _, err := slot.ConfigureRemoteToolBackendEnrollment(
		context.Background(), work.RemoteToolBackendEnrollmentCommand{},
	); !errors.Is(err, app.ErrRemoteToolBackendEnrollmentUnavailable) {
		t.Fatalf("closed remote tool enrollment port err=%v", err)
	}
	if closed != 1 {
		t.Fatalf("Governance close count=%d", closed)
	}
}

func TestCOMP2CGovernanceFailurePreventsWorkAndProductScope(t *testing.T) {
	recorder := &compositionTestRecorder{}
	workConstructed := false
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-governance-failure", recorder,
		productCompatibilityConstruction{
			governanceSlot: &productGovernanceRouteSlot{},
			governanceFactory: func(context.Context) (productGovernanceRoutes, error) {
				return productGovernanceRoutes{}, errors.New("Governance construction failed")
			},
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				workConstructed = true
				return productWorkRoutesFixture(), nil
			},
		},
	)
	if err == nil || facade != nil || workConstructed {
		t.Fatalf("facade=%#v err=%v workConstructed=%t", facade, err, workConstructed)
	}
	for _, record := range recorder.snapshot() {
		if record.ScopeKind == composition.ScopeProduct && record.Stage == composition.StageScopeOpen {
			t.Fatalf("Product scope opened after Governance failure: %#v", record)
		}
	}
}

func TestCOMP2CGovernanceStartsBeforeWorkAndClosesAfterIt(t *testing.T) {
	events := make([]string, 0, 4)
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-governance-order", nil,
		productCompatibilityConstruction{
			governanceSlot: &productGovernanceRouteSlot{},
			governanceFactory: func(context.Context) (productGovernanceRoutes, error) {
				events = append(events, "governance_start")
				routes := productGovernanceRoutesFixture()
				routes.close = func() error {
					events = append(events, "governance_close")
					return nil
				}
				return routes, nil
			},
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				events = append(events, "work_start")
				routes := productWorkRoutesFixture()
				routes.close = func() error {
					events = append(events, "work_close")
					return nil
				}
				return routes, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	want := []string{"governance_start", "work_start", "work_close", "governance_close"}
	if len(events) != len(want) {
		t.Fatalf("events=%v want=%v", events, want)
	}
	for index := range want {
		if events[index] != want[index] {
			t.Fatalf("events=%v want=%v", events, want)
		}
	}
}

func TestCOMP2CProductionBuilderDoesNotConstructGovernanceOutsideBundle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" &&
			function.Name.Name != "buildProductSetupService" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch target := call.Fun.(type) {
			case *ast.Ident:
				name = target.Name
			case *ast.SelectorExpr:
				name = target.Sel.Name
			}
			if function.Name.Name == "buildProductSetupService" {
				if name == "NewAuthority" {
					t.Errorf("Setup constructs Provider policy authority outside loom-governance Bundle")
				}
				return true
			}
			switch name {
			case "newPermissionApprovalPort", "NewLocalPermissionService",
				"NewLocalPermissionAPI", "NewLocalCustomerRuleService",
				"NewLocalCustomerRuleAPI", "NewStandingOrderAuthority",
				"NewLocalStandingOrderService", "NewLocalStandingOrderAPI",
				"NewPreparedMissionDecisionBackend", "NewLocalProductSetupWriter",
				"NewProjectionMissionFallbackDecisionPreparer",
				"NewLocalProductDecisionService",
				"NewPreparedMissionExecutionDecisionRouter",
				"NewLocalProductDecisionAPI":
				t.Errorf("production constructs %s outside loom-governance Bundle", name)
			}
			return true
		})
	}
}

func TestCOMP2CGovernanceConstructionPreservesBuildFailureStage(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{errors.Join(errProductGovernancePermissionConstruction, errors.New("private")), "build_permissions"},
		{errors.Join(errProductGovernanceCustomerRuleConstruction, errors.New("private")), "build_customer_rule"},
		{errors.Join(errProductGovernanceStandingOrderConstruction, errors.New("private")), "build_standing_order"},
		{errors.Join(errProductGovernanceDecisionConstruction, errors.New("private")), "build_decision"},
		{errors.Join(errProductGovernanceProviderPolicyConstruction, errors.New("private")), "build_setup_policy"},
	} {
		if got := productCompatibilityBuildFailureReason(test.err); got != test.want {
			t.Fatalf("failure reason=%q want=%q", got, test.want)
		}
	}
}

type productPermissionRouteFixture struct{}

func (productPermissionRouteFixture) PermissionSnapshot(
	context.Context, app.PermissionSnapshotRequest,
) (app.PermissionSnapshot, error) {
	return app.PermissionSnapshot{ViewVersion: "fixture-permission"}, nil
}
func (productPermissionRouteFixture) PermissionAttention(
	context.Context, app.PermissionAttentionRequest,
) (app.PermissionAttention, error) {
	return app.PermissionAttention{}, nil
}
func (productPermissionRouteFixture) PermissionCommand(
	context.Context, app.PermissionCommandRequest,
) (app.PermissionCommandResult, error) {
	return app.PermissionCommandResult{}, nil
}

type productCustomerRuleRouteFixture struct{}

func (productCustomerRuleRouteFixture) Snapshot(
	context.Context, app.CustomerRuleSnapshotRequest,
) (app.CustomerRuleSnapshot, error) {
	return app.CustomerRuleSnapshot{ViewVersion: "fixture-customer-rule"}, nil
}
func (productCustomerRuleRouteFixture) Command(
	context.Context, app.CustomerRuleCommandRequest,
) (app.CustomerRuleCommandResult, error) {
	return app.CustomerRuleCommandResult{}, nil
}

type productStandingOrderRouteFixture struct{}

func (productStandingOrderRouteFixture) Snapshot(
	context.Context, app.StandingOrderSnapshotRequest,
) (app.StandingOrderSnapshot, error) {
	return app.StandingOrderSnapshot{ViewVersion: "fixture-standing-order"}, nil
}
func (productStandingOrderRouteFixture) Command(
	context.Context, app.StandingOrderCommandRequest,
) (app.StandingOrderCommandResult, error) {
	return app.StandingOrderCommandResult{}, nil
}

type productApprovalPortFixture struct{}

func (productApprovalPortFixture) RequestPermissionApproval(
	context.Context, rules.PermissionApprovalInput,
) (rules.ApprovalRequestRecord, error) {
	return rules.ApprovalRequestRecord{}, nil
}
func (productApprovalPortFixture) ConsumePermissionApproval(
	context.Context, rules.PermissionApprovalConsumptionInput,
) (rules.ApprovalRequestRecord, error) {
	return rules.ApprovalRequestRecord{}, nil
}

type productDecisionRouteFixture struct{}

func (productDecisionRouteFixture) ReadMissionDecision(
	context.Context, app.MissionDecisionCommand,
) (app.MissionDecisionSheet, error) {
	return app.MissionDecisionSheet{Title: "fixture-decision"}, nil
}
func (productDecisionRouteFixture) DecideMission(
	context.Context, app.MissionDecisionCommand,
) (app.MissionDecisionResult, error) {
	return app.MissionDecisionResult{}, nil
}

type productDecisionCommandSourceFixture struct{}

func (productDecisionCommandSourceFixture) ListMissionDecisionCommands(
	context.Context, app.MissionDecisionCommandQuery,
) ([]app.MissionDecisionCommand, error) {
	return nil, nil
}

type productExecutionDecisionRouterFixture struct{}

func (productExecutionDecisionRouterFixture) RouteMissionExecutionControl(
	context.Context, app.MissionExecutionCommand,
) error {
	return nil
}

type productFallbackDecisionPreparerFixture struct{}

func (productFallbackDecisionPreparerFixture) PrepareMissionFallbackDecisions(
	context.Context, string, string, []app.MissionFallbackDecisionCandidate,
) error {
	return nil
}

type productProviderPolicyAuthorityFixture struct{}

func (productProviderPolicyAuthorityFixture) ConfigureProviderAccountPolicy(
	context.Context, work.ProviderAccountPolicyCommand,
) (work.ProviderAccountPolicy, error) {
	return work.ProviderAccountPolicy{}, nil
}

func (productProviderPolicyAuthorityFixture) ConfigureProviderModelRateCard(
	context.Context, work.ProviderModelRateCardCommand,
) (work.ProviderModelRateCard, error) {
	return work.ProviderModelRateCard{}, nil
}

func (productProviderPolicyAuthorityFixture) ConfigureRemoteToolBackendEnrollment(
	context.Context, work.RemoteToolBackendEnrollmentCommand,
) (work.RemoteToolBackendEnrollment, error) {
	return work.RemoteToolBackendEnrollment{}, nil
}

func (productProviderPolicyAuthorityFixture) RevokeRemoteToolBackendEnrollment(
	context.Context, work.RemoteToolBackendEnrollmentRevokeCommand,
) (work.RemoteToolBackendEnrollment, error) {
	return work.RemoteToolBackendEnrollment{}, nil
}

func productGovernanceRoutesFixture() productGovernanceRoutes {
	return productGovernanceRoutes{
		permission:                   productPermissionRouteFixture{},
		customerRule:                 productCustomerRuleRouteFixture{},
		standingOrder:                productStandingOrderRouteFixture{},
		approvals:                    productApprovalPortFixture{},
		decision:                     productDecisionRouteFixture{},
		decisionCommands:             productDecisionCommandSourceFixture{},
		executionDecisions:           productExecutionDecisionRouterFixture{},
		fallbackDecisions:            productFallbackDecisionPreparerFixture{},
		providerAccountPolicies:      productProviderPolicyAuthorityFixture{},
		providerModelRateCards:       productProviderPolicyAuthorityFixture{},
		remoteToolBackendEnrollments: productProviderPolicyAuthorityFixture{},
		close:                        func() error { return nil },
	}
}
