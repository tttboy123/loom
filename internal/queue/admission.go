package queue

import "fmt"

// CompileSubmission validates eligibility, vertical capability, exit
// conditions, verification/integration strategy, and protected-authority
// claims (Decomposition Compiler rules 4/6/7/8) and returns the compiled
// projection record fields. It creates no Journal facts.
func CompileSubmission(input JobSubmission) (CompiledJob, error) {
	if err := ValidateJobSubmission(input); err != nil {
		return CompiledJob{}, err
	}
	switch input.EligibilityAuthority {
	case "user":
	case "bounded_policy":
	default:
		return CompiledJob{}, fmt.Errorf("%w: eligibility authority missing or unsupported", ErrDenied)
	}
	if thinCapability(input.CapabilityKind) {
		return CompiledJob{}, fmt.Errorf("%w: thin decomposition (capability_kind %q)", ErrDenied, input.CapabilityKind)
	}
	if len(input.ExitConditions) == 0 {
		return CompiledJob{}, fmt.Errorf("%w: exit conditions are required", ErrDenied)
	}
	if input.VerificationStrategy == "" {
		return CompiledJob{}, fmt.Errorf("%w: verification strategy is required", ErrDenied)
	}
	if input.IntegrationStrategy == "" {
		return CompiledJob{}, fmt.Errorf("%w: integration strategy is required", ErrDenied)
	}
	for _, path := range input.ProtectedAuthorityPaths {
		if protectedAuthorityClaim(path) {
			return CompiledJob{}, fmt.Errorf(
				"%w: protected authority path claim %q requires a separately reviewed human-governed contract",
				ErrDenied, path,
			)
		}
	}
	return input.compiled(), nil
}

// ValidateDAG rejects a candidate whose dependencies create a cycle among
// active queue nodes, or whose dag_node_id duplicates active (non-terminal)
// work. Terminal duplicates are allowed (a later lineage may reuse the node
// id after the earlier lineage completed).
func ValidateDAG(active []QueueJob, candidate CompiledJob) error {
	nodes := make(map[string][]string, len(active)+1)
	for _, job := range active {
		if terminalStatus(job.Status) {
			continue
		}
		if job.DAGNodeID == candidate.DAGNodeID {
			return fmt.Errorf("%w: dag node %q already active", ErrDuplicateWork, candidate.DAGNodeID)
		}
		nodes[job.DAGNodeID] = append([]string(nil), job.Dependencies...)
	}
	nodes[candidate.DAGNodeID] = append([]string(nil), candidate.Dependencies...)
	visited := make(map[string]int, len(nodes))
	var visit func(node string, stack []string) error
	visit = func(node string, stack []string) error {
		if visited[node] == 2 {
			return nil
		}
		if visited[node] == 1 {
			return fmt.Errorf("%w: cycle involving %q", ErrDAGCycle, node)
		}
		visited[node] = 1
		for _, dependency := range nodes[node] {
			if _, known := nodes[dependency]; known {
				if err := visit(dependency, append(stack, node)); err != nil {
					return err
				}
			}
		}
		visited[node] = 2
		return nil
	}
	return visit(candidate.DAGNodeID, nil)
}
