package assert_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

type fakeTB struct {
	helper int
	failed bool
	msg    string
}

func (f *fakeTB) Helper() { f.helper++ }

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
}

func TestEqualPass(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.Equal(fake, 1, 1)
	assert.Equal(fake, "ok", "ok")
	if fake.failed {
		t.Fatalf("Equal failed: %s", fake.msg)
	}
	if fake.helper == 0 {
		t.Fatal("Equal did not call Helper")
	}
}

func TestEqualFail(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.Equal(fake, 1, 2)
	if fake.msg != "got 1, want 2" {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestEqualFailString(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.Equal(fake, "got", "want")
	if fake.msg != `got "got", want "want"` {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestDeepEqual(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.DeepEqual(fake, []int{1, 2}, []int{1, 2})
	if fake.failed {
		t.Fatalf("DeepEqual failed: %s", fake.msg)
	}

	assert.DeepEqual(fake, []int{1}, []int{2})
	if fake.msg != "got []int{1}, want []int{2}" {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestNoError(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.NoError(fake, nil)
	if fake.failed {
		t.Fatalf("NoError failed: %s", fake.msg)
	}

	assert.NoError(fake, errors.New("boom"))
	if fake.msg != "unexpected error: boom" {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.Error(fake, errors.New("boom"))
	if fake.failed {
		t.Fatalf("Error failed: %s", fake.msg)
	}

	assert.Error(fake, nil)
	if fake.msg != "expected an error" {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestErrorIs(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	wrapped := fmt.Errorf("page: %w", sentinel)

	fake := &fakeTB{}
	assert.ErrorIs(fake, wrapped, sentinel)
	if fake.failed {
		t.Fatalf("ErrorIs failed: %s", fake.msg)
	}

	other := errors.New("other")
	assert.ErrorIs(fake, wrapped, other)
	if fake.msg != `got "page: boom", want errors.Is "other"` {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestNil(t *testing.T) {
	t.Parallel()

	var p *int
	fake := &fakeTB{}
	assert.Nil(fake, nil)
	assert.Nil(fake, p)
	if fake.failed {
		t.Fatalf("Nil failed: %s", fake.msg)
	}

	n := 1
	assert.Nil(fake, &n)
	if fake.msg != "got (*int)(0x0), want nil" && !hasPrefix(fake.msg, "got (*int)(") {
		t.Fatalf("msg = %q", fake.msg)
	}
	if !hasSuffix(fake.msg, ", want nil") {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestNotNil(t *testing.T) {
	t.Parallel()

	n := 1
	fake := &fakeTB{}
	assert.NotNil(fake, &n)
	if fake.failed {
		t.Fatalf("NotNil failed: %s", fake.msg)
	}

	var p *int
	assert.NotNil(fake, p)
	if fake.msg != "got (*int)(nil), want non-nil" {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func TestTrueFalse(t *testing.T) {
	t.Parallel()

	fake := &fakeTB{}
	assert.True(fake, true)
	assert.False(fake, false)
	if fake.failed {
		t.Fatalf("bool assert failed: %s", fake.msg)
	}

	assert.True(fake, false)
	if fake.msg != "got false, want true" {
		t.Fatalf("msg = %q", fake.msg)
	}

	fake.failed = false
	assert.False(fake, true)
	if fake.msg != "got true, want false" {
		t.Fatalf("msg = %q", fake.msg)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
