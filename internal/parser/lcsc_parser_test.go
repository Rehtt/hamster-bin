package parser

import "testing"

func TestLCSCParser_parseByAPI(t *testing.T) {
	p := NewLCSCParser()
	info, err := p.parseByAPI("C103086")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(info)
}
