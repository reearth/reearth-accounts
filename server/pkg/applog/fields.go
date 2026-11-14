package applog

import (
	"fmt"
	"time"
)

// Field is a typed log field. Its key always comes from the constructors in
// this file, so the set of keys accounts emits is fixed and queryable.
type Field struct {
	key   string
	value any
}

func (f Field) Key() string { return f.key }
func (f Field) Value() any  { return f.value }

func field(key string, value any) Field { return Field{key: key, value: value} }

func stringer(v any) any {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return v
}

// Request entry fields (ADR-0003 decision 9).

func RequestID(v string) Field        { return field("request_id", v) }
func ViewerID(v string) Field         { return field("viewer_id", v) }
func Caller(v string) Field           { return field("caller", v) }
func CallerClientID(v string) Field   { return field("caller_client_id", v) }
func Operation(v string) Field        { return field("operation", v) }
func OperationType(v string) Field    { return field("operation_type", v) }
func Fields(v []string) Field         { return field("fields", v) }
func Route(v string) Field            { return field("route", v) }
func Method(v string) Field           { return field("method", v) }
func Status(v int) Field              { return field("status", v) }
func LatencyMS(v time.Duration) Field { return field("latency_ms", v.Milliseconds()) }
func Err(v error) Field               { return field("error", v) }
func ErrMessage(v string) Field       { return field("error", v) }

// Resource identifiers. These are opaque IDs, never names, aliases or emails.

func UserID(v any) Field          { return field("user_id", stringer(v)) }
func WorkspaceID(v any) Field     { return field("workspace_id", stringer(v)) }
func WorkspaceIDs(v any) Field    { return field("workspace_ids", v) }
func AdminID(v any) Field         { return field("admin_id", stringer(v)) }
func NewOwnerID(v any) Field      { return field("new_owner_id", stringer(v)) }
func DemotedUserIDs(v any) Field  { return field("demoted_user_ids", v) }
func IntegrationID(v any) Field   { return field("integration_id", stringer(v)) }
func EnrollmentID(v string) Field { return field("enrollment_id", v) }

// Authorization.

func Role(v string) Field   { return field("role", v) }
func Roles(v any) Field     { return field("roles", v) }
func Action(v string) Field { return field("action", v) }
func Actions(v any) Field   { return field("actions", v) }
func Allowed(v bool) Field  { return field("allowed", v) }

// Token claims of a rejected client credentials token.

func Issuer(v string) Field  { return field("issuer", v) }
func Subject(v string) Field { return field("subject", v) }

// Rate limiting. LimitKey is the bucket key: user:<id>, or ip:<client IP> for an anonymous caller.

func LimitKey(v string) Field { return field("limit_key", v) }
func Enforced(v bool) Field   { return field("enforced", v) }

// Outcomes and counters.

func Path(v string) Field   { return field("path", v) }
func Code(v string) Field   { return field("code", v) }
func Reason(v string) Field { return field("reason", v) }
func Count(v int) Field     { return field("count", v) }

// Infrastructure.

func LockName(v string) Field            { return field("lock_name", v) }
func LockID(v string) Field              { return field("lock_id", v) }
func Host(v string) Field                { return field("host", v) }
func Command(v string) Field             { return field("command", v) }
func DurationMS(v time.Duration) Field   { return field("duration_ms", v.Milliseconds()) }
func Service(v string) Field             { return field("service", v) }
func Address(v string) Field             { return field("address", v) }
func Window(v time.Duration) Field       { return field("window", v.String()) }
func Config(v any) Field                 { return field("config", v) }
func Resource(v any) Field               { return field("resource", fmt.Sprint(v)) }
func Endpoint(v string) Field            { return field("endpoint", v) }
func Insecure(v bool) Field              { return field("insecure", v) }
func Exporter(v any) Field               { return field("exporter", fmt.Sprint(v)) }
func SamplingRatio(v float64) Field      { return field("sampling_ratio", v) }
func BatchTimeout(v time.Duration) Field { return field("batch_timeout", v.String()) }
func MaxExportBatchSize(v int) Field     { return field("max_export_batch_size", v) }
func MaxQueueSize(v int) Field           { return field("max_queue_size", v) }
