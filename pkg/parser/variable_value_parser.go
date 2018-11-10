package parser

import (
	"github.com/jensneuse/graphql-go-tools/pkg/document"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/token"
)

func (p *Parser) parseVariableValue() (val document.VariableValue, err error) {

	tok, err := p.read(WithWhitelist(token.VARIABLE))
	if err != nil {
		return val, err
	}

	val.Name = string(tok.Literal)

	return
}