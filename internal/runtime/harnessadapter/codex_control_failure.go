package harnessadapter

import "errors"

const (
	codexControlFailureFirstTurn       = "first_turn"
	codexControlFailureSelectionDecode = "selection_decode"
	codexControlFailureDirectMatch     = "direct_match"
	codexControlFailureBrokerCall      = "broker_call"
	codexControlFailureBrokerInput     = "broker_input"
	codexControlFailureBrokerTransport = "broker_transport"
	codexControlFailureBrokerInvalid   = "broker_invalid_request"
	codexControlFailureBrokerTurn      = "broker_turn_unavailable"
	codexControlFailureBrokerLimit     = "broker_call_limit"
	codexControlFailureBrokerTool      = "broker_tool_unavailable"
	codexControlFailureBrokerResponse  = "broker_response_validation"
	codexControlFailureBrokerSuspend   = "broker_suspend"
	codexControlFailureResultSummary   = "result_summary"
	codexControlFailureAccounting      = "accounting"
	codexControlFailureResultDecode    = "result_decode"
)

type codexControlArbitrationFailure struct {
	stage string
	cause error
}

func (failure *codexControlArbitrationFailure) Error() string {
	return ErrHarnessControlMCP.Error()
}

func (failure *codexControlArbitrationFailure) Unwrap() error {
	return failure.cause
}

func (failure *codexControlArbitrationFailure) CodexControlArbitrationFailureStage() string {
	return failure.stage
}

func newCodexControlArbitrationFailure(stage string, cause error) error {
	if !validCodexControlArbitrationFailureStage(stage) {
		return errors.Join(ErrHarnessControlMCP, cause)
	}
	return &codexControlArbitrationFailure{
		stage: stage,
		cause: errors.Join(ErrHarnessControlMCP, cause),
	}
}

func CodexControlArbitrationFailureStage(err error) string {
	var failure interface {
		CodexControlArbitrationFailureStage() string
	}
	if !errors.As(err, &failure) || failure == nil {
		return ""
	}
	stage := failure.CodexControlArbitrationFailureStage()
	if !validCodexControlArbitrationFailureStage(stage) {
		return ""
	}
	return stage
}

func validCodexControlArbitrationFailureStage(stage string) bool {
	switch stage {
	case codexControlFailureFirstTurn,
		codexControlFailureSelectionDecode,
		codexControlFailureDirectMatch,
		codexControlFailureBrokerCall,
		codexControlFailureBrokerInput,
		codexControlFailureBrokerTransport,
		codexControlFailureBrokerInvalid,
		codexControlFailureBrokerTurn,
		codexControlFailureBrokerLimit,
		codexControlFailureBrokerTool,
		codexControlFailureBrokerResponse,
		codexControlFailureBrokerSuspend,
		codexControlFailureResultSummary,
		codexControlFailureAccounting,
		codexControlFailureResultDecode:
		return true
	default:
		return false
	}
}

func codexControlBrokerFailureStage(err error) string {
	switch HarnessControlCallFailureCode(err) {
	case harnessControlCallFailureInputAdmission,
		harnessControlCallFailureRequestEncoding:
		return codexControlFailureBrokerInput
	case harnessControlCallFailureTransport:
		return codexControlFailureBrokerTransport
	case harnessControlCallFailureInvalidRequest:
		return codexControlFailureBrokerInvalid
	case harnessControlCallFailureTurnUnavailable:
		return codexControlFailureBrokerTurn
	case harnessControlCallFailureCallLimit:
		return codexControlFailureBrokerLimit
	case harnessControlCallFailureToolUnavailable:
		return codexControlFailureBrokerTool
	case harnessControlCallFailureResponseValidation:
		return codexControlFailureBrokerResponse
	default:
		return codexControlFailureBrokerCall
	}
}
