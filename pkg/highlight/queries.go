package highlight

import _ "embed"

// Highlight queries embedded from tree-sitter grammar packages.
// Update these files by copying from the grammar's queries/highlights.scm
// and updating the filename to reflect the new version.

//go:embed queries/go-v0.25.0.scm
var goHighlightQuery string

//go:embed queries/python-v0.25.0.scm
var pythonHighlightQuery string

//go:embed queries/yaml-v0.7.2.scm
var yamlHighlightQuery string
