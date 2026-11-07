// Package storage is the internal seam between the engine and a settings store
// backend. Package store implements it with bbolt; the seam exists so that a
// shared backend can be added later without changing the public API.
package storage
