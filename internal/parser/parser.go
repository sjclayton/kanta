package parser

type Span struct {
	Start int
	End   int
}

type Document struct {
	Raw []byte
}

func Parse(_ []byte) (Document, error) {
	return Document{}, nil
}
