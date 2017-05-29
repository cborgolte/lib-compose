package composition

import (
	"bytes"
	"io"
	"io/ioutil"
)

// StringFragment is a simple template based representation of a fragment.
type StringFragment struct {
	content     string
	stylesheets []string
	name        string
}

func NewStringFragment(c string) *StringFragment {
	return &StringFragment{
		content:     c,
		stylesheets: nil,
	}
}

func (f *StringFragment) Content() string {
	return f.content
}

func (f *StringFragment) Name() string {
	return f.name
}

func (f *StringFragment) Stylesheets() []string {
	return f.stylesheets
}

func (f *StringFragment) AddStylesheets(stylesheets []string) {
	f.stylesheets = append(f.stylesheets, stylesheets...)
}

func (f *StringFragment) SetName(name string) {
	f.name = name
}

func (f *StringFragment) Execute(w io.Writer, data map[string]interface{}, executeNestedFragment func(nestedFragmentName string) error) error {
	result := bytes.NewBuffer(make([]byte, 0, DefaultBufferSize))
	err := executeTemplate(result, f.Content(), data, executeNestedFragment)
	w.Write(result.Bytes())
	w.Write([]byte("\n"))
	ioutil.WriteFile("/tmp/exec-"+f.Name(), result.Bytes(), 0644)
	return err
}

// MemorySize return the estimated size in bytes, for this object in memory
func (f *StringFragment) MemorySize() int {
	return len(f.content)
}
