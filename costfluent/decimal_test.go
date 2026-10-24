package costfluent

import (
	"encoding/json"
	"testing"
)

func TestParseDecimalAcceptsWhatTheAPIAccepts(t *testing.T) {
	for _, ok := range []string{"0", "49", "-3.5", "1234.12345678", "0.00000001"} {
		if _, err := ParseDecimal(ok); err != nil {
			t.Errorf("ParseDecimal(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1e3", "1,000", "1.123456789", "+1", ".5", "5.", "12.5 ", "NaN"} {
		if _, err := ParseDecimal(bad); err == nil {
			t.Errorf("ParseDecimal(%q) accepted an amount the API refuses", bad)
		}
	}
}

func TestDecimalComparesByValueWhateverTheScale(t *testing.T) {
	if !Decimal("49").Equal("49.00000000") || !Decimal("10.00").Equal("10") {
		t.Error("the same amount at another scale must be equal")
	}
	if Decimal("0.1").Equal("0.10000001") {
		t.Error("amounts a float would round together must stay apart")
	}
	if c, err := Decimal("-1").Cmp("0.5"); err != nil || c != -1 {
		t.Errorf("Cmp(-1, 0.5) = %d, %v", c, err)
	}
	if _, err := Decimal("1e3").Cmp("1"); err == nil {
		t.Error("Cmp must refuse a value that is not an amount")
	}
	if _, err := Decimal("1").Cmp("1e3"); err == nil {
		t.Error("Cmp must refuse a value that is not an amount on either side")
	}
	if Decimal("").Equal("") || Decimal("").IsSet() {
		t.Error("an absent amount is neither set nor equal to anything")
	}
}

func TestDecimalCanonicalIsTheAPIsEightDecimals(t *testing.T) {
	for in, want := range map[Decimal]Decimal{"49": "49.00000000", "-0.5": "-0.50000000", "1234.12345678": "1234.12345678"} {
		if got, err := in.Canonical(); err != nil || got != want {
			t.Errorf("Canonical(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := Decimal("1e3").Canonical(); err == nil {
		t.Error("Canonical must refuse a value that is not an amount")
	}
	if Decimal("12.50").String() != "12.50" {
		t.Error("String returns the amount as written")
	}
}

func TestDecimalTravelsAsAJSONString(t *testing.T) {
	var v struct {
		Amount  Decimal  `json:"amount"`
		Ceiling *Decimal `json:"ceiling"`
		Missing Decimal  `json:"missing"`
	}
	if err := json.Unmarshal([]byte(`{"amount":"0.12345678","ceiling":null,"missing":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.Amount != "0.12345678" || v.Ceiling != nil || v.Missing.IsSet() {
		t.Fatalf("decoded %+v", v)
	}
	if err := json.Unmarshal([]byte(`{"amount":0.1}`), &v); err == nil {
		t.Error("a JSON number must not decode into an amount")
	}
	out, _ := json.Marshal(struct {
		Amount Decimal `json:"amount"`
	}{"12.50"})
	if string(out) != `{"amount":"12.50"}` {
		t.Errorf("encoded %s", out)
	}
}
