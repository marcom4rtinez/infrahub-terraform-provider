// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type InputGraphQLQuery struct {
	QueryName       string
	ObjectName      string
	Required        string
	Fields          []Field
	GenqlientFields []GenqlientField
}

type Field struct {
	Name string
	Type string
}

type GenqlientField struct {
	Name  string
	Query string
}

type DataSourceTemplateData struct {
	QueryName       string
	ObjectName      string
	Required        string
	StructName      string
	Fields          []Field
	GenqlientFields []GenqlientField
}

func GraphQLToTerraform(graphqlType string) string {
	switch graphqlType {
	case "String":
		return "types.String"
	case "Int":
		return "types.Int64"
	case "Float":
		return "types.Float64"
	case "Boolean":
		return "types.Bool"
	default:
		return "types.String"
	}
}

func ParseGraphQLQuery(query string) (*InputGraphQLQuery, error) {
	lines := strings.Split(query, "\n")

	var queryName, required, parentPrefix, objectName string
	var fields []Field
	var inBlock bool
	var prefixList, prefixListImmutable []string

	for number, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "query ") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				containsBracket := strings.IndexByte(parts[1], '(')
				if containsBracket != -1 {
					queryName = parts[1][:containsBracket]
				} else {
					queryName = parts[1]
				}
				queryName = strings.ToLower(string(queryName[0])) + queryName[1:]
			}
		} else if number == 1 {
			// } else if strings.Contains(line, ": $") {
			// This identifies the required field (e.g., name__value: $device_name)
			if strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				required = parts[1][strings.Index(parts[1], "$")+1 : strings.Index(parts[1][strings.Index(parts[1], "$"):], " ")+strings.Index(parts[1], "$")]
				required = strings.TrimRight(required, ")")
				objectNameParts := strings.Split(parts[0], "(")
				objectName = objectNameParts[0]
			} else {
				parts := strings.Split(line, " ")
				objectName = parts[0]
			}
		} else if strings.HasSuffix(line, " {") {
			inBlock = true
			prefix := line[:len(line)-2]
			prefixList = append(prefixList, prefix)
			if strings.Contains(prefix, "_") {
				prefixListImmutable = append(prefixListImmutable, prefix)
			}
			parentPrefix = parentPrefix + prefix + "_"
		} else if line == "}" {
			inBlock = false
			if strings.Count(parentPrefix, "_") < 2 {
				parentPrefix = ""
				break
			}
			// remove last _ and length of last prefix added, workaround for underscores in schema
			parentPrefix = parentPrefix[:len(parentPrefix)-1-len(prefixList[len(prefixList)-1])]
			prefixList = prefixList[:len(prefixList)-1]
			// parentPrefix = parentPrefix[:strings.LastIndex(parentPrefix[:strings.LastIndex(parentPrefix, "_")], "_")+1]
		} else if inBlock {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				fields = append(fields, Field{
					Name: parentPrefix + strings.TrimSpace(parts[0]),
					Type: "String",
				})
				if strings.Contains(parts[0], "_") {
					prefixListImmutable = append(prefixListImmutable, parts[0])
				}
			}
		}
	}

	customSplit := func(str string, exceptions []string) []string {
		var result []string
		var currentWord string

		for _, char := range str {
			if char == '_' {
				isException := false
				for _, exception := range exceptions {
					if strings.HasPrefix(exception, currentWord) {
						if len(currentWord) == len(exception) {
							break
						}
						isException = true
						break
					}
				}
				if !isException {
					result = append(result, currentWord)
					currentWord = ""
				} else {
					currentWord += string(char)
				}
			} else {
				currentWord += string(char)
			}
		}
		result = append(result, currentWord)
		return result
	}

	var genqlientFields []GenqlientField

	for _, entry := range fields {
		parts := customSplit(entry.Name, prefixListImmutable)

		// Capitalize each part except for the first one
		caser := cases.Title(language.English)
		for i := range parts {
			// Capitalize the first letter of each part
			parts[i] = caser.String(parts[i])
			if required != "" {
				if parts[i] == "Edges" {
					parts[i] = "Edges[0]"
				}
			} else {
				if parts[i] == "Edges" {
					parts[i] = "Edges[i]"
				}
			}
		}

		// Join the parts using a dot separator
		genqlientFields = append(genqlientFields, GenqlientField{
			Name:  entry.Name,
			Query: objectName + "." + strings.Join(parts, "."),
		})
	}

	if queryName == "" {
		return nil, fmt.Errorf("failed to parse GraphQL query: missing query name")
	}

	return &InputGraphQLQuery{
		QueryName:       queryName,
		ObjectName:      objectName,
		Required:        required,
		Fields:          fields,
		GenqlientFields: genqlientFields,
	}, nil
}

