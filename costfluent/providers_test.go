package costfluent

import (
	"context"
	"slices"
	"testing"
)

const providerJSON = `{"id":"prv_1","type":"cloud","key":"aws","displayName":"AWS prod","status":"active",
	"parentProviderId":"prv_0","externalId":"123456789012","lastSyncAt":"2026-09-21T10:00:00Z",
	"syncFrequencyMinutes":360,"createdAt":"2026-09-01T00:00:00Z"}`

func TestProviderDecodes(t *testing.T) {
	c, got := testClient(t, stub{200, providerJSON})

	p, err := c.GetProvider(context.Background(), "prv_1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/providers/prv_1")
	if p.ID != "prv_1" || p.DisplayName != "AWS prod" || *p.ParentProviderID != "prv_0" || *p.ExternalID != "123456789012" ||
		p.SyncFrequencyMinutes != 360 || p.LastSyncAt == nil {
		t.Fatalf("decoded %+v", p)
	}
}

func TestCreateProviderReturnsCreated(t *testing.T) {
	c, got := testClient(t, stub{200, `{"id":"prv_1","type":"cloud","key":"aws","displayName":"AWS prod"}`})

	created, err := c.CreateProvider(context.Background(), &CreateProviderInput{
		Key: "aws", DisplayName: "AWS prod", Credentials: map[string]string{"roleArn": "arn:aws:iam::1:role/r"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/providers")
	wantBody(t, (*got)[0], "key", "aws")
	wantBody(t, (*got)[0], "displayName", "AWS prod")
	if created.ID != "prv_1" || created.DisplayName != "AWS prod" {
		t.Fatalf("created %+v", created)
	}
}

func TestUpdateProviderIsPut(t *testing.T) {
	c, got := testClient(t, stub{200, providerJSON})

	minutes := 720
	ext := "123456789012"
	if _, err := c.UpdateProvider(context.Background(), "prv_1", &UpdateProviderInput{
		ExternalID: &ext, SyncFrequencyMinutes: &minutes,
	}); err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "PUT", "/v1/providers/prv_1")
	wantBody(t, (*got)[0], "syncFrequencyMinutes", float64(720))
	wantBody(t, (*got)[0], "externalId", ext)
}

func TestTriggerProviderSync(t *testing.T) {
	c, got := testClient(t, stub{200, `{"triggered":true,"message":"queued"}`})

	resp, err := c.TriggerProviderSync(context.Background(), "prv_1", true)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/providers/prv_1/sync")
	wantBody(t, (*got)[0], "fullSync", true)
	if !resp.Triggered || *resp.Message != "queued" {
		t.Fatalf("resp %+v", resp)
	}
}

func TestProvisionGcpServiceAccountIsPostWithoutBody(t *testing.T) {
	c, got := testClient(t, stub{200, `{"serviceAccountEmail":"cfp-abc@costfluent-prod-connect.iam.gserviceaccount.com","organizationId":"501164525917"}`})

	account, err := c.ProvisionGcpServiceAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "POST", "/v1/providers/gcp/service-account")
	if account.ServiceAccountEmail != "cfp-abc@costfluent-prod-connect.iam.gserviceaccount.com" ||
		account.OrganizationID == nil || *account.OrganizationID != "501164525917" || account.DirectoryCustomerID != nil {
		t.Fatalf("decoded %+v", account)
	}
}

func TestCreateProviderGcpAccessPendingCarriesItsCode(t *testing.T) {
	c, _ := testClient(t, stub{400, `{"title":"Bad Request","status":400,"detail":"not shared yet",
		"errors":[{"name":"credentials","reason":"not shared yet","code":"Provider.GcpAccessPending"}]}`})

	_, err := c.CreateProvider(context.Background(), &CreateProviderInput{Key: "gcp", Credentials: map[string]string{}})

	if !HasCode(err, CodeGcpAccessPending) {
		t.Fatalf("expected %s, got %v", CodeGcpAccessPending, err)
	}
	if HasCode(err, "Other.Code") {
		t.Fatal("matched a code the response did not carry")
	}
}

func TestProviderKeysMatchTheContract(t *testing.T) {
	wantContractValues(t, "ProviderKey", ProviderKeys())
}

func TestProviderStatusesMatchTheContract(t *testing.T) {
	wantContractValues(t, "ProviderStatus", ProviderStatuses())
}

// The capabilities are compared by their wire values: /v1/me sends them, and a client compares them.
func TestCapabilitiesMatchTheContract(t *testing.T) {
	wantContractValues(t, "ApiTokenCapability", Capabilities())
}

// contractOmission is a value the contract declares that the SDK leaves out on purpose, and why.
type contractOmission struct{ value, reason string }

func wantContractValues[T ~string](t *testing.T, schema string, values []T, omitted ...contractOmission) {
	t.Helper()
	declared := loadSpec(t).Components.Schemas[schema]
	if declared == nil {
		t.Fatalf("the Public API spec declares no %s", schema)
	}
	// An extensible enum declares its wire values. A closed one declares its wire values in enum
	// and its names in x-enumNames, which the API reads case-insensitively, so the SDK may spell a
	// closed enum either way, but one way throughout.
	wire, names := declared.Extensible, declared.Extensible
	if len(wire) == 0 {
		wire, names = declared.Enum, declared.EnumNames
		if len(wire) == 0 {
			wire = names
		}
		if len(names) == 0 {
			names = wire
		}
	}
	if len(names) != len(wire) {
		t.Fatalf("the contract's %s declares %d values and %d names", schema, len(wire), len(names))
	}
	left := map[int]bool{}
	for _, o := range omitted {
		i := slices.Index(wire, o.value)
		if i < 0 {
			t.Errorf("the contract's %s no longer declares %q, which the SDK leaves out because %s; remove the omission",
				schema, o.value, o.reason)
			continue
		}
		left[i] = true
	}
	var want, wantNames []string
	for i := range wire {
		if !left[i] {
			want, wantNames = append(want, wire[i]), append(wantNames, names[i])
		}
	}
	if len(values) != len(want) {
		t.Fatalf("SDK has %d %s values %v, the contract %d %v", len(values), schema, values, len(want), want)
	}
	// The SDK's values are held to whichever spelling they match more of, so a list that mixes the
	// two fails on the values spelled the other way.
	spelled := want
	if matching(values, wantNames) > matching(values, want) {
		spelled = wantNames
	}
	for i, value := range values {
		if string(value) != spelled[i] {
			t.Errorf("%s value %d is %q, the contract says %q", schema, i, value, spelled[i])
		}
	}
}

func matching[T ~string](values []T, want []string) int {
	n := 0
	for i, value := range values {
		if string(value) == want[i] {
			n++
		}
	}
	return n
}
