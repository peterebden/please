package participle

// A fileInput is the top-level structure of a BUILD file.
type fileInput struct {
	Statements *statement `@@`
}

// A statement is the type we work with most; it's a single Python statement.
type statement struct {
}
