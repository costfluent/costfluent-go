package costfluent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// contractOps is the operation each exported client method starts with. A method that creates and
// then reads, or walks pages, may send more requests; each of those must be a spec operation too.
var contractOps = map[string]struct{ Method, Path string }{
	"ListAllocationRules":    {"GET", "/v1/allocation/rules"},
	"CreateAllocationRule":   {"POST", "/v1/allocation/rules"},
	"UpdateAllocationRule":   {"PUT", "/v1/allocation/rules/{ruleId}"},
	"ReorderAllocationRules": {"POST", "/v1/allocation/rules/order"},
	"DeleteAllocationRule":   {"DELETE", "/v1/allocation/rules/{ruleId}"},
	"GetAllocationCoverage":  {"GET", "/v1/allocation/coverage"},

	"ListAnnotations":    {"GET", "/v1/annotations"},
	"ListAllAnnotations": {"GET", "/v1/annotations"},
	"GetAnnotation":      {"GET", "/v1/annotations/{annotationId}"},
	"CreateAnnotation":   {"POST", "/v1/annotations"},
	"UpdateAnnotation":   {"PUT", "/v1/annotations/{annotationId}"},
	"ResolveAnnotation":  {"POST", "/v1/annotations/{annotationId}/resolve"},
	"ReopenAnnotation":   {"POST", "/v1/annotations/{annotationId}/reopen"},
	"DeleteAnnotation":   {"DELETE", "/v1/annotations/{annotationId}"},

	"ListAnomalies":      {"GET", "/v1/anomalies"},
	"AcknowledgeAnomaly": {"POST", "/v1/anomalies/{anomalyId}/acknowledge"},

	"ListCommitments":       {"GET", "/v1/commitments"},
	"ListAllCommitments":    {"GET", "/v1/commitments"},
	"GetCommitmentCoverage": {"GET", "/v1/commitments/coverage"},

	"GetKubernetesEfficiency": {"GET", "/v1/kubernetes/efficiency"},

	"ListSavingsItems":    {"GET", "/v1/savings-items"},
	"ListAllSavingsItems": {"GET", "/v1/savings-items"},
	"GetSavingsItem":      {"GET", "/v1/savings-items/{savingsItemId}"},

	"ListSavingsViews":    {"GET", "/v1/savings-views"},
	"ListAllSavingsViews": {"GET", "/v1/savings-views"},
	"GetSavingsView":      {"GET", "/v1/savings-views/{savingsViewId}"},
	"CreateSavingsView":   {"POST", "/v1/savings-views"},
	"UpdateSavingsView":   {"PUT", "/v1/savings-views/{savingsViewId}"},
	"DeleteSavingsView":   {"DELETE", "/v1/savings-views/{savingsViewId}"},

	"GetEntitlement": {"GET", "/v1/entitlement"},
	"GetUsage":       {"GET", "/v1/usage"},

	"GetMe": {"GET", "/v1/me"},

	"ListAuditLogs":    {"GET", "/v1/audit-logs"},
	"ListAllAuditLogs": {"GET", "/v1/audit-logs"},

	"ListBusinessMetrics":        {"GET", "/v1/business-metrics"},
	"ListAllBusinessMetrics":     {"GET", "/v1/business-metrics"},
	"GetBusinessMetric":          {"GET", "/v1/business-metrics/{metricId}"},
	"CreateBusinessMetric":       {"POST", "/v1/business-metrics"},
	"UpdateBusinessMetric":       {"PUT", "/v1/business-metrics/{metricId}"},
	"DeleteBusinessMetric":       {"DELETE", "/v1/business-metrics/{metricId}"},
	"ImportBusinessMetricValues": {"POST", "/v1/business-metrics/{metricId}/values"},
	"ListBusinessMetricValues":   {"GET", "/v1/business-metrics/{metricId}/values"},

	"ListBudgets":    {"GET", "/v1/budgets"},
	"ListAllBudgets": {"GET", "/v1/budgets"},
	"GetBudget":      {"GET", "/v1/budgets/{budgetId}"},
	"CreateBudget":   {"POST", "/v1/budgets"},
	"UpdateBudget":   {"PUT", "/v1/budgets/{budgetId}"},
	"DeleteBudget":   {"DELETE", "/v1/budgets/{budgetId}"},

	"ListCostAlerts":    {"GET", "/v1/cost-alerts"},
	"ListAllCostAlerts": {"GET", "/v1/cost-alerts"},
	"GetCostAlert":      {"GET", "/v1/cost-alerts/{alertId}"},
	"CreateCostAlert":   {"POST", "/v1/cost-alerts"},
	"UpdateCostAlert":   {"PUT", "/v1/cost-alerts/{alertId}"},
	"DeleteCostAlert":   {"DELETE", "/v1/cost-alerts/{alertId}"},
	"PauseCostAlert":    {"POST", "/v1/cost-alerts/{alertId}/pause"},
	"ResumeCostAlert":   {"POST", "/v1/cost-alerts/{alertId}/resume"},

	"QueryCostData":  {"GET", "/v1/costs"},
	"GetCostSummary": {"GET", "/v1/costs/summary"},

	"ListCostReports":   {"GET", "/v1/cost-reports"},
	"GetCostReport":     {"GET", "/v1/cost-reports/{costReportId}"},
	"GetCostReportData": {"GET", "/v1/cost-reports/{costReportId}/data"},
	"CreateCostReport":  {"POST", "/v1/cost-reports"},
	"UpdateCostReport":  {"PUT", "/v1/cost-reports/{costReportId}"},
	"DeleteCostReport":  {"DELETE", "/v1/cost-reports/{costReportId}"},

	"ListDashboards":    {"GET", "/v1/dashboards"},
	"ListAllDashboards": {"GET", "/v1/dashboards"},
	"GetDashboard":      {"GET", "/v1/dashboards/{dashboardId}"},
	"CreateDashboard":   {"POST", "/v1/dashboards"},
	"UpdateDashboard":   {"PUT", "/v1/dashboards/{dashboardId}"},
	"DeleteDashboard":   {"DELETE", "/v1/dashboards/{dashboardId}"},

	"SetDashboardSavedFilter": {"PUT", "/v1/dashboards/{dashboardId}/saved-filter"},
	"SetDashboardDefault":     {"PUT", "/v1/dashboards/{dashboardId}/default"},
	"GetDashboardWidget":      {"GET", "/v1/dashboards/{dashboardId}/widgets/{widgetId}"},
	"CreateDashboardWidget":   {"POST", "/v1/dashboards/{dashboardId}/widgets"},
	"UpdateDashboardWidget":   {"PUT", "/v1/dashboards/{dashboardId}/widgets/{widgetId}"},
	"DeleteDashboardWidget":   {"DELETE", "/v1/dashboards/{dashboardId}/widgets/{widgetId}"},

	"ListExchangeRates": {"GET", "/v1/exchange-rates"},

	"ListFolders":          {"GET", "/v1/folders"},
	"GetFolder":            {"GET", "/v1/folders/{folderId}"},
	"CreateFolder":         {"POST", "/v1/folders"},
	"UpdateFolder":         {"PUT", "/v1/folders/{folderId}"},
	"SetFolderSavedFilter": {"PUT", "/v1/folders/{folderId}/saved-filter"},
	"DeleteFolder":         {"DELETE", "/v1/folders/{folderId}"},

	"ListProviders":              {"GET", "/v1/providers"},
	"ListAllProviders":           {"GET", "/v1/providers"},
	"GetProvider":                {"GET", "/v1/providers/{providerId}"},
	"CreateProvider":             {"POST", "/v1/providers"},
	"UpdateProvider":             {"PUT", "/v1/providers/{providerId}"},
	"DeleteProvider":             {"DELETE", "/v1/providers/{providerId}"},
	"TestProviderConnection":     {"POST", "/v1/providers/{providerId}/test"},
	"TriggerProviderSync":        {"POST", "/v1/providers/{providerId}/sync"},
	"ProvisionGcpServiceAccount": {"POST", "/v1/providers/gcp/service-account"},
	"GetAwsConnector":            {"GET", "/v1/providers/aws/connector"},

	"TranslateCostFilter": {"GET", "/v1/cost-filters/translate"},

	"ListSavedFilters":    {"GET", "/v1/saved-filters"},
	"ListAllSavedFilters": {"GET", "/v1/saved-filters"},
	"GetSavedFilter":      {"GET", "/v1/saved-filters/{filterId}"},
	"CreateSavedFilter":   {"POST", "/v1/saved-filters"},
	"UpdateSavedFilter":   {"PUT", "/v1/saved-filters/{filterId}"},
	"DeleteSavedFilter":   {"DELETE", "/v1/saved-filters/{filterId}"},

	"ListSegments":  {"GET", "/v1/segments"},
	"CreateSegment": {"POST", "/v1/segments"},
	"UpdateSegment": {"PUT", "/v1/segments/{segmentId}"},
	"DeleteSegment": {"DELETE", "/v1/segments/{segmentId}"},

	"ListVirtualTags":    {"GET", "/v1/virtual-tags"},
	"ListAllVirtualTags": {"GET", "/v1/virtual-tags"},
	"GetVirtualTag":      {"GET", "/v1/virtual-tags/{virtualTagId}"},
	"CreateVirtualTag":   {"POST", "/v1/virtual-tags"},
	"UpdateVirtualTag":   {"PATCH", "/v1/virtual-tags/{virtualTagId}"},
	"DeleteVirtualTag":   {"DELETE", "/v1/virtual-tags/{virtualTagId}"},

	"GetTagPolicy":    {"GET", "/v1/workspaces/{workspaceId}/tag-policy"},
	"UpdateTagPolicy": {"PUT", "/v1/workspaces/{workspaceId}/tag-policy"},

	"ListWorkspaceProviders":  {"GET", "/v1/workspace-providers"},
	"GetWorkspaceProvider":    {"GET", "/v1/workspace-providers/{workspaceProviderId}"},
	"CreateWorkspaceProvider": {"POST", "/v1/workspace-providers"},
	"UpdateWorkspaceProvider": {"PUT", "/v1/workspace-providers/{workspaceProviderId}"},
	"DeleteWorkspaceProvider": {"DELETE", "/v1/workspace-providers/{workspaceProviderId}"},

	"ListApps":    {"GET", "/v1/apps"},
	"ListAllApps": {"GET", "/v1/apps"},
	"GetApp":      {"GET", "/v1/apps/{appId}"},
	"CreateApp":   {"POST", "/v1/apps"},
	"UpdateApp":   {"PUT", "/v1/apps/{appId}"},
	"DeleteApp":   {"DELETE", "/v1/apps/{appId}"},

	"ListCostReportSubscriptions":      {"GET", "/v1/cost-report-subscriptions"},
	"GetCostReportSubscription":        {"GET", "/v1/cost-report-subscriptions/{subscriptionId}"},
	"CreateCostReportSubscription":     {"POST", "/v1/cost-report-subscriptions"},
	"UpdateCostReportSubscription":     {"PATCH", "/v1/cost-report-subscriptions/{subscriptionId}"},
	"DeleteCostReportSubscription":     {"DELETE", "/v1/cost-report-subscriptions/{subscriptionId}"},
	"ActivateCostReportSubscription":   {"POST", "/v1/cost-report-subscriptions/{subscriptionId}/activate"},
	"DeactivateCostReportSubscription": {"POST", "/v1/cost-report-subscriptions/{subscriptionId}/deactivate"},

	"ListTeams":                  {"GET", "/v1/teams"},
	"ListAllTeams":               {"GET", "/v1/teams"},
	"GetTeam":                    {"GET", "/v1/teams/{teamId}"},
	"CreateTeam":                 {"POST", "/v1/teams"},
	"UpdateTeam":                 {"PUT", "/v1/teams/{teamId}"},
	"DeleteTeam":                 {"DELETE", "/v1/teams/{teamId}"},
	"AddTeamMember":              {"POST", "/v1/teams/{teamId}/members"},
	"UpdateTeamMember":           {"PUT", "/v1/teams/{teamId}/members/{userId}"},
	"RemoveTeamMember":           {"DELETE", "/v1/teams/{teamId}/members/{userId}"},
	"ListTeamWorkspaceAccess":    {"GET", "/v1/teams/{teamId}/workspaces"},
	"ListAllTeamWorkspaceAccess": {"GET", "/v1/teams/{teamId}/workspaces"},
	"SetTeamWorkspaceAccess":     {"PUT", "/v1/teams/{teamId}/workspaces/{workspaceId}"},
	"SetTeamDefaultDashboard":    {"PUT", "/v1/teams/{teamId}/workspaces/{workspaceId}/default-dashboard"},
	"RemoveTeamWorkspaceAccess":  {"DELETE", "/v1/teams/{teamId}/workspaces/{workspaceId}"},

	"SubmitCosts": {"POST", "/v1/custom-providers/{providerId}/costs"},

	"ListUsers":    {"GET", "/v1/users"},
	"ListAllUsers": {"GET", "/v1/users"},
	"GetUser":      {"GET", "/v1/users/{userId}"},
	"InviteUser":   {"POST", "/v1/users"},

	"ListWorkspaces":    {"GET", "/v1/workspaces"},
	"ListAllWorkspaces": {"GET", "/v1/workspaces"},
	"GetWorkspace":      {"GET", "/v1/workspaces/{workspaceId}"},
	"CreateWorkspace":   {"POST", "/v1/workspaces"},
	"UpdateWorkspace":   {"PUT", "/v1/workspaces/{workspaceId}"},
	"DeleteWorkspace":   {"DELETE", "/v1/workspaces/{workspaceId}"},
}

