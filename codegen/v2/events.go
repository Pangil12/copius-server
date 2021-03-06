package main

// *Set type generator for functions like Walk(), Filter(), FindByID() & IDs()

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"

	"github.com/cortezaproject/corteza-server/pkg/cli"
)

const (
	eventsTemplateFile = "codegen/v2/*.go.tpl"
)

type (
	eventDefMap map[string]eventDef

	eventDef struct {
		On          []string     `yaml:"on"`
		BeforeAfter []string     `yaml:"ba"`
		Properties  []eventProps `yaml:"props"`
		Result      string       `yaml:"result"`
	}

	eventProps struct {
		Name string
		Type string

		// Import path for prop type, use package's type by default (see importTypePathTpl)
		Import string

		// Set property internally only, not via constructor
		Internal bool

		// Do not allow change of the variable through
		Immutable bool
	}

	tplPayload struct {
		Command string
		YAML    string

		Package string

		// List of imports
		// Used only by generated file and not pre-generated-user-file
		Imports []string

		// will be used as string
		ResourceString string

		// will be used as (go) ident
		ResourceIdent string
		Events        eventDef
	}
)

func main() {

	tpl := template.New("").Funcs(map[string]interface{}{
		"camelCase":  camelCase,
		"makeEvents": makeEvents,
	})

	tpl = template.Must(tpl.ParseGlob(eventsTemplateFile))

	var (
		definitionsPathStr string
		serviceStr         string
		overwrite          bool
		// outputFile         string

		decoder *yaml.Decoder

		tplData = &tplPayload{}
	)

	const (
		defResArgName = "result"

		yamlDefFileName    = "events.yaml"
		definitionsPathTpl = "%s/service/event/"
		importTypePathTpl  = "github.com/cortezaproject/corteza-server/%s/types"
		importAuthPath     = "github.com/cortezaproject/corteza-server/pkg/auth"
	)

	flag.StringVar(&definitionsPathStr, "definitions", "", "Location of event definitions file (generated from service if omitted) and output dir")
	flag.StringVar(&serviceStr, "service", "", "Comma separated list of imports")
	flag.BoolVar(&overwrite, "overwrite", false, "Overwrite all files")

	flag.Parse()

	if serviceStr == "" {
		cli.HandleError(fmt.Errorf("can not generate event code without service"))
	}

	if definitionsPathStr == "" {
		definitionsPathStr = fmt.Sprintf(definitionsPathTpl, serviceStr)
	}

	if f, err := os.Open(definitionsPathStr + yamlDefFileName); err != nil {
		cli.HandleError(err)
	} else {
		decoder = yaml.NewDecoder(f)
	}

	tplData.Command = "go run codegen/v2/events.go --service " + serviceStr
	tplData.YAML = definitionsPathStr + yamlDefFileName
	tplData.Package = "event"

	defs := eventDefMap{}
	cli.HandleError(decoder.Decode(&defs))

	for resName, evDef := range defs {
		var (
			l     = len(serviceStr)
			fname = resName
		)

		tplData.Imports = make([]string, 0)
		tplData.ResourceString = resName

		if resName == serviceStr {
			// if resource name = service name, leave it as-is
			tplData.ResourceIdent = resName
		} else if len(resName) <= l || resName[:l+2] == serviceStr+":" {
			// check if resource name is shorter  and has invalid prefix
			cli.HandleError(fmt.Errorf("invalid resource prefix: %q", resName))
		} else {
			tplData.ResourceIdent = camelCase(strings.Split(resName[l+1:], ":")...)

			fname = resName[l+1:]
			fname = strings.ReplaceAll(fname, ":", "_")
			fname = strings.ReplaceAll(fname, "-", "_")
		}

		// Prepare the data

		// no default ("result") result set, use first one from properties
		if evDef.Result == "" && len(evDef.Properties) > 0 {
			evDef.Result = evDef.Properties[0].Name
		}

		// Invoker - user that invoked (triggered) the event
		evDef.Properties = append(evDef.Properties, eventProps{
			Name:      "invoker",
			Type:      "auth.Identifiable",
			Import:    importAuthPath,
			Immutable: false,
			Internal:  true,
		})

		for _, p := range evDef.Properties {
			if p.Import == "" {
				if strings.HasPrefix(p.Type, "*types.") || strings.HasPrefix(p.Type, "types.") {
					p.Import = fmt.Sprintf(importTypePathTpl, serviceStr)
				}
			}

			if p.Import == "" {
				// Do not import empty paths
				continue
			}

			if inSlice(p.Import, tplData.Imports) {
				continue
			}

			tplData.Imports = append(tplData.Imports, p.Import)
		}

		tplData.Events = evDef

		var (
			usrOutput = fmt.Sprintf("%s%s.go", definitionsPathStr, fname)
			genOutput = fmt.Sprintf("%s/%s.gen.go", definitionsPathStr, fname)
		)

		_, err := os.Stat(usrOutput)
		if overwrite || os.IsNotExist(err) {
			writeTo(tpl, tplData, "events.go.tpl", usrOutput)
		}

		writeTo(tpl, tplData, "events.gen.go.tpl", genOutput)
	}
}

func inSlice(s string, ss []string) bool {
	for i := range ss {
		if ss[i] == s {
			return true
		}
	}

	return false
}

// Merge on/before/after events
func makeEvents(def eventDef) []string {
	return append(
		makeEventGroup("on", def.On),
		append(
			makeEventGroup("before", def.BeforeAfter),
			makeEventGroup("after", def.BeforeAfter)...,
		)...,
	)
}

func camelCase(pp ...string) (out string) {
	for i, p := range pp {
		if i > 0 && len(p) > 1 {
			p = strings.ToUpper(p[:1]) + p[1:]
		}

		out = out + p
	}

	return out
}

func makeEventGroup(pfix string, ee []string) (out []string) {
	for _, e := range ee {
		out = append(out, pfix+strings.ToUpper(e[:1])+e[1:])
	}

	return
}

func writeTo(tpl *template.Template, payload interface{}, name, dst string) {
	var output io.WriteCloser
	buf := bytes.Buffer{}

	if err := tpl.ExecuteTemplate(&buf, name, payload); err != nil {
		cli.HandleError(err)
	} else {
		fmtsrc, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "fmt warn: %v", err)
			fmtsrc = buf.Bytes()
		}

		if dst == "" || dst == "-" {
			output = os.Stdout
		} else {
			// cli.HandleError(os.Remove(dst))
			if output, err = os.Create(dst); err != nil {
				cli.HandleError(err)
			}

			defer output.Close()
		}

		if _, err := output.Write(fmtsrc); err != nil {
			cli.HandleError(err)
		}
	}
}
