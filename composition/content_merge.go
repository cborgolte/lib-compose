package composition

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

const (
	LayoutFragmentName = "layout"
	FragmentSeparater  = "#"
	DefaultBufferSize  = 1024 * 100
)

// ContentMerge is a helper type for creation of a combined html document
// out of multiple Content pages.
type ContentMerge struct {
	MetaJSON  map[string]interface{}
	Head      []Fragment
	BodyAttrs []Fragment

	// Aggregator for the Body Fragments of the results.
	// Each fragment is insertes twice with full name and local name,
	// The full name only ends with a FragmentSeparater ('#'), if the local name is not empty
	// and the local name is always prefixed with FragmentSeparater ('#').
	Body map[string]Fragment

	// Aggregator for the Tail Fragments of the results.
	Tail     []Fragment
	Buffered bool

	// merge priorities for the content objects
	// no entry means priority == 0
	priorities map[Content]int

	// all linkTags contained in used fragments
	linkTags [][]html.Attribute
	// all script tags contained in used fragments
	scriptTags []ScriptFragment

	// strategy to prevent duplicacte <link rel="stylesheet"> tags
	linkTagDeduplicationStrategy DeduplicationStrategy
}

// NewContentMerge creates a new buffered ContentMerge
func NewContentMerge(metaJSON map[string]interface{}) *ContentMerge {
	cntx := &ContentMerge{
		MetaJSON:   metaJSON,
		Head:       make([]Fragment, 0, 0),
		BodyAttrs:  make([]Fragment, 0, 0),
		Body:       make(map[string]Fragment),
		Tail:       make([]Fragment, 0, 0),
		Buffered:   true,
		priorities: make(map[Content]int),
	}
	return cntx
}

func (cntx *ContentMerge) SetDeduplicationStrategy(strategy DeduplicationStrategy) {
	cntx.linkTagDeduplicationStrategy = strategy
}

// TODO: Rename method
func (cntx *ContentMerge) collectStylesheets(f Fragment) {
	cntx.linkTags = append(cntx.linkTags, f.LinkTags()...)
	cntx.scriptTags = append(cntx.scriptTags, f.ScriptTags()...)
}

func (cntx *ContentMerge) deduplicateLinkTags() {
	if cntx.linkTagDeduplicationStrategy != nil {
		cntx.linkTags = cntx.linkTagDeduplicationStrategy.Deduplicate(cntx.linkTags)
	}
}

func (cntx *ContentMerge) writeLinkTags(w io.Writer) {

	// first make sure, linkTags are deduplicated
	cntx.deduplicateLinkTags()

	for _, attrs := range cntx.linkTags {
		joinedAttr := joinAttrs(attrs)
		stylesheet := fmt.Sprintf("\n    <link %s>", joinedAttr)
		io.WriteString(w, stylesheet)
	}
}

func (cntx *ContentMerge) writeScriptTags(w io.Writer) {

	// first make sure, linkTags are deduplicated
	// cntx.deduplicateLinkTags()

	for _, scriptData := range cntx.scriptTags {
		joinedAttr := joinAttrs(scriptData.Attrs)
		stylesheet := fmt.Sprintf("\n      <script %s>%s</script>", joinedAttr, string(scriptData.Text))
		io.WriteString(w, stylesheet)
	}
}

func generateExecutionFunction(cntx *ContentMerge, w io.Writer) (executeFragment func(fragmentName string) error) {
	executeFragment = func(fragmentName string) error {
		f, exist := cntx.GetBodyFragmentByName(fragmentName)
		if !exist {
			missingFragmentString := generateMissingFragmentString(cntx.Body, fragmentName)
			return errors.New(missingFragmentString)
		}
		cntx.collectStylesheets(f)
		return f.Execute(w, cntx.MetaJSON, executeFragment)
	}
	return executeFragment
}

