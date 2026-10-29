# costfluent-go

Go client for the [Costfluent](https://costfluent.com) Public API: typed models, workspace
scoping, pagination, retries and exact decimal money, with no dependency beyond the standard
library.

```bash
go get github.com/costfluent/costfluent-go
```

It needs Go 1.24 or later and an API token, created under **Settings**, **API tokens**, on a plan
that includes API access.

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/costfluent/costfluent-go/costfluent"
)

func main() {
    client := costfluent.NewClient(
        costfluent.WithAPIKey(os.Getenv("COSTFLUENT_API_KEY")),
    )

    ctx := context.Background()

    // List all providers
    providers, err := client.ListAllProviders(ctx, 100)
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range providers {
        log.Printf("Provider: %s (%s)", p.DisplayName, p.ID)
    }
}
```

## Configuration

```go
client := costfluent.NewClient(
    costfluent.WithAPIKey("cf_org_xxxx"),        // API key (required)
    costfluent.WithWorkspace("wsp_yyyy"),        // Default workspace
    costfluent.WithBaseURL("https://api.costfluent.com"), // Custom base URL
    costfluent.WithTimeout(60 * time.Second),    // Custom timeout
    costfluent.WithMaxRetryAfter(10 * time.Second), // Longest rate-limit wait to sit out (default 60s)
    costfluent.WithDebug(true),                  // Enable debug logging
)
```

The API key is an organization token (`cf_org_`) or a workspace token (`cf_ws_`). A workspace
token works in its own workspace only: list calls return that workspace's items, naming another
workspace returns a 403 error, and organization-level calls such as providers by ID, billing, and
creating or deleting a workspace need an organization token.

## Workspace Scoping

Workspace-scoped operations take a workspace ID argument, sent as the API's `workspaceId`. An
empty ID falls back to the client's default workspace, and a call with neither fails before it
is sent:

```go
client := costfluent.NewClient(
    costfluent.WithAPIKey(apiKey),
    costfluent.WithWorkspace("wsp_aaaa"),
)

foldersA, _ := client.ListFolders(ctx, "")          // wsp_aaaa, the default
foldersB, _ := client.ListFolders(ctx, "wsp_bbbb")  // explicit
wsC := client.Workspace("wsp_cccc")                 // a copy with another default
target := client.ResolveWorkspace("")               // "wsp_aaaa": the workspace a call would use
```

## Pagination

List methods support Vantage-style pagination with HATEOAS links:

```go
// Manual pagination
resp, err := client.ListProviders(ctx, &costfluent.PageOptions{
    Page:  1,
    Limit: 10,
})
// resp.Data contains []Provider
// resp.Links contains navigation links (self, first, prev, next, last)

// Check for next page
if resp.Links.HasNextPage() {
    // fetch next page
}

