package testsuite

const (
	MaxSuites      = 64
	MaxCases       = 128
	MaxFixtures    = 1024
	MaxEncodedSize = 1 << 20
	CaseTimeoutSec = 5
)

var TargetKinds = []string{"Action", "Rule"}

var ErrorCodes = []string{
	"RULE_DIVIDE_BY_ZERO",
	"RULE_INVALID_ARITY",
	"RULE_INVALID_SHAPE",
	"RULE_MISSING_VALUE",
	"RULE_RESOURCE_LIMIT",
	"RULE_TYPE_MISMATCH",
	"RULE_UNKNOWN_OPERATOR",
	"RULE_UNKNOWN_PATH",
	"RULE_UNKNOWN_SOURCE",
	"busy",
	"conflict",
	"foreign_key_violation",
	"internal",
	"invalid_query",
	"not_found",
	"serialization_failure",
	"timeout",
	"unavailable",
	"unique_violation",
}
