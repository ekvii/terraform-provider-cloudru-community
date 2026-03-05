package provider

import (
	"fmt"
	"net/http"
	"strings"
)

// isNotFound reports whether err originates from an HTTP 404 response as
// surfaced by client.DoExpecting.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), fmt.Sprintf("unexpected HTTP status %d", http.StatusNotFound))
}
