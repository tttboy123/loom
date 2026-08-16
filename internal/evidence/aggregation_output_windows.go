//go:build windows

package evidence

import "context"

const AggregationScopeAuthorizedOutputEvents = "authorized_output_events"

type AggregationOutput struct{}

func (AggregationOutput) Binding() AttemptCaptureInput { return AttemptCaptureInput{} }
func (AggregationOutput) EvidenceDigest() string       { return "" }
func (AggregationOutput) OutputSummaryDigest() string  { return "" }
func (AggregationOutput) Content() []byte              { return nil }
func (*AggregationOutput) Close()                      {}

func (*Store) ReadAggregationOutput(
	context.Context,
	AttemptReceipt,
	int64,
) (AggregationOutput, error) {
	return AggregationOutput{}, ErrUnsupportedPlatform
}
