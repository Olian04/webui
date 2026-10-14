package render

import (
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The mount prefix is a literal in the rules, though a URL pattern gives some characters a meaning.
func TestThePrefixIsALiteralInTheRules(t *testing.T) {
	t.Parallel()

	assert.Equal(t, patternEscape("/admin"), "/admin")
	assert.Equal(t, patternEscape("/a:b/(c)*"), `/a\:b/\(c\)\*`)

	rules, err := speculationRules("/ops")
	assert.NoError(t, err)
	assert.Contains(t, string(rules), `"href_matches":"/ops/*"`)
	rules, err = speculationRules("")
	assert.NoError(t, err)
	assert.Contains(t, string(rules), `"href_matches":"/*"`)
}
