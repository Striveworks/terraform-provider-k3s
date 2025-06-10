package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type versionDiagnositcs struct {
	severity diag.Severity
	summary  string
	detail   string
}

func fromError(summary string, e error) diag.Diagnostic {
	return versionDiagnositcs{
		severity: 1,
		summary:  summary,
		detail:   e.Error(),
	}
}

func (v versionDiagnositcs) Severity() diag.Severity {
	return v.severity
}
func (v versionDiagnositcs) Summary() string {
	return v.summary
}
func (v versionDiagnositcs) Detail() string {
	return v.detail
}
func (v versionDiagnositcs) Equal(o diag.Diagnostic) bool {
	return v.severity == o.Severity() && v.summary == o.Summary() && v.detail == o.Detail()
}

type MarkdownDescription string

func (s MarkdownDescription) ToMarkdown() string {
	return strings.ReplaceAll(string(s), "!!!", "```")
}
