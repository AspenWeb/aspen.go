package goaspen

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	SIMPLATE_TYPE_RENDERED   = "rendered"
	SIMPLATE_TYPE_STATIC     = "static"
	SIMPLATE_TYPE_NEGOTIATED = "negotiated"
	SIMPLATE_TYPE_JSON       = "json"
)

var (
	SIMPLATE_TYPES = []string{
		SIMPLATE_TYPE_JSON,
		SIMPLATE_TYPE_NEGOTIATED,
		SIMPLATE_TYPE_RENDERED,
		SIMPLATE_TYPE_STATIC,
	}
	simplateGenFileTmpl = template.Must(template.New("goaspen-gen").Parse(strings.Replace(`
package goaspen_gen
// GENERATED FILE - DO NOT EDIT
// Rebuild with simplate filesystem parsing thingy!

import (
    "bytes"
    "net/http"
    "text/template"

    "github.com/meatballhat/goaspen"
)

{{.InitPage.Body}}

{{if .HasTemplatePage}}
const (
    SIMPLATE_TMPL_{{.ConstName}} = __BACKTICK__{{.TemplatePage.Body}}__BACKTICK__
)

var (
    simplateTmpl{{.FuncName}} = template.Must(template.New("{{.FuncName}}").Parse(SIMPLATE_TMPL_{{.ConstName}}))
)
{{end}}

func SimplateHandlerFunc{{.FuncName}}(w http.ResponseWriter, req *http.Request) {
    var err error
    ctx := make(map[string]interface{})

    {{range .LogicPages}}
        {{.Body}}
    {{end}}

    {{if .HasTemplatePage}}
    var tmplBuf bytes.Buffer
    err = simplateTmpl{{.FuncName}}.Execute(&tmplBuf, ctx)
    if err != nil {
        w.Header().Set("Content-Type", "text/html")
        w.WriteHeader(http.StatusInternalServerError)
        w.Write(goaspen.HTTP_500_RESPONSE)
        return
    }

    w.Header().Set("Content-Type", "{{.ContentType}}")
    w.WriteHeader(http.StatusOK)
    w.Write(tmplBuf.Bytes())
    {{end}}
}
`, "__BACKTICK__", "`", -1)))
)

type Simplate struct {
	SiteRoot     string
	Filename     string
	Type         string
	ContentType  string
	InitPage     *SimplatePage
	LogicPages   []*SimplatePage
	TemplatePage *SimplatePage
}

type SimplatePage struct {
	Body string
}

func NewSimplateFromString(siteRoot, filename, content string) (*Simplate, error) {
	var err error

	filename, err = filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	filename, err = filepath.Rel(siteRoot, filename)
	if err != nil {
		return nil, err
	}

	rawPages := strings.Split(content, "")
	nbreaks := len(rawPages) - 1

	s := &Simplate{
		SiteRoot:    siteRoot,
		Filename:    filename,
		Type:        SIMPLATE_TYPE_STATIC,
		ContentType: mime.TypeByExtension(path.Ext(filename)),
	}

	if nbreaks == 1 || nbreaks == 2 {
		s.InitPage = &SimplatePage{Body: rawPages[0]}
		s.LogicPages = append(s.LogicPages, &SimplatePage{Body: rawPages[1]})

		if s.ContentType == "application/json" {
			s.Type = SIMPLATE_TYPE_JSON
		} else {
			s.Type = SIMPLATE_TYPE_RENDERED
			s.TemplatePage = &SimplatePage{Body: rawPages[2]}
		}

		return s, nil
	}

	if nbreaks > 2 {
		s.Type = SIMPLATE_TYPE_NEGOTIATED
		s.InitPage = &SimplatePage{Body: rawPages[0]}

		for _, rawPage := range rawPages {
			s.LogicPages = append(s.LogicPages, &SimplatePage{Body: rawPage})
		}

		return s, nil
	}

	return s, nil
}

func (me *Simplate) Execute(wr io.Writer) (err error) {
	errAddr := &err

	defer func(err *error) {
		r := recover()
		if r != nil {
			*err = errors.New(fmt.Sprintf("%v", r))
		}
	}(errAddr)

	debugf("Executing to %s\n", wr)
	*errAddr = simplateGenFileTmpl.Execute(wr, me)

	return
}

func (me *Simplate) escapedFilename() string {
	fn := filepath.Clean(me.Filename)
	lessDots := strings.Replace(fn, ".", "-DOT-", -1)
	lessSlashes := strings.Replace(lessDots, "/", "-SLASH-", -1)
	return strings.Replace(lessSlashes, " ", "-SPACE-", -1)
}

func (me *Simplate) OutputName() string {
	if me.Type == SIMPLATE_TYPE_STATIC {
		return me.Filename
	}

	return me.escapedFilename() + ".go"
}

func (me *Simplate) FuncName() string {
	escaped := me.escapedFilename()
	parts := strings.Split(escaped, "-")
	for i, part := range parts {
		var capitalized []string
		capitalized = append(capitalized, strings.ToUpper(string(part[0])))
		capitalized = append(capitalized, strings.ToLower(part[1:]))
		parts[i] = strings.Join(capitalized, "")
	}

	return strings.Join(parts, "")
}

func (me *Simplate) ConstName() string {
	escaped := me.escapedFilename()
	uppered := strings.ToUpper(escaped)
	return strings.Replace(uppered, "-", "_", -1)
}

func (me *Simplate) HasTemplatePage() bool {
	return me.TemplatePage != nil && len(me.TemplatePage.Body) > 0
}
