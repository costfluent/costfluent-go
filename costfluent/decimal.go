package costfluent

import (
	"fmt"
	"math/big"
	"regexp"
)

// Decimal is an amount of money exactly as the API writes it: a decimal string such as
// "1234.50000000", never a binary float, so sums and comparisons do not drift. The API answers
// with eight decimals and accepts at most eight. The zero value "" means the API sent no amount.
type Decimal string

// DecimalScale is the number of decimals the API writes every amount with.
const DecimalScale = 8

var decimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]{1,8})?$`)

// ParseDecimal checks s is an amount the API accepts: digits, an optional sign and at most eight
// decimals, with no exponent or separators.
func ParseDecimal(s string) (Decimal, error) {
	if !decimalPattern.MatchString(s) {
		return "", fmt.Errorf("%q is not a decimal amount with at most %d decimals", s, DecimalScale)
	}
	return Decimal(s), nil
}

// IsSet reports whether the API sent an amount.
func (d Decimal) IsSet() bool { return d != "" }

// String returns the amount as written.
func (d Decimal) String() string { return string(d) }

// Rat returns the exact value of the amount.
func (d Decimal) Rat() (*big.Rat, error) {
	if _, err := ParseDecimal(string(d)); err != nil {
		return nil, err
	}
	// The pattern admits only what SetString parses.
	r, _ := new(big.Rat).SetString(string(d))
	return r, nil
}

// Cmp compares two amounts by value, so the scale does not matter: "49" and "49.00000000" are
// equal. It returns -1, 0 or +1, or an error when either is not a decimal amount.
func (d Decimal) Cmp(other Decimal) (int, error) {
	a, err := d.Rat()
	if err != nil {
		return 0, err
	}
	b, err := other.Rat()
	if err != nil {
		return 0, err
	}
	return a.Cmp(b), nil
}

// Equal reports whether two amounts have the same value; false when either is not an amount.
func (d Decimal) Equal(other Decimal) bool {
	c, err := d.Cmp(other)
	return err == nil && c == 0
}

// Canonical returns the amount as the API writes it, with exactly eight decimals.
func (d Decimal) Canonical() (Decimal, error) {
	r, err := d.Rat()
	if err != nil {
		return "", err
	}
	return Decimal(r.FloatString(DecimalScale)), nil
}
