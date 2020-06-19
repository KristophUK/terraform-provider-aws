package sweepers

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"strings"
	"text/template"
)

// "_test" is required for the functions to be compiled in the correct package
const filenameFormat = `gen_%s_sweepers_test.go`

type ResourceType struct {
	ListerFunction       string
	ListerOutputType     string
	ListerPageField      string
	ResourceNameFunction string
}

type TemplateData struct {
	Package       string
	ServiceName   string
	ResourceTypes map[string]ResourceType
}

func Run(serviceName string, resourceTypes map[string]ResourceType) {
	destinationPackage := os.Getenv("GOPACKAGE")
	if destinationPackage == "" {
		log.Fatal("error: required environment variable GOPACKAGE not defined")
	}

	templateData := TemplateData{
		Package:       destinationPackage,
		ServiceName:   serviceName,
		ResourceTypes: resourceTypes,
	}
	templateFuncMap := template.FuncMap{
		"Title": strings.Title,
	}

	tmpl, err := template.New("sweepers").Funcs(templateFuncMap).Parse(sweepersBody)

	if err != nil {
		log.Fatalf("error parsing template: %s", err)
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, templateData)

	if err != nil {
		log.Fatalf("error executing template: %s", err)
	}

	generatedFileContents, err := format.Source(buffer.Bytes())

	if err != nil {
		log.Fatalf("error formatting generated file: %s", err)
	}

	filename := fmt.Sprintf(filenameFormat, serviceName)
	f, err := os.Create(filename)

	if err != nil {
		log.Fatalf("error creating file (%s): %s", filename, err)
	}

	defer f.Close()

	_, err = f.Write(generatedFileContents)

	if err != nil {
		log.Fatalf("error writing to file (%s): %s", filename, err)
	}
}

var sweepersBody = `// Code generated by {{ .ServiceName }}/generators/sweepers/main.go; DO NOT EDIT.

package {{ .Package }}

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/{{ .ServiceName }}"
	"github.com/hashicorp/go-multierror"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/{{ .ServiceName }}/lister"
)

{{- $serviceName := .ServiceName }}
{{- $serviceNameTitle := .ServiceName | Title }}
{{- range $resourceTypeName, $resourceType := .ResourceTypes }}

{{- $fullResourceName := printf "%s %s" $serviceNameTitle $resourceTypeName }}
{{- $fullResourceNamePlural := printf "%s %ss" $serviceNameTitle $resourceTypeName }}

func testSweep{{ $serviceNameTitle }}{{ $resourceTypeName }}s(region string) error {
	conn, err := shared{{ $serviceNameTitle }}ClientForRegion(region)
	if err != nil {
		return err
	}

	var sweeperErrs *multierror.Error

	err = lister.{{ $resourceType.ListerFunction }}(conn, func(page *{{ $serviceName }}.{{ $resourceType.ListerOutputType }}, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, r := range page.{{ $resourceType.ListerPageField }} {
			name := aws.StringValue(r.{{ $resourceType.ResourceNameFunction }})

			log.Printf("[INFO] Deleting {{ $fullResourceName }}: %s", name)
			err := delete{{ $serviceNameTitle }}{{ $resourceTypeName }}(conn, delete{{ $serviceNameTitle }}{{ $resourceTypeName }}InputFromAPIResource(r))
			if err != nil {
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error deleting {{ $fullResourceName }} (%s): %w", name, err))
				continue
			}
		}

		return !lastPage
	})

	if testSweepSkipSweepError(err) {
		log.Printf("[WARN] Skipping {{ $fullResourceName }} sweeper for %q: %s", region, err)
		return sweeperErrs.ErrorOrNil() // In case we have completed some pages, but had errors
	}

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing {{ $fullResourceNamePlural }}: %w", err))
	}

	return sweeperErrs.ErrorOrNil()
}
{{- end }}
`
