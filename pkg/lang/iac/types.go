package iac

// Class is how an attribute's value is known.
type Class string

// The five classes. Absence is not one.
const (
	// ClassLiteral: written as a constant. A check may treat it as a fact.
	ClassLiteral Class = "literal"
	// ClassDefaulted: resolves only through a variable default or a local chain ending in one. A check may treat
	// it as an assumption an operator can override.
	ClassDefaulted Class = "defaulted"
	// ClassReference: names another resource, data source, module output or parameter. A check may treat it as a
	// relation.
	ClassReference Class = "reference"
	// ClassExpression: computed, with no static answer. A check treats it as unknown.
	ClassExpression Class = "expression"
	// ClassUnknown: the reader could not classify it. A check treats it as unknown.
	ClassUnknown Class = "unknown"
)

// Tool values of Resource.Tool.
const (
	ToolTerraform      = "terraform"
	ToolCloudFormation = "cloudformation"
	ToolPulumi         = "pulumi"
)

// Mode values of Resource.Mode. A module call is a resource-like relation and is never expanded.
const (
	ModeManaged   = "managed"
	ModeData      = "data"
	ModeModule    = "module"
	ModeEphemeral = "ephemeral"
)

// MultiplicityKind is how many instances a resource declares.
type MultiplicityKind string

// The kinds of Multiplicity.Kind.
const (
	MultiSingle    MultiplicityKind = "single"
	MultiCount     MultiplicityKind = "count"
	MultiForEach   MultiplicityKind = "for_each"
	MultiCondition MultiplicityKind = "condition"
)

// Span is a region of a source file. Lines and columns are 1-based, columns count characters and not bytes, and
// the end is exclusive. File is the path the caller gave.
type Span struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
}

// Multiplicity is the instance count of a resource. Class is the class of the count, for_each or condition
// expression, and is "" for MultiSingle. Refs names what the expression reads.
type Multiplicity struct {
	Kind  MultiplicityKind `json:"kind"`
	Class Class            `json:"class"`
	Refs  []string         `json:"refs"`
	Span  Span             `json:"span"`
}

// Attr is one attribute of a resource.
type Attr struct {
	// Path is versioning[*].enabled or rule[*].id: every nested Terraform block segment takes [*], one block or
	// several, and so does every CloudFormation or Pulumi list element. A resource's own lifecycle block is
	// meta-configuration and takes none. The path is empty for the one attribute at the root that stands for a
	// Properties value the reader could not walk (an intrinsic, an alias, an interpolation); it reads unknown.
	Path  string `json:"path"`
	Class Class  `json:"class"`
	// Value is set only under WithValues, and only for literal and defaulted classes whose value could be
	// computed without evaluating a function, and never for what the source marks secret. A number is an int64
	// when it is an integer in range written without a fraction or exponent, a float64 when float64 holds it
	// exactly, a json.Number with its exact text when it is finite as float64 and does not fit either, and its
	// source text as a string when it is past float64's range. json.Marshal of a Value never fails.
	Value any `json:"value,omitempty"`
	// Refs are the static references the value reads, sorted: var.x, aws_s3_bucket.logs.id, Param, Res.Arn.
	// They name things, never values.
	Refs []string `json:"refs"`
	Span Span     `json:"span"`
}

// Resource is one resource, data source or module call.
type Resource struct {
	// Tool is a Tool* value, and Mode a Mode* value.
	Tool string `json:"tool"`
	Mode string `json:"mode"`
	// Type is aws_s3_bucket, AWS::S3::Bucket or aws:s3/bucket:Bucket.
	Type string `json:"type"`
	Name string `json:"name"`
	// Provider is the provider selection a Terraform resource states (aws.west), and "" for the rest.
	Provider     string       `json:"provider"`
	Span         Span         `json:"span"`
	Attrs        []Attr       `json:"attrs"`
	Multiplicity Multiplicity `json:"multiplicity"`
}

// Dropped names a resource-section entry that was not read as a resource: a Fn::ForEach, an alias, a merge-only
// entry, a value that is not a mapping, an entry without a type, or a Terraform data source inside a check block
// (check.NAME.data.TYPE.NAME). Absence is never silent: a resource that is in neither Resources nor Dropped does
// not exist in the file.
type Dropped struct {
	Name string `json:"name"`
}

// ResourceSet is what one read returned. Resources are sorted by span and attributes by path, then span.
type ResourceSet struct {
	Resources []Resource `json:"resources"`
	// SyntaxErrors counts the syntax errors the parser reported, keys a mapping repeats, YAML documents after the
	// first (which are not read), values the reader could not walk, a resource section that is not a mapping and a
	// resource whose merge key may carry its properties. A read still returns the resources it could parse, and an
	// attribute it could not read is unknown, never what a parser's recovery guessed.
	SyntaxErrors int       `json:"syntax_errors"`
	Dropped      []Dropped `json:"dropped"`
}