func (cntx *ContentMerge) GetHtml() ([]byte, error) {

	if len(cntx.priorities) > 0 {
		cntx.processMetaPriorityParsing()
	}

	// start header, but don't close it. We will add stylsheets later on
	header := bytes.NewBuffer(make([]byte, 0, DefaultBufferSize))
	io.WriteString(header, "<!DOCTYPE html>\n<html>\n  <head>\n    ")

	for _, f := range cntx.Head {
		cntx.collectStylesheets(f)
		executeFragment := generateExecutionFunction(cntx, header)
		if err := f.Execute(header, cntx.MetaJSON, executeFragment); err != nil {
			return nil, err
		}
	}

	// open body tag
	body := bytes.NewBuffer(make([]byte, 0, DefaultBufferSize))
	io.WriteString(body, "\n  <body")
	for _, f := range cntx.BodyAttrs {
		io.WriteString(body, " ")
		executeFragment := generateExecutionFunction(cntx, body)
		if err := f.Execute(body, cntx.MetaJSON, executeFragment); err != nil {
			return nil, err
		}
	}

	io.WriteString(body, ">\n    ")

	startFragmentName := ""
	if _, exist := cntx.GetBodyFragmentByName(LayoutFragmentName); exist {
		startFragmentName = LayoutFragmentName
	}

	// recursively process body fragments
	executeFragment := generateExecutionFunction(cntx, body)
	if err := executeFragment(startFragmentName); err != nil {
		return nil, err
	}

	// write inline scripts etc. to own buffer
	tailInline := bytes.NewBuffer(make([]byte, 0, DefaultBufferSize))
	for _, f := range cntx.Tail {
		cntx.collectStylesheets(f)
		if err := f.Execute(tailInline, cntx.MetaJSON, executeFragment); err != nil {
			return nil, err
		}
	}

	io.WriteString(tailInline, "\n  </body>\n</html>\n")

	// write the collected linkTags to the header and close it
	cntx.writeLinkTags(header)
	io.WriteString(header, "\n  </head>")

	// write the collected script tags in own buffer
	tailScripts := bytes.NewBuffer(make([]byte, 0, DefaultBufferSize))
	cntx.writeScriptTags(tailScripts)

	// return concatenated header, tails and body
	html := append(header.Bytes(),
		append(body.Bytes(),
			append(tailScripts.Bytes(), tailInline.Bytes()...)...)...)
	return html, nil
}

// GetBodyFragmentByName returns a fragment by ists name.
// If the name does not contain a FragmentSeparater ('#'), and no such fragment is found.
// also a lookup for '#name' is done, to check, if there is a local name matching.
// The bool return value indicates, if the fragment was found.
func (cntx *ContentMerge) GetBodyFragmentByName(name string) (Fragment, bool) {
	f, found := cntx.Body[name]

	// Normalize: e.g. main# -> main
	if !found && strings.HasSuffix(name, FragmentSeparater) {
		f, found = cntx.Body[name[0:len(name)-1]]
	}

	// search also for local fragment if nothing else found
	if !found && !strings.Contains(name, FragmentSeparater) {
		f, found = cntx.Body[FragmentSeparater+name]
	}

	return f, found
}

func (cntx *ContentMerge) AddContent(c Content, priority int) {
	cntx.addHead(c.Head())
	cntx.addBodyAttributes(c.BodyAttributes())
	cntx.addBody(c)
	cntx.addTail(c.Tail())
	if priority > 0 {
		cntx.priorities[c] = priority
	}
}

func (cntx *ContentMerge) addHead(f Fragment) {
	if f != nil {
		cntx.Head = append(cntx.Head, f)
	}
}

func (cntx *ContentMerge) addBodyAttributes(f Fragment) {
	if f != nil {
		cntx.BodyAttrs = append(cntx.BodyAttrs, f)
	}
}

func (cntx *ContentMerge) addBody(c Content) {

	for localName, f := range c.Body() {
		// add twice: local and full qualified name
		cntx.Body[FragmentSeparater+localName] = f
		fqn := c.Name()
		if localName != "" {
			fqn += FragmentSeparater + localName
		}
		cntx.Body[fqn] = f
	}
}

func (cntx *ContentMerge) addTail(f Fragment) {
	if f != nil {
		cntx.Tail = append(cntx.Tail, f)
	}
}

// Generates String for the missing Fragment error message. It adds all existing fragments from the body
func generateMissingFragmentString(body map[string]Fragment, fragmentName string) string {
	text := "Fragment does not exist: " + fragmentName + ". Existing fragments: "
	index := 0
	for k, _ := range body {
		if index == 0 {
			text += `"` + k + `"`
		} else {
			text += `, "` + k + `"`
		}
		index++
	}
	return text
}

// Processes all heads to remove duplicate meta and title tags, respecting the priority of head fragments
func (cntx *ContentMerge) processMetaPriorityParsing() {
	headPropertyMap := make(map[string]string)

	for i := len(cntx.Head) - 1; i >= 0; i-- {
		var currentHead interface{} = cntx.Head[i]
		if currentHead != nil {
			currentStringFragment := currentHead.(*StringFragment)
			ParseHeadFragment(currentStringFragment, headPropertyMap)
			cntx.Head[i] = currentStringFragment
		}
	}
}