// notOperations are the exported client methods that send nothing.
var notOperations = map[string]bool{"Workspace": true, "BaseURL": true, "ResolveWorkspace": true}

type specSchema struct {
	Ref        string                 `json:"$ref"`
	Type       json.RawMessage        `json:"type"`
	Format     string                 `json:"format"`
	Properties map[string]*specSchema `json:"properties"`
	Required   []string               `json:"required"`
	Items      *specSchema            `json:"items"`
	AllOf      []*specSchema          `json:"allOf"`
	OneOf      []*specSchema          `json:"oneOf"`
	Extensible []string               `json:"x-extensible-enum"`
	EnumNames  []string               `json:"x-enumNames"`
	Enum       []string               `json:"enum"`
}

type specContent map[string]struct {
	Schema *specSchema `json:"schema"`
}

type specOperation struct {
	Parameters []struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required bool   `json:"required"`
	} `json:"parameters"`
	RequestBody *struct {
		Content specContent `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Content specContent `json:"content"`
	} `json:"responses"`
}

type spec struct {
	Paths      map[string]map[string]*specOperation `json:"paths"`
	Components struct {
		Schemas map[string]*specSchema `json:"schemas"`
	} `json:"components"`

	routes  []route
	checked map[checkedPair]bool
}

// checkedPair stops checkFields on a recursive type such as a folder's children.
type checkedPair struct {
	typ    reflect.Type
	schema *specSchema
}

type route struct {
	method, template string
	pattern          *regexp.Regexp
	op               *specOperation
}

var templateParam = regexp.MustCompile(`\{[^}]+\}`)

// The constant names the document the contract test holds the client to, so a release says which
// contract it was built from; make generate rewrites both together.
func TestContractSHA256IsTheContractsDigest(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "public.v1.json"))
	if err != nil {
		t.Fatalf("reading the Public API spec: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != ContractSHA256 {
		t.Errorf("ContractSHA256 = %s, but testdata/public.v1.json hashes to %s; run make generate", ContractSHA256, got)
	}
}

func loadSpec(t *testing.T) *spec {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "public.v1.json"))
	if err != nil {
		t.Fatalf("reading the Public API spec: %v", err)
	}
	s := spec{checked: map[checkedPair]bool{}}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("decoding the Public API spec: %v", err)
	}
	for template, ops := range s.Paths {
		parts := templateParam.Split(template, -1)
		for i, part := range parts {
			parts[i] = regexp.QuoteMeta(part)
		}
		pattern := regexp.MustCompile("^" + strings.Join(parts, `[^/]+`) + "$")
		for method, op := range ops {
			s.routes = append(s.routes, route{strings.ToUpper(method), template, pattern, op})
		}
	}
	return &s
}

func (s *spec) match(method, path string) *route {
	for i := range s.routes {
		r := &s.routes[i]
		if r.method == method && r.pattern.MatchString(path) {
			return r
		}
	}
	return nil
}

// resolve follows $ref and single-member allOf/oneOf wrappers, merging allOf properties.
func (s *spec) resolve(sc *specSchema) *specSchema {
	for sc != nil {
		switch {
		case sc.Ref != "":
			sc = s.Components.Schemas[strings.TrimPrefix(sc.Ref, "#/components/schemas/")]
		case len(sc.OneOf) == 1:
			sc = sc.OneOf[0]
		case len(sc.AllOf) > 0:
			merged := &specSchema{Type: sc.Type, Format: sc.Format, Items: sc.Items, Properties: map[string]*specSchema{}}
			for _, part := range append(sc.AllOf, &specSchema{Properties: sc.Properties}) {
				part = s.resolve(part)
				for k, v := range part.Properties {
					merged.Properties[k] = v
				}
				merged.Required = append(merged.Required, part.Required...)
				// A nullable or defaulted wrapper such as allOf: [$ref: SomeEnum] has no type of its
				// own; without the part's, its field's type would go unchecked.
				if len(merged.Type) == 0 {
					merged.Type, merged.Format, merged.Items = part.Type, part.Format, part.Items
				}
			}
			return merged
		default:
			return sc
		}
	}
	return nil
}

// schemaTypes reads type as a set: a 3.0 document writes one name beside nullable, a 3.1 document
// a list such as ["integer","string"] or ["string","null"]. null is left out, since a nil pointer
// or an omitted field carries it.
func schemaTypes(sc *specSchema) map[string]bool {
	set := map[string]bool{}
	var one string
	var many []string
	if json.Unmarshal(sc.Type, &one) == nil {
		set[one] = true
	} else if json.Unmarshal(sc.Type, &many) == nil {
		for _, name := range many {
			set[name] = true
		}
	}
	delete(set, "null")
	return set
}

func isArray(sc *specSchema) bool { return sc != nil && schemaTypes(sc)["array"] }

func jsonSchemaOf(c specContent) *specSchema {
	if c == nil {
		return nil
	}
	return c["application/json"].Schema
}

func successResponse(op *specOperation) (status string, schema *specSchema) {
	for code, r := range op.Responses {
		if strings.HasPrefix(code, "2") {
			return code, jsonSchemaOf(r.Content)
		}
	}
	return "", nil
}

var timeType = reflect.TypeOf(time.Time{})

// reporter is the part of testing.T the checker uses, so a test can collect what it reports.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// wrongKind says why values of typ cannot be the schema's, or returns "" when they can or the
// schema names no type.
func wrongKind(typ reflect.Type, sc *specSchema) string {
	types := schemaTypes(sc)
	if len(types) == 0 {
		return ""
	}
	if typ == timeType {
		if types["string"] && sc.Format == "date-time" {
			return ""
		}
		return fmt.Sprintf("%s decodes only a date-time string, the schema is type %s format %q", typ, sc.Type, sc.Format)
	}
	var want string
	switch typ.Kind() {
	case reflect.String:
		want = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		want = "integer"
	case reflect.Float32, reflect.Float64:
		want = "number"
	case reflect.Bool:
		want = "boolean"
	case reflect.Struct, reflect.Map:
		want = "object"
	default:
		return ""
	}
	if types[want] {
		return ""
	}
	return fmt.Sprintf("%s is a json %s, the schema is type %s format %q", typ, want, sc.Type, sc.Format)
}

// checkFields fails for every json field of typ, recursively, that the schema does not declare or
// whose Go type cannot hold the schema's values.
func (s *spec) checkFields(r reporter, where string, typ reflect.Type, sc *specSchema) {
	r.Helper()
	sc = s.resolve(sc)
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		if typ.Kind() == reflect.Slice {
			if !isArray(sc) {
				r.Errorf("%s: %s is a list, the schema is not", where, typ)
				return
			}
			sc = s.resolve(sc.Items)
		}
		typ = typ.Elem()
	}
	if sc == nil {
		return
	}
	if msg := wrongKind(typ, sc); msg != "" {
		r.Errorf("%s: %s", where, msg)
		return
	}
	if typ.Kind() != reflect.Struct || typ == timeType {
		return
	}
	pair := checkedPair{typ, sc}
	if s.checked[pair] {
		return
	}
	s.checked[pair] = true
	s.checkStruct(r, where, typ, sc)
}

func (s *spec) checkStruct(r reporter, where string, typ reflect.Type, sc *specSchema) {
	r.Helper()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name == "" {
			s.checkStruct(r, where, f.Type, sc)
			continue
		}
		if !f.IsExported() || name == "-" || name == "" {
			continue
		}
		prop, ok := sc.Properties[name]
		if !ok {
			r.Errorf("%s: %s.%s is json %q, which the schema does not declare", where, typ.Name(), f.Name, name)
			continue
		}
		s.checkFields(r, where+"."+name, f.Type, prop)
	}
}

type recorded struct {
	method, path string
	query        map[string][]string
	body         []byte
}

// recorder answers every request with the smallest body its spec operation can decode into, and
// keeps what it received.
type recorder struct {
	spec *spec
	mu   sync.Mutex
	got  []recorded
}

func (rec *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec.mu.Lock()
	rec.got = append(rec.got, recorded{r.Method, r.URL.Path, r.URL.Query(), body})
	rec.mu.Unlock()

	rt := rec.spec.match(r.Method, r.URL.Path)
	if rt == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	status, schema := successResponse(rt.op)
	if status == "204" || schema == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if isArray(rec.spec.resolve(schema)) {
		_, _ = w.Write([]byte(`[]`))
		return
	}
	_, _ = w.Write([]byte(`{"id":"id-1"}`))
}

// fixedTime is the instant a filled input carries wherever it takes a time.
var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// filled returns a typ with every exported field set: "1" for a string, so a Decimal holds a valid
// amount too, true, 1, fixedTime, and one element in each slice and map. depth stops a recursive
// type such as a folder's children.
func filled(typ reflect.Type, depth int) reflect.Value {
	v := reflect.New(typ).Elem()
	if depth == 0 {
		return v
	}
	if typ == timeType {
		v.Set(reflect.ValueOf(fixedTime))
		return v
	}
	switch typ.Kind() {
	case reflect.String:
		v.SetString("1")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Pointer:
		p := reflect.New(typ.Elem())
		p.Elem().Set(filled(typ.Elem(), depth-1))
		v.Set(p)
	case reflect.Slice:
		v.Set(reflect.Append(v, filled(typ.Elem(), depth-1)))
	case reflect.Map:
		m := reflect.MakeMap(typ)
		m.SetMapIndex(filled(typ.Key(), depth-1), filled(typ.Elem(), depth-1))
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).IsExported() {
				v.Field(i).Set(filled(typ.Field(i).Type, depth-1))
			}
		}
	}
	return v
}

// call invokes a client method with placeholder arguments: a string for every ID, and for every
// input a zero value, or a filled one when fill is set; no request options.
func call(ctx context.Context, m reflect.Value, fill bool) []reflect.Value {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	n := m.Type().NumIn()
	if m.Type().IsVariadic() {
		n--
	}
	args := make([]reflect.Value, n)
	for i := range args {
		in := m.Type().In(i)
		switch {
		case in == ctxType:
			args[i] = reflect.ValueOf(ctx)
		case in.Kind() == reflect.String:
			args[i] = reflect.ValueOf("arg-1").Convert(in)
		case fill:
			args[i] = filled(in, 4)
		case in.Kind() == reflect.Pointer && in.Elem().Kind() == reflect.Struct:
			args[i] = reflect.New(in.Elem())
		default:
			args[i] = reflect.Zero(in)
		}
	}
	return m.Call(args)
}

func contractMethods() []string {
	names := make([]string, 0, len(contractOps))
	for name := range contractOps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// exercise calls the client method name against a recorder of s, and fails unless the call succeeds
// and sends a request.
func exercise(t *testing.T, s *spec, name string, fill bool) (*recorder, reflect.Value) {
	t.Helper()
	rec := &recorder{spec: s}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	client := NewClient(WithBaseURL(srv.URL), WithAPIKey("key"), WithWorkspace("ws-default"))

	m := reflect.ValueOf(client).MethodByName(name)
	if !m.IsValid() {
		t.Fatalf("client has no method %s", name)
	}
	out := call(context.Background(), m, fill)
	if err, _ := out[len(out)-1].Interface().(error); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(rec.got) == 0 {
		t.Fatal("sent no request")
	}
	return rec, m
}

func TestClientMatchesPublicAPI(t *testing.T) {
	s := loadSpec(t)
	for _, name := range contractMethods() {
		want := contractOps[name]
		t.Run(name, func(t *testing.T) {
			rec, m := exercise(t, s, name, false)

			first := s.match(rec.got[0].method, rec.got[0].path)
			if first == nil || first.method != want.Method || first.template != want.Path {
				t.Fatalf("first request %s %s, want %s %s", rec.got[0].method, rec.got[0].path, want.Method, want.Path)
			}
			if _, ok := s.Paths[want.Path][strings.ToLower(want.Method)]; !ok {
				t.Fatalf("%s %s is not a Public API operation", want.Method, want.Path)
			}

			var last *route
			for _, got := range rec.got {
				last = s.match(got.method, got.path)
				if last == nil {
					t.Fatalf("%s %s is not a Public API operation", got.method, got.path)
				}
				s.checkRequest(t, last, got)
			}

			methodType := m.Type()
			if first.op.RequestBody != nil {
				body := jsonSchemaOf(first.op.RequestBody.Content)
				for i := 0; i < methodType.NumIn(); i++ {
					if in := methodType.In(i); in.Kind() == reflect.Pointer && in.Elem().Kind() == reflect.Struct {
						s.checkFields(t, "request "+in.Elem().Name(), in, body)
					}
				}
			}
			if methodType.NumOut() == 2 {
				_, schema := successResponse(last.op)
				resp := s.resolve(schema)
				ret := methodType.Out(0)
				// A ListAll method returns the pages' data rather than a page.
				if ret.Kind() == reflect.Slice && !isArray(resp) && resp != nil && resp.Properties["data"] != nil {
					resp = resp.Properties["data"]
				}
				s.checkFields(t, "response", ret, resp)
			}
		})
	}
}

// A zero input sends no optional query parameter, so a filter the client and the spec spell
// differently passes the pass above and is silently ignored by the API.
func TestClientOptionalParametersMatchPublicAPI(t *testing.T) {
	s := loadSpec(t)
	for _, name := range contractMethods() {
		t.Run(name, func(t *testing.T) {
			rec, _ := exercise(t, s, name, true)
			for _, got := range rec.got {
				rt := s.match(got.method, got.path)
				if rt == nil {
					t.Fatalf("%s %s is not a Public API operation", got.method, got.path)
				}
				s.checkRequest(t, rt, got)
			}
		})
	}
}

func (s *spec) checkRequest(t *testing.T, rt *route, got recorded) {
	t.Helper()
	where := got.method + " " + rt.template
	params := map[string]bool{}
	for _, p := range rt.op.Parameters {
		if p.In != "query" {
			continue
		}
		params[p.Name] = true
		if p.Name == "workspaceId" && p.Required && len(got.query["workspaceId"]) == 0 {
			t.Errorf("%s: missing required workspaceId query parameter", where)
		}
	}
	for key := range got.query {
		if !params[key] {
			t.Errorf("%s: query parameter %q is not declared", where, key)
		}
	}

	if rt.op.RequestBody == nil {
		if len(got.body) > 0 && string(got.body) != "null" {
			t.Errorf("%s: sent a body the operation does not take: %s", where, got.body)
		}
		return
	}
	body := s.resolve(jsonSchemaOf(rt.op.RequestBody.Content))
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(got.body, &sent); err != nil {
		t.Errorf("%s: body is not a JSON object: %s", where, got.body)
		return
	}
	for key := range sent {
		if _, ok := body.Properties[key]; !ok {
			t.Errorf("%s: body field %q is not declared", where, key)
		}
	}
	for _, key := range body.Required {
		if key == "workspaceId" && len(sent[key]) == 0 {
			t.Errorf("%s: body is missing required workspaceId", where)
		}
	}
}

// collected is a reporter that keeps what the checker reports instead of failing a test.
type collected []string

func (c *collected) Helper() {}

func (c *collected) Errorf(format string, args ...any) { *c = append(*c, fmt.Sprintf(format, args...)) }

func TestContractCheckerRefusesAWrongType(t *testing.T) {
	components := `{
		"Amount": {"type": "string", "format": "decimal"},
		"Status": {"type": "string", "enum": ["active", "paused"]}
	}`
	tests := []struct {
		name    string
		goType  reflect.Type
		schema  string
		refused bool
	}{
		{"int against a string", reflect.TypeOf(0), `{"type": "string"}`, true},
		{"float64 against a decimal string", reflect.TypeOf(0.0), `{"type": "string", "format": "decimal"}`, true},
		{"float64 against a referenced decimal", reflect.TypeOf(0.0), `{"$ref": "#/components/schemas/Amount"}`, true},
		{"time.Time against a date", timeType, `{"type": "string", "format": "date"}`, true},
		{"string against an integer", reflect.TypeOf(""), `{"type": "integer", "format": "int32"}`, true},
		{"Decimal against a number", reflect.TypeOf(Decimal("")), `{"type": "number", "format": "double"}`, true},
		{"bool against a string", reflect.TypeOf(false), `{"type": "string"}`, true},
		{"map against a string", reflect.TypeOf(map[string]string{}), `{"type": "string"}`, true},
		{"int against a nullable enum wrapper", reflect.TypeOf(0), `{"allOf": [{"$ref": "#/components/schemas/Status"}], "nullable": true}`, true},
		{"int against a nullable string list", reflect.TypeOf(new(int)), `{"type": ["string", "null"]}`, true},

		{"int against an integer or string union", reflect.TypeOf(0), `{"type": ["integer", "string"], "format": "int32"}`, false},
		{"string pointer against a nullable string list", reflect.TypeOf(new(string)), `{"type": ["string", "null"]}`, false},
		{"string pointer against a nullable string", reflect.TypeOf(new(string)), `{"type": "string", "nullable": true}`, false},
		{"Decimal against a referenced decimal", reflect.TypeOf(Decimal("")), `{"$ref": "#/components/schemas/Amount"}`, false},
		{"string against a nullable enum wrapper", reflect.TypeOf(""), `{"allOf": [{"$ref": "#/components/schemas/Status"}], "nullable": true}`, false},
		{"time.Time against a date-time", timeType, `{"type": "string", "format": "date-time"}`, false},
		{"float64 against a number", reflect.TypeOf(0.0), `{"type": "number", "format": "double"}`, false},
		{"map against an object", reflect.TypeOf(map[string]string{}), `{"type": "object"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &spec{checked: map[checkedPair]bool{}}
			if err := json.Unmarshal([]byte(components), &s.Components.Schemas); err != nil {
				t.Fatal(err)
			}
			prop := &specSchema{}
			if err := json.Unmarshal([]byte(tt.schema), prop); err != nil {
				t.Fatal(err)
			}
			typ := reflect.StructOf([]reflect.StructField{{Name: "Field", Type: tt.goType, Tag: `json:"field"`}})
			var errs collected
			s.checkFields(&errs, "response", typ, &specSchema{Type: json.RawMessage(`"object"`), Properties: map[string]*specSchema{"field": prop}})
			if refused := len(errs) > 0; refused != tt.refused {
				t.Errorf("refused = %v, want %v; reported %q", refused, tt.refused, errs)
			}
		})
	}
}

// TestContractCoversEveryMethod keeps contractOps complete: an exported method missing from it
// would escape the contract check.
func TestContractCoversEveryMethod(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// A scan that finds nothing passes every check below, so it must find at least the methods
	// contractOps already names.
	scanned := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			if recv, ok := star.X.(*ast.Ident); !ok || recv.Name != "Client" {
				continue
			}
			if _, listed := contractOps[fn.Name.Name]; listed {
				scanned++
			} else if !notOperations[fn.Name.Name] {
				t.Errorf("%s: (*Client).%s is not in contractOps", file, fn.Name.Name)
			}
		}
	}
	if scanned < len(contractOps) {
		t.Errorf("found %d (*Client) methods named in contractOps across %d Go files, want %d: the scan no longer sees the client", scanned, len(files), len(contractOps))
	}
}
