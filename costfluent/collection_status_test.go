package costfluent

import (
	"context"
	"testing"
)

const collectionStatusJSON = `{"providerId":"prov_1","name":"AWS","key":"aws","status":"active","cadence":"daily","period":"day",
	"hasProviderTotal":true,"lastImportedAt":"2026-11-05T06:00:00Z","dataThrough":"2026-11-04",
	"nextDelivery":{"startDate":"2026-11-05","endDate":"2026-11-05","dueOn":"2026-11-05"},
	"daysOverdue":1,"isLate":false,"awaitingFirstImport":false,"lastSyncStatus":"success"}`

func TestListCollectionStatusDecodesAPageAndSendsItsOptions(t *testing.T) {
	c, got := testClient(t, stub{200, `{"links":{"self":"https://api.test/v1/providers/collection-status?page=2"},"data":[` + collectionStatusJSON + `]}`})

	page, err := c.ListCollectionStatus(context.Background(), &PageOptions{Page: 2, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/providers/collection-status")
	wantQuery(t, (*got)[0], "page", "2")
	wantQuery(t, (*got)[0], "limit", "1")
	if len(page.Data) != 1 {
		t.Fatalf("got %d sources", len(page.Data))
	}
	s := page.Data[0]
	if s.ProviderID != "prov_1" || s.Key != ProviderKeyAws || s.Status != ProviderStatusActive || s.Cadence != "daily" ||
		s.Period != "day" || !s.HasProviderTotal || s.IsLate || s.DaysOverdue == nil || *s.DaysOverdue != 1 {
		t.Fatalf("decoded %+v", s)
	}
	if s.DataThrough == nil || *s.DataThrough != "2026-11-04" || s.LastImportedAt == nil || s.NextDelivery == nil ||
		s.NextDelivery.DueOn != "2026-11-05" || s.NextDelivery.StartDate != "2026-11-05" {
		t.Fatalf("decoded freshness %+v", s)
	}
}

func TestListCollectionStatusLeavesAnUploadWithoutANextDelivery(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"links":{},"data":[{"providerId":"prov_2","name":"Uploads","key":"custom","status":"active",
		"cadence":"onUpload","period":"day","hasProviderTotal":false,"lastImportedAt":null,"dataThrough":null,
		"nextDelivery":null,"daysOverdue":null,"isLate":false,"awaitingFirstImport":false,"lastSyncStatus":null}]}`})

	page, err := c.ListCollectionStatus(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	s := page.Data[0]
	if s.NextDelivery != nil || s.DaysOverdue != nil || s.DataThrough != nil || s.LastImportedAt != nil || s.Cadence != "onUpload" {
		t.Fatalf("decoded %+v", s)
	}
}

func TestListAllCollectionStatusWalksEveryPage(t *testing.T) {
	c, got := testClient(t,
		stub{200, `{"links":{"self":"https://api.test/v1/providers/collection-status?page=1","next":"https://api.test/v1/providers/collection-status?page=2"},"data":[` + collectionStatusJSON + `]}`},
		stub{200, `{"links":{"self":"https://api.test/v1/providers/collection-status?page=2"},"data":[` + collectionStatusJSON + `]}`},
	)

	all, err := c.ListAllCollectionStatus(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || len(*got) != 2 {
		t.Fatalf("got %d sources in %d requests", len(all), len(*got))
	}
}

func TestGetProviderCollectionStatusDecodesItsMonthsAndMissingDays(t *testing.T) {
	c, got := testClient(t, stub{200, `{"summary":` + collectionStatusJSON + `,"amountsWithheldReason":null,
		"months":[{"month":"2026-10-01","isImported":true,"recordCount":62,"lastImportedAt":"2026-11-01T02:00:00Z","onRequest":false,
			"restatement":{"lastCommittedAt":"2026-11-03T02:00:00Z","records":50},
			"amounts":[{"currency":"USD","amount":"125.50000000"}],
			"providerTotal":{"status":"agrees","reason":null,"providerAmount":"125.52000000","costfluentAmount":"125.50000000",
				"difference":"-0.02000000","tolerance":"1.00000000","currency":"USD","comparedAt":"2026-11-03T06:00:00Z"}}],
		"missing":[{"startDate":"2026-10-10","endDate":"2026-10-11","scopes":2}]}`})

	detail, err := c.GetProviderCollectionStatus(context.Background(), "prov 1")
	if err != nil {
		t.Fatal(err)
	}
	wantRequest(t, (*got)[0], "GET", "/v1/providers/prov 1/collection-status")
	if detail.Summary.ProviderID != "prov_1" || detail.AmountsWithheldReason != nil || len(detail.Months) != 1 || len(detail.Missing) != 1 {
		t.Fatalf("decoded %+v", detail)
	}
	m := detail.Months[0]
	if m.Month != "2026-10-01" || m.RecordCount != 62 || m.Restatement == nil || m.Restatement.Records != 50 ||
		len(m.Amounts) != 1 || m.Amounts[0].Amount != "125.50000000" || m.Amounts[0].Currency != "USD" {
		t.Fatalf("decoded month %+v", m)
	}
	total := m.ProviderTotal
	if total.Status != "agrees" || total.ProviderAmount == nil || *total.ProviderAmount != "125.52000000" ||
		total.Difference == nil || *total.Difference != "-0.02000000" || total.ComparedAt == nil {
		t.Fatalf("decoded provider total %+v", total)
	}
	if detail.Missing[0] != (MissingRange{StartDate: "2026-10-10", EndDate: "2026-10-11", Scopes: 2}) {
		t.Fatalf("decoded missing %+v", detail.Missing[0])
	}
}

func TestGetProviderCollectionStatusKeepsWithheldAndAbsentAmountsNil(t *testing.T) {
	c, _ := testClient(t, stub{200, `{"summary":` + collectionStatusJSON + `,"amountsWithheldReason":"Amounts are shown to members who manage connections.",
		"months":[{"month":"2026-10-01","isImported":true,"recordCount":3,"lastImportedAt":null,"onRequest":false,"restatement":null,
			"amounts":null,"providerTotal":{"status":"noProviderTotal","reason":"Costfluent reads no provider total for this source.",
				"providerAmount":null,"costfluentAmount":null,"difference":null,"tolerance":null,"currency":null,"comparedAt":null}}],
		"missing":[]}`})

	detail, err := c.GetProviderCollectionStatus(context.Background(), "prov_1")
	if err != nil {
		t.Fatal(err)
	}
	m := detail.Months[0]
	if detail.AmountsWithheldReason == nil || m.Amounts != nil || m.Restatement != nil {
		t.Fatalf("decoded %+v", m)
	}
	if m.ProviderTotal.Status != "noProviderTotal" || m.ProviderTotal.Reason == nil || m.ProviderTotal.ProviderAmount != nil ||
		m.ProviderTotal.CostfluentAmount != nil || m.ProviderTotal.Currency != nil {
		t.Fatalf("decoded provider total %+v", m.ProviderTotal)
	}
}

func TestGetProviderCollectionStatusOfAnotherOrganizationsProviderIsNotFound(t *testing.T) {
	c, _ := testClient(t, stub{404, `{"status":404,"title":"Not Found","code":"not_found"}`})

	_, err := c.GetProviderCollectionStatus(context.Background(), "prov_foreign")
	if !IsNotFound(err) {
		t.Fatalf("got %v, want not found", err)
	}
}
