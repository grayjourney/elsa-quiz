// Package id generates short, collision-resistant, URL-safe identifiers.
package id

import (
	"crypto/rand"
	"encoding/base32"
)

const idBytes = 10

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// New returns a new unique, URL-safe identifier.
func New() string {
	b := make([]byte, idBytes)
	if _, err := rand.Read(b); err != nil {
		panic("id: cannot read from crypto/rand: " + err.Error())
	}
	return encoding.EncodeToString(b)
}
