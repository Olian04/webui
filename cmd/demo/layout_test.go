package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The weighted columns page has a row for each weighting it shows, and each column carries its weight.
func TestTheWeightedColumnsPageHasEachWeightingInItsOwnRow(t *testing.T) {
	rec := get(handler(t), "/admin/layout/weights")
	assert.Equal(t, rec.Code, http.StatusOK)
	body := rec.Body.String()

	assert.Equal(t, strings.Count(body, `<div class="columns">`), 4)
	assert.Equal(t, strings.Count(body, `<div class="col" style="--w:1;">`), 6) // 25/75 and the four equal ones
	assert.Equal(t, strings.Count(body, `<div class="col" style="--w:3;">`), 2)
	assert.Contains(t, body, `<div class="col" style="--w:50;">`)
	assert.Equal(t, strings.Count(body, `<div class="col" style="--w:25;">`), 2)
	assert.Contains(t, body, `class="nav-item active" href="/admin/layout/weights"`)
}