// Auto-pagination (fetches all pages)
all, err := client.ListAllProviders(ctx, 100) // page size
```

Response format follows Vantage API style:
```json
{
  "links": {
    "self": "https://api.costfluent.com/v1/providers?page=1&limit=10",
    "first": "https://api.costfluent.com/v1/providers?page=1&limit=10",
    "next": "https://api.costfluent.com/v1/providers?page=2&limit=10",
    "last": "https://api.costfluent.com/v1/providers?page=5&limit=10"
  },
  "data": [...]
}
```

Folders, segments, cost reports, anomalies and allocation rules are not paginated: their list
methods return the whole listing in its own envelope (`FolderList`, `[]Segment`,
`CostReportList`, `AnomalyList`, `[]AllocationRule`).

## Error Handling

```go
provider, err := client.GetProvider(ctx, "prv_xxx")
if err != nil {
    if costfluent.IsNotFound(err) {
        log.Println("Provider not found")
        return
    }
    if costfluent.IsRateLimited(err) {
        log.Println("Rate limited, back off")
        return
    }
    if costfluent.IsUnauthorized(err) {
        log.Println("Invalid API key")
        return
    }
    log.Fatal(err)
}
```

API errors are `*costfluent.APIError`, decoded from the response's problem details: `Detail`,
`Title`, `TraceID`, and the validation failures in `Errors`. `Message()` returns the detail
followed by each field error it does not already state, or the title when there is neither.

A 429 is retried for every method, POST included, because the API's rate limiter refuses a
request before any endpoint runs it. A 5xx or a connection error is retried for GET, PUT, DELETE
and every create, but not for a trigger, which would run twice if it reached the API before its
response was lost. A 404 that is not the API's `application/problem+json` comes from the proxy in
front of the API, which answers `404 page not found` while a container is recreated: it is retried
as a 5xx is, is `IsRetryable(err)` when it outlasts the retries, and is never `IsNotFound(err)`.
Every create sends a new `Idempotency-Key` and resends it on each retry, so the
API answers a retried create with the first response instead of creating twice;
`IsIdempotencyKeyReused(err)` is true when the API refuses a key already used with another request.
A replayed answer leaves out what a create answers only once, so a webhook's generated signing
secret is absent from it: `CreateApp` sets `App.Replayed` when the answer was a replay, and an
empty `SigningSecret` then means the secret exists but cannot be read; give the webhook a `secret`
of your own instead. The wait honours `Retry-After`, in seconds or as an HTTP date, or else the `RateLimit-Reset`
seconds the API sends beside it, and stops when the context is done. A wait over 60 seconds means
the token's quota is spent: the error is returned at once instead of waited out. The client's
timeout applies to each attempt.

Money is a `costfluent.Decimal`, the exact decimal string the API writes (`"1250.50000000"`), never a
`float64`: compare amounts with `Equal` or `Cmp`, which ignore the scale, and take the exact value
with `Rat`. `ParseDecimal` checks an amount before it is sent. Percentages stay `float64`.

A single read, create or update of an allocation rule, app, budget, business metric, cost alert,
cost report, cost report subscription, dashboard, widget, folder, provider, saved filter, savings
view, segment, team, virtual tag or workspace carries the resource's `ETag`; an allocation rule and a segment have
no single read, so only their create and update do. Passing it to an update, deletion, pause,
resume, activation or saved-filter change as
`costfluent.IfMatch(resource.ETag)` applies the write only while the resource is unchanged since;
otherwise the API answers 412, nothing changes, and `IsPreconditionFailed(err)` is true, so read it
again before retrying. A list carries no versions, and a write without `IfMatch` applies whatever
the version.

`QueryCostData` asks for `CostDataPageSize` (5,000) rows a page unless `Limit` says otherwise, and
paging reaches `CostDataMaxRows` (100,000) rows: a larger read splits its date window.

Error helper functions:
- `IsNotFound(err)` - 404 problem details from the API: the object does not exist or is not visible
- `IsConflict(err)` - 409
- `IsPreconditionFailed(err)` - 412, an `IfMatch` that is no longer current
- `IsIdempotencyKeyReused(err)` - 422, an `Idempotency-Key` already used with another request
- `IsRateLimited(err)` - 429
- `IsUnauthorized(err)` - 401
- `IsForbidden(err)` - 403
- `IsBadRequest(err)` - 400
- `IsServerError(err)` - 5xx
- `StatusCode(err)` - returns HTTP status code

## Available Methods

`costfluent/contract_test.go` checks every method below against the Public API contract.

### Providers
- `ListProviders`, `ListAllProviders`
- `GetProvider`, `CreateProvider`, `UpdateProvider`, `DeleteProvider`
- `TestProviderConnection`, `TriggerProviderSync`

### Custom Connections
- `SubmitCosts` writes a batch of `CostEntry` rows to a custom connection; the rows the API refuses
  come back in the result's `Errors`, not as an error.

### Workspaces
- `ListWorkspaces`, `ListAllWorkspaces`
- `GetWorkspace`, `CreateWorkspace`, `UpdateWorkspace`, `DeleteWorkspace`

### Workspace Providers
- `ListWorkspaceProviders`, `GetWorkspaceProvider`, `CreateWorkspaceProvider`,
  `UpdateWorkspaceProvider`, `DeleteWorkspaceProvider`
- `AccountIDs` narrows a workspace to chosen AWS accounts or Azure subscriptions of a connection;
  empty reads the whole connection. On update, nil leaves the accounts and an empty, non-nil slice
  reads the whole connection again.

### Folders
- `ListFolders`, `GetFolder`, `CreateFolder`, `UpdateFolder`, `DeleteFolder`

### Budgets
- `ListBudgets`, `ListAllBudgets`
- `GetBudget`, `CreateBudget`, `UpdateBudget`, `DeleteBudget`

### Cost Alerts
- `ListCostAlerts`, `ListAllCostAlerts`
- `GetCostAlert`, `CreateCostAlert`, `UpdateCostAlert`, `DeleteCostAlert`
- `PauseCostAlert`, `ResumeCostAlert`

### Cost Reports
- `ListCostReports`, `GetCostReport`, `CreateCostReport`, `UpdateCostReport`, `DeleteCostReport`

### Annotations
- `ListAnnotations`, `ListAllAnnotations`
- `GetAnnotation`, `CreateAnnotation`, `UpdateAnnotation`, `DeleteAnnotation`
- `ResolveAnnotation`, `ReopenAnnotation`

### Dashboards
- `ListDashboards`, `ListAllDashboards`
- `GetDashboard`, `CreateDashboard`, `UpdateDashboard`, `DeleteDashboard`

### Allocation
- `ListSegments`, `CreateSegment`, `UpdateSegment`, `DeleteSegment`
- `ListAllocationRules`, `CreateAllocationRule`, `UpdateAllocationRule`, `ReorderAllocationRules`,
  `DeleteAllocationRule`, `GetAllocationCoverage`

### Business Metrics
- `ListBusinessMetrics`, `ListAllBusinessMetrics`
- `GetBusinessMetric`, `CreateBusinessMetric`, `UpdateBusinessMetric`, `DeleteBusinessMetric`
- `ImportBusinessMetricValues`, `ListBusinessMetricValues`

### Saved Filters
- `ListSavedFilters`, `ListAllSavedFilters`
- `GetSavedFilter`, `CreateSavedFilter`, `UpdateSavedFilter`, `DeleteSavedFilter`

### Virtual Tags
- `ListVirtualTags`, `ListAllVirtualTags`
- `GetVirtualTag`, `CreateVirtualTag`, `UpdateVirtualTag`, `DeleteVirtualTag`
- `ActivateVirtualTag`, `DeactivateVirtualTag`

### Anomalies
- `ListAnomalies`, `AcknowledgeAnomaly`

### Savings
- `ListSavingsItems`, `ListAllSavingsItems`, `GetSavingsItem` (read only; a finding is answered in
  the console)
- `ListSavingsViews`, `ListAllSavingsViews`
- `GetSavingsView`, `CreateSavingsView`, `UpdateSavingsView`, `DeleteSavingsView`

### Commitments
- `ListCommitments`, `ListAllCommitments`
- `GetCommitmentCoverage`

### Kubernetes
- `GetKubernetesEfficiency`

### Cost Data
- `QueryCostData`, `GetCostSummary`

### Exchange Rates and Billing
- `ListExchangeRates`
- `GetEntitlement`, `GetUsage`

## Versions

Releases are `v1.MINOR.PATCH`, numbered with the Costfluent Python and TypeScript clients. The
major is the API's path version, `v1`, which only grows; the minor rises with each release that
carries a newer API contract, the patch is a fix to the client alone. The package's `ContractSHA256`
is the SHA-256 of the API document a release was built and tested against, the copy in
`costfluent/testdata/public.v1.json`.

## Development

```bash
# Run tests, including the contract test against costfluent/testdata/public.v1.json
make test

# Record ContractSHA256 after replacing the contract copy
make generate OPENAPI_SPEC=path/to/public.v1.json

# Lint
make lint

# Format
make fmt
```

## License

Apache-2.0, see `LICENSE`.
