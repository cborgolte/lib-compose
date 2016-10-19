package composition

import (
        "bytes"
        "errors"
        "io"
        "strings"
)

const (
        DefaultBufferSize = 1024 * 100
)

// ContentMerge is a helper type for creation of a combined html document
// out of multiple Content pages.
type ContentMerge struct {
        MetaJSON  map[string]interface{}
        Head      []Fragment
        BodyAttrs []Fragment
        Body      map[string]Fragment
        Tail      []Fragment
        Buffered  bool
        FdHashes  []string
        priority  bool
}

// NewContentMerge creates a new buffered ContentMerge
func NewContentMerge(metaJSON map[string]interface{}) *ContentMerge {
        cntx := &ContentMerge{
                MetaJSON:  metaJSON,
                Head:      make([]Fragment, 0, 0),
                BodyAttrs: make([]Fragment, 0, 0),
                Body:      make(map[string]Fragment),
                Tail:      make([]Fragment, 0, 0),
                Buffered:  true,
                FdHashes:  make([]string, 0, 0),
                priority:  false,
        }
        return cntx
}

func (cntx *ContentMerge) GetHtml() ([]byte, error) {
        if (cntx.priority) {
                cntx.processMetaPriorityParsing()
        }
        w := bytes.NewBuffer(make([]byte, 0, DefaultBufferSize))

        var executeFragment func(fragmentName string) error
        executeFragment = func(fragmentName string) error {
                f, exist := cntx.Body[fragmentName]
                if !exist {
                        missingFragmentString := generateMissingFragmentString(cntx.Body, fragmentName)
                        return errors.New(missingFragmentString)
                }
                return f.Execute(w, cntx.MetaJSON, executeFragment)
        }

        io.WriteString(w, "<!DOCTYPE html>\n<html>\n  <head>\n    ")

        for _, f := range cntx.Head {
                if err := f.Execute(w, cntx.MetaJSON, executeFragment); err != nil {
                        return nil, err
                }
        }
        io.WriteString(w, "\n  </head>\n  <body")

        for _, f := range cntx.BodyAttrs {
                io.WriteString(w, " ")

                if err := f.Execute(w, cntx.MetaJSON, executeFragment); err != nil {
                        return nil, err
                }
        }

        io.WriteString(w, ">\n    ")

        if err := executeFragment(""); err != nil {
                return nil, err
        }

        for _, f := range cntx.Tail {
                if err := f.Execute(w, cntx.MetaJSON, executeFragment); err != nil {
                        return nil, err
                }
        }

        io.WriteString(w, "\n  </body>\n</html>\n")

        return w.Bytes(), nil
}

func (cntx *ContentMerge) AddContent(fetchResult *FetchResult) {
        cntx.addHead(fetchResult.Content.Head())
        cntx.addBodyAttributes(fetchResult.Content.BodyAttributes())
        cntx.addBody(fetchResult.Def.URL, fetchResult.Content.Body())
        cntx.addTail(fetchResult.Content.Tail())
        cntx.addFdHash(fetchResult.Hash)
        if(fetchResult.Def.Priority > 0) {
                cntx.priority = true
        }
}

func (cntx *ContentMerge) GetHashes() []string {
        return cntx.FdHashes
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

func (cntx *ContentMerge) addBody(url string, bodyFragmentMap map[string]Fragment) {
        for name, f := range bodyFragmentMap {
                // add twice: local and full qualified name
                cntx.Body[name] = f
                cntx.Body[urlToFragmentName(url + "#" + name)] = f
        }
}

func (cntx *ContentMerge) addTail(f Fragment) {
        if f != nil {
                cntx.Tail = append(cntx.Tail, f)
        }
}

func (cntx *ContentMerge) addFdHash(hash string) {
        if hash != "" {
                cntx.FdHashes = append(cntx.FdHashes, hash)
        }
}

// Returns a name from a url, which has template placeholders eliminated
func urlToFragmentName(url string) string {
        url = strings.Replace(url, `§[`, `\§\[`, -1)
        url = strings.Replace(url, `]§`, `\]\§`, -1)
        return url
}

// Generates String for the missing Fragment error message. It adds all existing fragments from the body
func generateMissingFragmentString(body map[string]Fragment, fragmentName string) string {
        text := "Fragment does not exist: " + fragmentName + ". Existing fragments: "
        index := 0
        for k, _ := range body {

                if k != "" {
                        if index == 0 {
                                text += k
                        } else {
                                text += ", " + k
                        }
                        index++
                }
        }
        return text
}

// Processes all heads to remove duplicate meta and title tags, respecting the priority of head fragments
func (cntx *ContentMerge) processMetaPriorityParsing() {
        headPropertyMap := make(map[string]string)

        for i := len(cntx.Head) - 1; i >= 0; i-- {
                var currentHead interface{} = cntx.Head[i];
                if (currentHead != nil) {
                        currentStringFragment := currentHead.(StringFragment)
                        ParseHeadFragment(&currentStringFragment, headPropertyMap)
                        cntx.Head[i] = currentStringFragment
                }
        }
}