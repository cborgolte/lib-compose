package composition

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_StringFragment(t *testing.T) {
	a := assert.New(t)

	f := NewStringFragment("§[foo]§")
	f.AddStylesheets([]string{"/abc/def", "ghi/xyz"})
	a.Equal([]string{"/abc/def", "ghi/xyz"}, f.Stylesheets())
	buf := bytes.NewBufferString("")
	err := f.Execute(buf, map[string]interface{}{"foo": "bar"}, nil)
	a.NoError(err)

	a.Equal("bar", buf.String())
}
