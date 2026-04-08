## Improve logs on GCP

Currently we print a lot of logs that make it hard to debug manually when an error occurs.

## Goals

- Research on ways we can better structure logs
- Review what packages we are using for logs and how the code is structured
- Reduce the amount of white noise currently in our logs
- See if we improve the quality of the logs such as printing GraphQL queries and mutations to better understand
  what happens

## Plan

- Carry out the research. Once done, simulate by running the application locally and see if the logs have improved
- Review logs in GCP to see where the noise is and see where we can improve the structure for a human to be able to
  debug
- Any pointers are welcome

## GCP Audit Findings (2026-04-06)

### 1. Workspace lookup spike — dashboard-api polling non-existent workspace (2026-04-06)

**Issue**: Dashboard-api repeatedly querying for workspace `01k1hr7z7w1se92vm01xm4y5ep` that doesn't exist in database

**Metrics**:
- **40 failed lookups** in 20 seconds (01:38:54 - 01:39:14 UTC / ~10:38-10:39 JST)
- **Polling interval**: ~2-4 seconds between requests
- **Pattern**: Single workspace ID, repeated lookups
- **Other failing workspaces**: 8 additional IDs with only 2 failures each (likely transient)

**Root cause — TBD**: Need to investigate:
1. Was workspace `01k1hr7z7w1se92vm01xm4y5ep` recently deleted?
2. Does dashboard-api have stale cached/session state referencing this workspace?
3. Is there retry/polling logic in dashboard-api that got stuck?

**Evidence**:
- GraphQL query from dashboard-api: `query ($id:ID!){findByID(id: $id){...}}`
- Error: `[FindByID] failed to fetch workspace by id: not found, workspace id: 01k1hr7z7w1se92vm01xm4y5ep`
- Database returns no record for this workspace ID

**Suggested fixes**:
1. Check dashboard-api logs for context on why it's polling this workspace
2. Add request tracing (request_id/correlation_id) to link accounts-api errors back to caller
3. Add workspace existence check at application startup or session creation
4. Consider adding stale workspace detection/cleanup

**Status**: Investigation needed

### 2. Distributed trace context propagation

**Goal**: Enable request tracing from visualizer → accounts API → database for better correlation

**Implementation**:

1. **HTTP middleware for incoming requests**
   - Register `otelecho.Middleware()` to extract trace context from inbound requests
   - Extracts W3C `traceparent` header (Cloud Run sends this automatically alongside `X-Cloud-Trace-Context`)
   - Propagate through request context

2. **Wrap outbound HTTP clients**
   - Use `otelhttp.NewTransport()` to wrap any downstream HTTP calls
   - Automatically includes trace headers in requests

3. **Ensure structured logs include trace context**
   - OpenTelemetry automatically adds trace IDs to logs via context
   - GCP Cloud Logging receives top-level `trace` field for log correlation

**Benefits**:
- Link workspace lookup failures back to the caller (visualizer)
- Trace request flow: visualizer request → accounts API → database query
- Reduce debugging time by having all related logs grouped in GCP

**Status**: Ready to implement
