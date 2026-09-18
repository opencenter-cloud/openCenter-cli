// Package magnum provides a small, standalone client for Magnum cluster
// lifecycle operations.
//
// Gophercloud v1 lifecycle operations do not accept contexts. Provider checks
// cancellation before and after each operation. Each real request group
// authenticates its own ProviderClient with its context already attached, so
// Gophercloud's reauthentication callback refreshes that same client and retry
// uses its refreshed token. Automatic reauthentication is bounded by the HTTP
// timeout.
package magnum
