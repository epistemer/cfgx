// Package validate holds Validate functions the schema refers to.
package validate

import (
	"fmt"
	"net/mail"
)

// Email checks that s is an email address.
func Email(s string) error {
	if _, err := mail.ParseAddress(s); err != nil {
		return fmt.Errorf("%q isn't an email address", s)
	}
	return nil
}
