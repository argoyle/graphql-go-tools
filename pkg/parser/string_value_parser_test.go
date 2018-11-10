package parser

import (
	"bytes"
	. "github.com/franela/goblin"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"testing"
)

func TestStringValueParser(t *testing.T) {

	g := Goblin(t)
	RegisterFailHandler(func(m string, _ ...int) { g.Fail(m) })

	g.Describe("parser.parseStringValue", func() {

		tests := []struct {
			it        string
			input     string
			expectErr types.GomegaMatcher
			expectVal types.GomegaMatcher
		}{
			{
				it:        "should parse single line string value",
				input:     `"lorem ipsum"`,
				expectErr: BeNil(),
				expectVal: Equal("lorem ipsum"),
			},
			{
				it: "should parse multi line string value",
				input: `"""
lorem ipsum
"""`,
				expectErr: BeNil(),
				expectVal: Equal("lorem ipsum"),
			},
			{
				it: "should parse multi line string value",
				input: `"""
foo \" bar 
"""`,
				expectErr: BeNil(),
				expectVal: Equal(`foo " bar`),
			},
			{
				it:        "should parse single line string with escaped\"",
				input:     `"foo bar \" baz"`,
				expectErr: BeNil(),
				expectVal: Equal("foo bar \" baz"),
			},
		}

		for _, test := range tests {
			test := test

			g.It(test.it, func() {
				reader := bytes.NewReader([]byte(test.input))
				parser := NewParser()
				parser.l.SetInput(reader)

				val, err := parser.parseStringValue()
				Expect(err).To(test.expectErr)
				Expect(val.Val).To(test.expectVal)
			})
		}
	})
}