func GenerateTerraformDataSource(parsedQuery *InputGraphQLQuery) (string, error) {
	structName := parsedQuery.QueryName + "DataSource"
	data := DataSourceTemplateData{
		QueryName:       parsedQuery.QueryName,
		ObjectName:      parsedQuery.ObjectName,
		Required:        parsedQuery.Required,
		StructName:      structName,
		Fields:          parsedQuery.Fields,
		GenqlientFields: parsedQuery.GenqlientFields,
	}

	// Template for the Terraform data source
	const tpl = `package provider

import (
	"context"
	"fmt"

	infrahub_sdk "github.com/opsmill/infrahub-sdk-go"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &{{.QueryName}}DataSource{}
	_ datasource.DataSourceWithConfigure = &{{.QueryName}}DataSource{}
)

// New{{.QueryName | title }}DataSource is a helper function to simplify the provider implementation.
func New{{.QueryName | title }}DataSource() datasource.DataSource {
	return &{{.QueryName}}DataSource{}
}


type {{.StructName}} struct {
	client     *graphql.Client
	{{- if .Required }}
	{{.Required | title }} types.String ` + "`tfsdk:\"{{.Required}}\"`" + `
	{{- range .Fields }}
	{{ .Name | title }} types.String ` + "`tfsdk:\"{{ .Name }}\"`" + `
	{{- end }}
	{{- else }}
	{{ .QueryName }} []{{ .QueryName}}Model ` + "`tfsdk:\"{{ .QueryName }}\"`" + `
	{{- end }}
}

{{- if not .Required }}
type {{ .QueryName}}Model struct {
	{{- range .Fields }}
	{{ .Name | title }} types.String ` + "`tfsdk:\"{{ .Name }}\"`" + `
	{{- end }}
}
{{- end }}

func (d *{{.QueryName}}DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_{{.QueryName}}"
}

func (d *{{.QueryName}}DataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			{{- if .Required }}
			"{{.Required}}": schema.StringAttribute{
				Required: true,
			},
			{{- range .Fields }}
			"{{ .Name }}": schema.StringAttribute{
				Computed: true,
			},
			{{- end }}
			{{- else}}
			"{{ .QueryName }}": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						{{- range .Fields }}
						"{{ .Name }}": schema.StringAttribute{
							Computed: true,
						},
						{{- end }}
					},
				},
			},
			{{- end }}
		},
	}
}

func (d *{{.QueryName }}DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading {{.QueryName}} data...")
	var config {{.StructName}}

	// Read configuration into config
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	{{- if .Required }}
	response, err := infrahub_sdk.{{.QueryName | title}}(ctx, *d.client, config.{{.Required | title }}.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read {{.QueryName}} from Infrahub",
			err.Error(),
		)
		return
	}

	if len(response.{{.ObjectName}}.Edges) != 1 {
		resp.Diagnostics.AddError(
			"Didn't receive a single {{.QueryName}}, query didn't return exactly 1 {{.QueryName}}",
			"Expected exactly 1 {{.QueryName}} in response, got a different count.",
		)
		return
	}

	state := {{.StructName}}{
		{{.Required | title}}: config.{{.Required | title }},
		{{- range .GenqlientFields }}
		{{ .Name | title }}: types.StringValue(response.{{ .Query }}),
		{{- end }}
	}
	{{- else }}
	response, err := infrahub_sdk.{{.QueryName | title}}(ctx, *d.client)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read {{.QueryName}} from Infrahub",
			err.Error(),
		)
		return
	}
	var state {{.StructName}}
	for i, _ := range response.{{.ObjectName}}.Edges {
		current := {{.QueryName}}Model{
			{{- range .GenqlientFields }}
			{{ .Name | title }}: types.StringValue(response.{{ .Query }}),
			{{- end }}
		}
		state.{{.QueryName}} = append(state.{{.QueryName}}, current)
	}
	{{- end}}


	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *{{.QueryName}}DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(graphql.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *graphql.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = &client
}

`

	// Render the template
	caser := cases.Title(language.English)
	tmpl, err := template.New("datasource").Funcs(template.FuncMap{
		"title": caser.String,
	}).Parse(tpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func readAndGenerateProvider(graphqlQuery string) {

	// Parse the query
	parsedQuery, err := ParseGraphQLQuery(graphqlQuery)
	if err != nil {
		fmt.Println("Error parsing GraphQL query:", err)
		os.Exit(1)
	}

	// Generate the Terraform data source code
	code, err := GenerateTerraformDataSource(parsedQuery)
	if err != nil {
		fmt.Println("Error generating Terraform data source:", err)
		os.Exit(1)
	}

	// Print the generated code
	// fmt.Println(code)

	file, err := os.Create(fmt.Sprintf("../internal/provider/%s_data_source.go", parsedQuery.QueryName))
	if err != nil {
		fmt.Println("Error creating the file:", err)
		return
	}
	defer file.Close()

	// Write the content to the file
	_, err = file.WriteString(code)
	if err != nil {
		fmt.Println("Error writing to the file:", err)
		return
	}

	fmt.Printf("Content written to %s_data_source.go file successfully!\n", parsedQuery.QueryName)
}

func main() {
	dir := "gql"

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			readAndGenerateProvider(string(data))
		}

		return nil
	})

	if err != nil {
		fmt.Println(err)
	}
}