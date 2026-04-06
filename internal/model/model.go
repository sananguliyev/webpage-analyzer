package model

type PageAnalysis struct {
	HTMLVersion   string
	Title         string
	Headings      map[string]int // {"h1": 2, "h2": 5, ...}
	InternalLinks int
	ExternalLinks int
	Links         []LinkResult
	HasLoginForm  bool
	TotalLinks    int
	CheckedLinks  int
}

type LinkResult struct {
	URL          string
	IsInternal   bool
	IsAccessible bool
	StatusCode   int
	Error        string
}
