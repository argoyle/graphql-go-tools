package astparser

import (
	"fmt"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/input"
	"github.com/jensneuse/graphql-go-tools/pkg/lexer"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/keyword"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/position"
	"io/ioutil"
	"testing"
)

func TestParser_Parse(t *testing.T) {

	type check func(in *input.Input, doc *ast.Document, extra interface{})
	type action func(parser *Parser) (interface{}, error)

	parse := func() action {
		return func(parser *Parser) (interface{}, error) {
			return nil, parser.Parse(parser.input, parser.document)
		}
	}

	parseType := func() action {
		return func(parser *Parser) (interface{}, error) {
			parser.lexer.SetInput(parser.input)
			parser.tokenize()
			ref := parser.parseType()
			return ref, parser.err
		}
	}

	parseValue := func() action {
		return func(parser *Parser) (interface{}, error) {
			parser.lexer.SetInput(parser.input)
			parser.tokenize()
			value := parser.parseValue()
			return value, parser.err
		}
	}

	parseSelectionSet := func() action {
		return func(parser *Parser) (interface{}, error) {
			parser.lexer.SetInput(parser.input)
			parser.tokenize()
			set := parser.parseSelectionSet()
			return set, parser.err
		}
	}

	parseFragmentSpread := func() action {
		return func(parser *Parser) (interface{}, error) {
			parser.lexer.SetInput(parser.input)
			parser.tokenize()
			fragmentSpread := parser.parseFragmentSpread(position.Position{})
			return parser.document.FragmentSpreads[fragmentSpread], parser.err
		}
	}

	parseInlineFragment := func() action {
		return func(parser *Parser) (interface{}, error) {
			parser.lexer.SetInput(parser.input)
			parser.tokenize()
			inlineFragment := parser.parseInlineFragment(position.Position{})
			return parser.document.InlineFragments[inlineFragment], parser.err
		}
	}

	parseVariableDefinitionList := func() action {
		return func(parser *Parser) (interface{}, error) {
			parser.lexer.SetInput(parser.input)
			parser.tokenize()
			variableDefinitionList := parser.parseVariableDefinitionList()
			return variableDefinitionList, parser.err
		}
	}

	run := func(inputString string, action func() action, wantErr bool, checks ...check) {

		in := &input.Input{}
		in.ResetInputBytes([]byte(inputString))
		doc := ast.NewDocument()

		parser := NewParser()
		parser.input = in
		parser.document = doc

		extra, err := action()(parser)

		if wantErr && err == nil {
			panic("want err, got nil")
		} else if !wantErr && err != nil {
			panic(fmt.Errorf("want nil, got err: %s", err.Error()))
		}

		for _, check := range checks {
			check(in, doc, extra)
		}
	}

	t.Run("tokenize", func(t *testing.T) {
		in := &input.Input{}
		in.ResetInputBytes([]byte(
			`schema {
				query: Query
				mutation: Mutation
				subscription: Subscription 
			}`,
		))

		doc := &ast.Document{}
		parser := NewParser()
		parser.input = in
		parser.document = doc
		parser.lexer.SetInput(in)

		parser.tokenize()

		for i, want := range []keyword.Keyword{
			keyword.SCHEMA, keyword.CURLYBRACKETOPEN,
			keyword.QUERY, keyword.COLON, keyword.IDENT,
			keyword.MUTATION, keyword.COLON, keyword.IDENT,
			keyword.SUBSCRIPTION, keyword.COLON, keyword.IDENT,
			keyword.CURLYBRACKETCLOSE,
		} {
			parser.peek()
			got := parser.read().Keyword
			if got != want {
				t.Fatalf("want keyword %s @ %d, got: %s", want, i, got)
			}
		}
	})

	t.Run("no err on empty input", func(t *testing.T) {
		run("", parse, false)
	})
	t.Run("schema", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`schema {
						query: Query
						mutation: Mutation
						subscription: Subscription 
					}`, parse,
				false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					definition := doc.RootNodes[0]
					if definition.Ref != 0 {
						panic("want 0")
					}
					if definition.Kind != ast.NodeKindSchemaDefinition {
						panic("want NodeKindSchemaDefinition")
					}
					schema := doc.SchemaDefinitions[0]
					query := doc.RootOperationTypeDefinitions[schema.RootOperationTypeDefinitions.Refs[0]]
					if query.OperationType != ast.OperationTypeQuery {
						panic("want OperationTypeQuery")
					}
					name := in.ByteSliceString(query.NamedType.Name)
					if name != "Query" {
						panic(fmt.Errorf("want 'Query', got '%s'", name))
					}
					mutation := doc.RootOperationTypeDefinitions[schema.RootOperationTypeDefinitions.Refs[1]]
					if mutation.OperationType != ast.OperationTypeMutation {
						panic("want OperationTypeMutation")
					}
					name = in.ByteSliceString(mutation.NamedType.Name)
					if name != "Mutation" {
						panic(fmt.Errorf("want 'Mutation', got '%s'", name))
					}
					subscription := doc.RootOperationTypeDefinitions[schema.RootOperationTypeDefinitions.Refs[2]]
					if subscription.OperationType != ast.OperationTypeSubscription {
						panic("want OperationTypeSubscription")
					}
					name = in.ByteSliceString(subscription.NamedType.Name)
					if name != "Subscription" {
						panic(fmt.Errorf("want 'Subscription', got '%s'", name))
					}
				})
		})
		t.Run("with directives", func(t *testing.T) {
			run(`schema @foo @bar(baz: "bal") {
						query: Query 
					}`, parse, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				schema := doc.SchemaDefinitions[0]
				foo := doc.Directives[schema.Directives.Refs[0]]
				fooName := in.ByteSliceString(foo.Name)
				if fooName != "foo" {
					panic("want foo, got: " + fooName)
				}
				if len(foo.Arguments.Refs) != 0 {
					panic("should not HasNext")
				}
				bar := doc.Directives[schema.Directives.Refs[1]]
				barName := in.ByteSliceString(bar.Name)
				if barName != "bar" {
					panic("want bar, got: " + barName)
				}
				baz := doc.Arguments[bar.Arguments.Refs[0]]
				bazName := in.ByteSliceString(baz.Name)
				if bazName != "baz" {
					panic("want baz, got: " + bazName)
				}
				if baz.Value.Kind != ast.ValueKindString {
					panic("want ValueKindString")
				}
				bal := in.ByteSliceString(doc.StringValues[baz.Value.Ref].Content)
				if bal != "bal" {
					panic("want bal, got: " + bal)
				}
			})
		})
		t.Run("invalid body missing", func(t *testing.T) {
			run(`schema`, parse, true)
		})
		t.Run("invalid body unclosed", func(t *testing.T) {
			run(`schema {`, parse, true)
		})
		t.Run("invalid directive arguments unclosed", func(t *testing.T) {
			run(`schema @foo( {}`, parse, true)
		})
		t.Run("invalid directive without @", func(t *testing.T) {
			run(`schema foo {}`, parse, true)
		})
	})
	t.Run("schema extension", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`extend schema {
						query: Query
						mutation: Mutation
						subscription: Subscription 
					}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {

					schema := doc.SchemaExtensions[0]

					query := doc.RootOperationTypeDefinitions[schema.RootOperationTypeDefinitions.Refs[0]]
					if query.OperationType != ast.OperationTypeQuery {
						panic("want OperationTypeQuery")
					}
					name := in.ByteSliceString(query.NamedType.Name)
					if name != "Query" {
						panic(fmt.Errorf("want 'Query', got '%s'", name))
					}

					mutation := doc.RootOperationTypeDefinitions[schema.RootOperationTypeDefinitions.Refs[1]]
					if mutation.OperationType != ast.OperationTypeMutation {
						panic("want OperationTypeMutation")
					}
					name = in.ByteSliceString(mutation.NamedType.Name)
					if name != "Mutation" {
						panic(fmt.Errorf("want 'Mutation', got '%s'", name))
					}
					subscription := doc.RootOperationTypeDefinitions[schema.RootOperationTypeDefinitions.Refs[2]]
					if subscription.OperationType != ast.OperationTypeSubscription {
						panic("want OperationTypeSubscription")
					}
					name = in.ByteSliceString(subscription.NamedType.Name)
					if name != "Subscription" {
						panic(fmt.Errorf("want 'Subscription', got '%s'", name))
					}
				})
		})
	})
	t.Run("object type extension", func(t *testing.T) {
		t.Run("complex", func(t *testing.T) {
			run(`extend type Person implements Foo & Bar {
							name: String
							"age of the person"
							age: Int
							"""
							date of birth
							"""
							dateOfBirth: Date
						}`, parse, false, func(in *input.Input, doc *ast.Document, extra interface{}) {

				person := doc.ObjectTypeExtensions[0]
				personName := in.ByteSliceString(person.Name)
				if personName != "Person" {
					panic("want person")
				}

				// interfaces

				implementsFoo := doc.Types[person.ImplementsInterfaces.Refs[0]]
				if implementsFoo.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(implementsFoo.Name) != "Foo" {
					panic("want Foo")
				}

				implementsBar := doc.Types[person.ImplementsInterfaces.Refs[1]]
				if implementsBar.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(implementsBar.Name) != "Bar" {
					panic("want Bar")
				}

				// field definitions
				nameString := doc.FieldDefinitions[person.FieldsDefinition.Refs[0]]
				name := in.ByteSliceString(nameString.Name)
				if name != "name" {
					panic("want name")
				}
				nameStringType := doc.Types[nameString.Type]
				if nameStringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				stringName := in.ByteSliceString(nameStringType.Name)
				if stringName != "String" {
					panic("want String")
				}

				ageField := doc.FieldDefinitions[person.FieldsDefinition.Refs[1]]
				if !ageField.Description.IsDefined {
					panic("want true")
				}
				if ageField.Description.IsBlockString {
					panic("want false	")
				}
				if in.ByteSliceString(ageField.Description.Content) != "age of the person" {
					panic("want 'age of the person'")
				}
				if in.ByteSliceString(ageField.Name) != "age" {
					panic("want age")
				}
				intType := doc.Types[ageField.Type]
				if intType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(intType.Name) != "Int" {
					panic("want Int")
				}

				dateOfBirthField := doc.FieldDefinitions[person.FieldsDefinition.Refs[2]]
				if in.ByteSliceString(dateOfBirthField.Name) != "dateOfBirth" {
					panic("want dateOfBirth")
				}
				if !dateOfBirthField.Description.IsDefined {
					panic("want true")
				}
				if !dateOfBirthField.Description.IsBlockString {
					panic("want true")
				}
				if in.ByteSliceString(dateOfBirthField.Description.Content) != `
							date of birth
							` {
					panic(fmt.Sprintf("want 'date of birth' got: '%s'", in.ByteSliceString(dateOfBirthField.Description.Content)))
				}
				dateType := doc.Types[dateOfBirthField.Type]
				if in.ByteSliceString(dateType.Name) != "Date" {
					panic("want Date")
				}
			})
		})
	})
	t.Run("interface type extension", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`extend interface NamedEntity @foo {
 								name: String
							}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					namedEntity := doc.InterfaceTypeExtensions[0]
					if in.ByteSliceString(namedEntity.Name) != "NamedEntity" {
						panic("want NamedEntity")
					}

					// fields
					name := doc.FieldDefinitions[namedEntity.FieldsDefinition.Refs[0]]
					if in.ByteSliceString(name.Name) != "name" {
						panic("want name")
					}

					//directives
					foo := doc.Directives[namedEntity.Directives.Refs[0]]
					if in.ByteSliceString(foo.Name) != "foo" {
						panic("want foo")
					}
				})
		})
	})
	t.Run("scalar type extension", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`extend scalar JSON @foo`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					scalar := doc.ScalarTypeExtensions[0]
					if in.ByteSliceString(scalar.Name) != "JSON" {
						panic("want JSON")
					}
					foo := doc.Directives[scalar.Directives.Refs[0]]
					if in.ByteSliceString(foo.Name) != "foo" {
						panic("want foo")
					}
				})
		})
	})
	t.Run("union type extension", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`extend union SearchResult = Photo | Person`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					SearchResult := doc.UnionTypeExtensions[0]

					// union member types

					// Photo
					Photo := doc.Types[SearchResult.UnionMemberTypes.Refs[0]]
					if Photo.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(Photo.Name) != "Photo" {
						panic("want Photo")
					}

					// Person
					Person := doc.Types[SearchResult.UnionMemberTypes.Refs[1]]
					if Person.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(Person.Name) != "Person" {
						panic("want Person")
					}

					// no more types
					if len(SearchResult.UnionMemberTypes.Refs) != 2 {
						t.Fatalf("want 2")
					}
				})
		})
	})
	t.Run("enum type extension", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`extend enum Direction @bar {
							  NORTH
							  EAST
							  SOUTH
							  "describes WEST"
							  WEST @foo
							}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					direction := doc.EnumTypeExtensions[0]
					if in.ByteSliceString(direction.Name) != "Direction" {
						panic("want Direction")
					}

					// directives
					bar := doc.Directives[direction.Directives.Refs[0]]
					if in.ByteSliceString(bar.Name) != "bar" {
						panic("want bar")
					}

					// values

					wantValue := func(index int, name string) {
						enum := doc.EnumValueDefinitions[direction.EnumValuesDefinition.Refs[index]]
						if in.ByteSliceString(enum.EnumValue) != name {
							panic(fmt.Sprintf("want %s", name))
						}
					}

					wantValue(0, "NORTH")
					wantValue(1, "EAST")
					wantValue(2, "SOUTH")
					wantValue(3, "WEST")

					west := doc.EnumValueDefinitions[direction.EnumValuesDefinition.Refs[3]]
					if !west.Description.IsDefined {
						panic("want true")
					}
					if in.ByteSliceString(west.Description.Content) != "describes WEST" {
						panic("want describes WEST")
					}

					foo := doc.Directives[west.Directives.Refs[0]]
					if in.ByteSliceString(foo.Name) != "foo" {
						panic("want foo")
					}

					if len(direction.EnumValuesDefinition.Refs) != 4 {
						panic("want 4")
					}
				})
		})
	})
	t.Run("input object type extension", func(t *testing.T) {
		t.Run("complex", func(t *testing.T) {
			run(`	extend input Person {
									name: String = "Gopher"
								}`, parse,
				false, func(in *input.Input, doc *ast.Document, extra interface{}) {
					person := doc.InputObjectTypeExtensions[0]
					if in.ByteSliceString(person.Name) != "Person" {
						panic("want person")
					}

					name := doc.InputValueDefinitions[person.InputFieldsDefinition.Refs[0]]
					if in.ByteSliceString(name.Name) != "name" {
						panic("want name")
					}
					if !name.DefaultValue.IsDefined {
						panic("want true")
					}
					if name.DefaultValue.Value.Kind != ast.ValueKindString {
						panic("want ValueKindString")
					}
					if in.ByteSliceString(doc.StringValues[name.DefaultValue.Value.Ref].Content) != "Gopher" {
						panic("want Gopher")
					}
				})
		})
	})
	t.Run("scalar type definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`scalar JSON`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					scalar := doc.ScalarTypeDefinitions[0]
					if in.ByteSliceString(scalar.Name) != "JSON" {
						panic("want JSON")
					}
				})
		})
		t.Run("with description", func(t *testing.T) {
			run(`"JSON scalar description" scalar JSON`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					scalar := doc.ScalarTypeDefinitions[0]
					if in.ByteSliceString(scalar.Name) != "JSON" {
						panic("want JSON")
					}
					if !scalar.Description.IsDefined {
						panic("want true")
					}
					if in.ByteSliceString(scalar.Description.Content) != "JSON scalar description" {
						panic("want 'JSON scalar description'")
					}
				})
		})
		t.Run("with directive", func(t *testing.T) {
			run(`scalar JSON @foo`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					scalar := doc.ScalarTypeDefinitions[0]
					if in.ByteSliceString(scalar.Name) != "JSON" {
						panic("want JSON")
					}
					foo := doc.Directives[scalar.Directives.Refs[0]]
					if in.ByteSliceString(foo.Name) != "foo" {
						panic("want foo")
					}
				})
		})
	})
	t.Run("object type definition", func(t *testing.T) {
		t.Run("complex", func(t *testing.T) {
			run(`type Person implements Foo & Bar {
							name: String
							"age of the person"
							age: Int
							"""
							date of birth
							"""
							dateOfBirth: Date
						}`, parse, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				person := doc.ObjectTypeDefinitions[0]
				personName := in.ByteSliceString(person.Name)
				if personName != "Person" {
					panic("want person")
				}

				// interfaces

				implementsFoo := doc.Types[person.ImplementsInterfaces.Refs[0]]
				if implementsFoo.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(implementsFoo.Name) != "Foo" {
					panic("want Foo")
				}

				implementsBar := doc.Types[person.ImplementsInterfaces.Refs[1]]
				if implementsBar.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(implementsBar.Name) != "Bar" {
					panic("want Bar")
				}

				// field definitions
				nameString := doc.FieldDefinitions[person.FieldsDefinition.Refs[0]]
				name := in.ByteSliceString(nameString.Name)
				if name != "name" {
					panic("want name")
				}
				nameStringType := doc.Types[nameString.Type]
				if nameStringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				stringName := in.ByteSliceString(nameStringType.Name)
				if stringName != "String" {
					panic("want String")
				}

				ageField := doc.FieldDefinitions[person.FieldsDefinition.Refs[1]]
				if !ageField.Description.IsDefined {
					panic("want true")
				}
				if ageField.Description.IsBlockString {
					panic("want false	")
				}
				if in.ByteSliceString(ageField.Description.Content) != "age of the person" {
					panic("want 'age of the person'")
				}
				if in.ByteSliceString(ageField.Name) != "age" {
					panic("want age")
				}
				intType := doc.Types[ageField.Type]
				if intType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(intType.Name) != "Int" {
					panic("want Int")
				}

				dateOfBirthField := doc.FieldDefinitions[person.FieldsDefinition.Refs[2]]
				if in.ByteSliceString(dateOfBirthField.Name) != "dateOfBirth" {
					panic("want dateOfBirth")
				}
				if !dateOfBirthField.Description.IsDefined {
					panic("want true")
				}
				if !dateOfBirthField.Description.IsBlockString {
					panic("want true")
				}
				if in.ByteSliceString(dateOfBirthField.Description.Content) != `
							date of birth
							` {
					panic(fmt.Sprintf("want 'date of birth' got: '%s'", in.ByteSliceString(dateOfBirthField.Description.Content)))
				}
				dateType := doc.Types[dateOfBirthField.Type]
				if in.ByteSliceString(dateType.Name) != "Date" {
					panic("want Date")
				}
			})
		})
		t.Run("with directives", func(t *testing.T) {
			run(`type Person @foo @bar {}`, parse, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				person := doc.ObjectTypeDefinitions[0]
				personName := in.ByteSliceString(person.Name)
				if personName != "Person" {
					panic("want person")
				}

				// directives

				foo := doc.Directives[person.Directives.Refs[0]]
				if in.ByteSliceString(foo.Name) != "foo" {
					panic("want foo")
				}

				bar := doc.Directives[person.Directives.Refs[1]]
				if in.ByteSliceString(bar.Name) != "bar" {
					panic("want bar")
				}
			})
		})
		t.Run("implements optional variant", func(t *testing.T) {
			run(`type Person implements & Foo & Bar {}`, parse, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				person := doc.ObjectTypeDefinitions[0]
				personName := in.ByteSliceString(person.Name)
				if personName != "Person" {
					panic("want person")
				}
				// interfaces

				implementsFoo := doc.Types[person.ImplementsInterfaces.Refs[0]]
				if implementsFoo.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(implementsFoo.Name) != "Foo" {
					panic("want Foo")
				}

				implementsBar := doc.Types[person.ImplementsInterfaces.Refs[1]]
				if implementsBar.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(implementsBar.Name) != "Bar" {
					panic("want Bar")
				}
			})
		})
		t.Run("input value definition list", func(t *testing.T) {
			run(`	type Person { 
									"name description"
									name(
										a: String!
										"b description"
										b: Int
										"""
										c description
										"""
										c: Float
									): String
								}`, parse, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				person := doc.ObjectTypeDefinitions[0]
				personName := in.ByteSliceString(person.Name)
				if personName != "Person" {
					panic("want person")
				}

				name := doc.FieldDefinitions[person.FieldsDefinition.Refs[0]]
				if !name.Description.IsDefined {
					panic("want true")
				}
				if in.ByteSliceString(name.Description.Content) != "name description" {
					panic("want 'name description'")
				}

				// a

				a := doc.InputValueDefinitions[name.ArgumentsDefinition.Refs[0]]
				if in.ByteSliceString(a.Name) != "a" {
					panic("want a")
				}
				if doc.Types[a.Type].TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(doc.Types[doc.Types[a.Type].OfType].Name) != "String" {
					panic("want String")
				}

				// b

				b := doc.InputValueDefinitions[name.ArgumentsDefinition.Refs[1]]
				if in.ByteSliceString(b.Name) != "b" {
					panic("want b")
				}
				if in.ByteSliceString(b.Description.Content) != "b description" {
					panic("want 'b description'")
				}
				if doc.Types[b.Type].TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(doc.Types[b.Type].Name) != "Int" {
					panic("want Float")
				}

				// c

				c := doc.InputValueDefinitions[name.ArgumentsDefinition.Refs[2]]
				if in.ByteSliceString(c.Name) != "c" {
					panic("want b")
				}
				if !c.Description.IsDefined {
					panic("want true")
				}
				if !c.Description.IsBlockString {
					panic("want true")
				}
				if in.ByteSliceString(c.Description.Content) != `
										c description
										` {
					panic("want 'c description'")
				}
				if doc.Types[c.Type].TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(doc.Types[c.Type].Name) != "Float" {
					panic("want Float")
				}
			})
		})
		t.Run("implements & without next", func(t *testing.T) {
			run(`type Person implements Foo & {}`, parse, true)
		})
	})
	t.Run("input object type definition", func(t *testing.T) {
		t.Run("complex", func(t *testing.T) {
			run(`	input Person {
									name: String = "Gopher"
								}`, parse,
				false, func(in *input.Input, doc *ast.Document, extra interface{}) {
					person := doc.InputObjectTypeDefinitions[0]
					if in.ByteSliceString(person.Name) != "Person" {
						panic("want person")
					}

					name := doc.InputValueDefinitions[person.InputFieldsDefinition.Refs[0]]
					if in.ByteSliceString(name.Name) != "name" {
						panic("want name")
					}
					if !name.DefaultValue.IsDefined {
						panic("want true")
					}
					if name.DefaultValue.Value.Kind != ast.ValueKindString {
						panic("want ValueKindString")
					}
					if in.ByteSliceString(doc.StringValues[name.DefaultValue.Value.Ref].Content) != "Gopher" {
						panic("want Gopher")
					}
				})
		})
	})
	t.Run("interface type definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`interface NamedEntity @foo {
 								name: String
							}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					namedEntity := doc.InterfaceTypeDefinitions[0]
					if in.ByteSliceString(namedEntity.Name) != "NamedEntity" {
						panic("want NamedEntity")
					}

					// fields
					name := doc.FieldDefinitions[namedEntity.FieldsDefinition.Refs[0]]
					if in.ByteSliceString(name.Name) != "name" {
						panic("want name")
					}

					//directives
					foo := doc.Directives[namedEntity.Directives.Refs[0]]
					if in.ByteSliceString(foo.Name) != "foo" {
						panic("want foo")
					}
				})
		})
		t.Run("with description", func(t *testing.T) {
			run(`"describes NamedEntity" interface NamedEntity {
 								name: String
							}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					namedEntity := doc.InterfaceTypeDefinitions[0]
					if in.ByteSliceString(namedEntity.Name) != "NamedEntity" {
						panic("want NamedEntity")
					}
					if !namedEntity.Description.IsDefined {
						panic("want true")
					}
					if in.ByteSliceString(namedEntity.Description.Content) != "describes NamedEntity" {
						panic("want 'describes NamedEntity'")
					}
				})
		})
	})
	t.Run("union type definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`union SearchResult = Photo | Person`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					SearchResult := doc.UnionTypeDefinitions[0]

					// union member types

					// Photo
					Photo := doc.Types[SearchResult.UnionMemberTypes.Refs[0]]
					if Photo.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(Photo.Name) != "Photo" {
						panic("want Photo")
					}

					// Person
					Person := doc.Types[SearchResult.UnionMemberTypes.Refs[1]]
					if Person.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(Person.Name) != "Person" {
						panic("want Person")
					}

					// no more types
					if len(SearchResult.UnionMemberTypes.Refs) != 2 {
						panic("want 2")
					}
				})
		})
		t.Run("without members", func(t *testing.T) {
			run(`union SearchResult`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					SearchResult := doc.UnionTypeDefinitions[0]

					// union member types

					// no more types
					if len(SearchResult.UnionMemberTypes.Refs) != 0 {
						panic("want 0")
					}
				})
		})
		t.Run("member missing after pipe", func(t *testing.T) {
			run(`union SearchResult = Photo |`, parse, true)
		})
		t.Run("without members", func(t *testing.T) {
			run(`union SearchResult =`, parse, true)
		})
	})
	t.Run("type", func(t *testing.T) {
		t.Run("named", func(t *testing.T) {
			run("String", parseType, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				stringType := doc.Types[0]
				if stringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(stringType.Name) != "String" {
					panic("want String")
				}
			})
		})
		t.Run("non null named", func(t *testing.T) {
			run("String!", parseType, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				nonNull := doc.Types[1]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				stringType := doc.Types[nonNull.OfType]
				if stringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(stringType.Name) != "String" {
					panic("want String")
				}
			})
		})
		t.Run("non null list of named", func(t *testing.T) {
			run("[String]!", parseType, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				nonNull := doc.Types[2]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				list := doc.Types[nonNull.OfType]
				if list.TypeKind != ast.TypeKindList {
					panic("want TypeKindList")
				}
				stringType := doc.Types[list.OfType]
				if stringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(stringType.Name) != "String" {
					panic("want String")
				}
			})
		})
		t.Run("non null list of non null named", func(t *testing.T) {
			run("[String!]!", parseType, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				nonNull := doc.Types[3]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				list := doc.Types[nonNull.OfType]
				if list.TypeKind != ast.TypeKindList {
					panic("want TypeKindList")
				}
				nonNull = doc.Types[list.OfType]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				stringType := doc.Types[nonNull.OfType]
				if stringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(stringType.Name) != "String" {
					panic("want String")
				}
			})
		})
		t.Run("non null list of non null list of named", func(t *testing.T) {
			run("[[String]!]!", parseType, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				nonNull := doc.Types[4]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				list := doc.Types[nonNull.OfType]
				if list.TypeKind != ast.TypeKindList {
					panic("want TypeKindList")
				}
				nonNull = doc.Types[list.OfType]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				list = doc.Types[nonNull.OfType]
				if list.TypeKind != ast.TypeKindList {
					panic("want TypeKindList")
				}
				stringType := doc.Types[list.OfType]
				if stringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(stringType.Name) != "String" {
					panic("want String")
				}
			})
		})
		t.Run("non null list of non null list of non null named", func(t *testing.T) {
			run("[[String!]!]!", parseType, false, func(in *input.Input, doc *ast.Document, extra interface{}) {
				nonNull := doc.Types[5]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				list := doc.Types[nonNull.OfType]
				if list.TypeKind != ast.TypeKindList {
					panic("want TypeKindList")
				}
				nonNull = doc.Types[list.OfType]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				list = doc.Types[nonNull.OfType]
				if list.TypeKind != ast.TypeKindList {
					panic("want TypeKindList")
				}
				nonNull = doc.Types[list.OfType]
				if nonNull.TypeKind != ast.TypeKindNonNull {
					panic("want TypeKindNonNull")
				}
				stringType := doc.Types[nonNull.OfType]
				if stringType.TypeKind != ast.TypeKindNamed {
					panic("want TypeKindNamed")
				}
				if in.ByteSliceString(stringType.Name) != "String" {
					panic("want String")
				}
			})
		})
		t.Run("err unexpected bang", func(t *testing.T) {
			run("!", parseType, true)
		})
		t.Run("err empty list", func(t *testing.T) {
			run("[]", parseType, true)
		})
		t.Run("err incomplete list", func(t *testing.T) {
			run("[", parseType, true)
		})
		t.Run("err unclosed list", func(t *testing.T) {
			run("[String", parseType, true)
		})
		t.Run("err unclosed list with bang", func(t *testing.T) {
			run("[String!", parseType, true)
		})
		t.Run("err double bang", func(t *testing.T) {
			run("String!!", parseType, true)
		})
		t.Run("err list close at beginning", func(t *testing.T) {
			run("]String", parseType, true)
		})
	})
	t.Run("enum type definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`"enums"
							enum Direction @bar {
							  NORTH
							  EAST
							  SOUTH
							  "describes WEST"
							  WEST @foo
							}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					direction := doc.EnumTypeDefinitions[0]
					if in.ByteSliceString(direction.Name) != "Direction" {
						panic("want Direction")
					}

					// directives
					bar := doc.Directives[direction.Directives.Refs[0]]
					if in.ByteSliceString(bar.Name) != "bar" {
						panic("want bar")
					}

					// values

					wantValue := func(index int, name string) {
						enum := doc.EnumValueDefinitions[direction.EnumValuesDefinition.Refs[index]]
						if in.ByteSliceString(enum.EnumValue) != name {
							panic(fmt.Sprintf("want %s", name))
						}
					}

					wantValue(0, "NORTH")
					wantValue(1, "EAST")
					wantValue(2, "SOUTH")
					wantValue(3, "WEST")

					west := doc.EnumValueDefinitions[direction.EnumValuesDefinition.Refs[3]]
					if !west.Description.IsDefined {
						panic("want true")
					}
					if in.ByteSliceString(west.Description.Content) != "describes WEST" {
						panic("want describes WEST")
					}

					foo := doc.Directives[west.Directives.Refs[0]]
					if in.ByteSliceString(foo.Name) != "foo" {
						panic("want foo")
					}

					if len(direction.EnumValuesDefinition.Refs) != 4 {
						panic("want 4")
					}
				})
		})
	})
	t.Run("directive definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`directive @example on FIELD`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					example := doc.DirectiveDefinitions[0]
					if in.ByteSliceString(example.Name) != "example" {
						panic("want example")
					}
					locations := example.DirectiveLocations.Iterable()
					if !locations.Next() {
						panic("want next")
					}
					if locations.Value() != ast.ExecutableDirectiveLocationField {
						panic("want ExecutableDirectiveLocationField")
					}
					if locations.Next() {
						panic("want false")
					}
				})
		})
		t.Run("multiple directive locations", func(t *testing.T) {
			run(`directive @example on FIELD | SCALAR | SCHEMA`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					example := doc.DirectiveDefinitions[0]
					if in.ByteSliceString(example.Name) != "example" {
						panic("want example")
					}
					locations := example.DirectiveLocations.Iterable()
					if !locations.Next() {
						panic("want next")
					}
					if locations.Value() != ast.ExecutableDirectiveLocationField {
						panic("want ExecutableDirectiveLocationField")
					}
					if !locations.Next() {
						panic("want next")
					}
					if locations.Value() != ast.TypeSystemDirectiveLocationSchema {
						panic("want TypeSystemDirectiveLocationSchema")
					}
					if !locations.Next() {
						panic("want next")
					}
					if locations.Value() != ast.TypeSystemDirectiveLocationScalar {
						panic("want TypeSystemDirectiveLocationScalar")
					}
					if locations.Next() {
						panic("want false")
					}
				})
		})
		t.Run("err pipe at end", func(t *testing.T) {
			run(`directive @example on FIELD | SCALAR | SCHEMA |`, parse, true)
		})
		t.Run("missing location", func(t *testing.T) {
			run(`directive @example on`, parse, true)
		})
		t.Run("invalid location", func(t *testing.T) {
			run(`directive @example on INVALID`, parse, true)
		})
	})
	t.Run("value", func(t *testing.T) {
		t.Run("variable value", func(t *testing.T) {
			t.Run("simple", func(t *testing.T) {
				run(`$foo`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindVariable {
							t.Fatal("want ValueKindVariable")
						}
						foo := doc.VariableValues[value.Ref]
						if in.ByteSliceString(foo.Name) != "foo" {
							t.Fatal("want foo")
						}
					})
			})
			t.Run("with underscore", func(t *testing.T) {
				run(`$_foo`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindVariable {
							t.Fatal("want ValueKindVariable")
						}
						foo := doc.VariableValues[value.Ref]
						if in.ByteSliceString(foo.Name) != "_foo" {
							t.Fatal("want foo")
						}
					})
			})
			t.Run("with numbers", func(t *testing.T) {
				run(`$foo123`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindVariable {
							t.Fatal("want ValueKindVariable")
						}
						foo := doc.VariableValues[value.Ref]
						if in.ByteSliceString(foo.Name) != "foo123" {
							t.Fatal("want foo123")
						}
					})
			})
			t.Run("err space", func(t *testing.T) {
				run(`$ foo`, parseValue, true)
			})
			t.Run("err start with A-Za-z", func(t *testing.T) {
				run(`$123`, parseValue, true)
			})
		})
		t.Run("int value", func(t *testing.T) {
			t.Run("simple", func(t *testing.T) {
				run(`123`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindInteger {
							panic("want ValueKindInteger")
						}
						intValue := doc.IntValues[value.Ref]
						if in.ByteSliceString(intValue.Raw) != "123" {
							panic("want 123")
						}
						if intValue.Negative {
							panic("want false")
						}
					})
			})
			t.Run("negative", func(t *testing.T) {
				run(`-123`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindInteger {
							panic("want ValueKindInteger")
						}
						intValue := doc.IntValues[value.Ref]
						if in.ByteSliceString(intValue.Raw) != "123" {
							panic("want 123")
						}
						if !intValue.Negative {
							panic("want false")
						}
					})
			})
			t.Run("err space after negative sign", func(t *testing.T) {
				run(`- 123`, parseValue, true)
			})
		})
		t.Run("float value", func(t *testing.T) {
			t.Run("simple", func(t *testing.T) {
				run(`13.37`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindFloat {
							panic("want ValueKindFloat")
						}
						intValue := doc.FloatValues[value.Ref]
						if in.ByteSliceString(intValue.Raw) != "13.37" {
							panic("want 13.37")
						}
						if intValue.Negative {
							panic("want false")
						}
					})
			})
			t.Run("negative", func(t *testing.T) {
				run(`-13.37`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindFloat {
							panic("want ValueKindFloat")
						}
						intValue := doc.FloatValues[value.Ref]
						if in.ByteSliceString(intValue.Raw) != "13.37" {
							panic("want 13.37")
						}
						if !intValue.Negative {
							panic("want false")
						}
					})
			})
			t.Run("err space after negative sign", func(t *testing.T) {
				run(`- 13.37`, parseValue, true)
			})
		})
		t.Run("null value", func(t *testing.T) {
			run(`null`, parseValue, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					value := extra.(ast.Value)
					if value.Kind != ast.ValueKindNull {
						panic("want ValueKindNull")
					}
				})
		})
		t.Run("list value", func(t *testing.T) {
			t.Run("complex", func(t *testing.T) {
				run(`[1,2,"3",[4]]`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindList {
							panic("want ValueKindList")
						}

						list := doc.ListValues[value.Ref]

						// 1
						val := doc.Values[list.Refs[0]]
						if val.Kind != ast.ValueKindInteger {
							panic("want ValueKindInteger")
						}
						if in.ByteSliceString(doc.IntValues[val.Ref].Raw) != "1" {
							panic("want 1")
						}

						// 2
						val = doc.Values[list.Refs[1]]
						if val.Kind != ast.ValueKindInteger {
							panic("want ValueKindInteger")
						}
						if in.ByteSliceString(doc.IntValues[val.Ref].Raw) != "2" {
							panic("want 1")
						}

						// "3"
						val = doc.Values[list.Refs[2]]
						if val.Kind != ast.ValueKindString {
							panic("want ValueKindString")
						}
						if in.ByteSliceString(doc.StringValues[val.Ref].Content) != "3" {
							panic("want 3")
						}

						// [4]
						val = doc.Values[list.Refs[3]]
						if val.Kind != ast.ValueKindList {
							panic("want ValueKindString")
						}
						inner := doc.ListValues[val.Ref]

						four := doc.Values[inner.Refs[0]]
						if four.Kind != ast.ValueKindInteger {
							panic("want ValueKindInteger")
						}
						if in.ByteSliceString(doc.IntValues[four.Ref].Raw) != "4" {
							panic("want 4")
						}
						if len(inner.Refs) != 1 {
							panic("want 1")
						}

						// no more
						if len(list.Refs) != 4 {
							panic("want 4")
						}
					})
			})
		})
		t.Run("object value", func(t *testing.T) {
			t.Run("complex", func(t *testing.T) {
				run(`{lon: 12.43, lat: -53.211, list: [1] }`, parseValue, false,
					func(in *input.Input, doc *ast.Document, extra interface{}) {
						value := extra.(ast.Value)
						if value.Kind != ast.ValueKindObject {
							panic("want ValueKindObject")
						}
						object := doc.ObjectValues[value.Ref]

						// lon
						lon := doc.ObjectFields[object.Refs[0]]
						if in.ByteSliceString(lon.Name) != "lon" {
							panic("want lon")
						}
						if lon.Value.Kind != ast.ValueKindFloat {
							panic("want float")
						}
						if in.ByteSliceString(doc.FloatValues[lon.Value.Ref].Raw) != "12.43" {
							panic("want 12.43")
						}

						// lat
						lat := doc.ObjectFields[object.Refs[1]]
						if in.ByteSliceString(lat.Name) != "lat" {
							panic("want lat")
						}
						if lon.Value.Kind != ast.ValueKindFloat {
							panic("want float")
						}
						if !doc.FloatValues[lat.Value.Ref].Negative {
							panic("want negative")
						}
						if in.ByteSliceString(doc.FloatValues[lat.Value.Ref].Raw) != "53.211" {
							panic("want 53.211")
						}

						// list
						list := doc.ObjectFields[object.Refs[2]]
						if list.Value.Kind != ast.ValueKindList {
							panic("want ValueKindList")
						}
						listValue := doc.ListValues[list.Value.Ref]
						one := doc.Values[listValue.Refs[0]]
						if in.ByteSliceString(doc.IntValues[one.Ref].Raw) != "1" {
							panic("want 1")
						}
						if len(listValue.Refs) != 1 {
							panic("want 1")
						}

						if len(object.Refs) != 3 {
							panic("want 3")
						}
					})
			})
		})
	})
	t.Run("operation definition", func(t *testing.T) {
		t.Run("unnamed query", func(t *testing.T) {
			run(`query {field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					query := doc.OperationDefinitions[0]
					if query.OperationType != ast.OperationTypeQuery {
						panic("want OperationTypeQuery")
					}
					if in.ByteSliceString(query.Name) != "" {
						panic("want empty string")
					}
					fieldSelection := doc.Selections[query.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("shorthand query", func(t *testing.T) {
			run(`{field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					query := doc.OperationDefinitions[0]
					if query.OperationType != ast.OperationTypeQuery {
						panic("want OperationTypeQuery")
					}
					if in.ByteSliceString(query.Name) != "" {
						panic("want empty string")
					}
					fieldSelection := doc.Selections[query.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("named query", func(t *testing.T) {
			run(`query Query1 {field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					query := doc.OperationDefinitions[0]
					if query.OperationType != ast.OperationTypeQuery {
						panic("want OperationTypeQuery")
					}
					if in.ByteSliceString(query.Name) != "Query1" {
						panic("want Query1")
					}
					fieldSelection := doc.Selections[query.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("unnamed mutation", func(t *testing.T) {
			run(`mutation {field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					mutation := doc.OperationDefinitions[0]
					if mutation.OperationType != ast.OperationTypeMutation {
						panic("want OperationTypeMutation")
					}
					if in.ByteSliceString(mutation.Name) != "" {
						panic("want empty string")
					}
					fieldSelection := doc.Selections[mutation.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("named mutation", func(t *testing.T) {
			run(`mutation Mutation1 {field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					mutation := doc.OperationDefinitions[0]
					if mutation.OperationType != ast.OperationTypeMutation {
						panic("want OperationTypeMutation")
					}
					if in.ByteSliceString(mutation.Name) != "Mutation1" {
						panic("want Mutation1")
					}
					fieldSelection := doc.Selections[mutation.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("unnamed subscription", func(t *testing.T) {
			run(`subscription {field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					mutation := doc.OperationDefinitions[0]
					if mutation.OperationType != ast.OperationTypeSubscription {
						panic("want OperationTypeSubscription")
					}
					if in.ByteSliceString(mutation.Name) != "" {
						panic("want empty string")
					}
					fieldSelection := doc.Selections[mutation.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("named subscription", func(t *testing.T) {
			run(`subscription Sub1 {field}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					mutation := doc.OperationDefinitions[0]
					if mutation.OperationType != ast.OperationTypeSubscription {
						panic("want OperationTypeSubscription")
					}
					if in.ByteSliceString(mutation.Name) != "Sub1" {
						panic("want empty Sub1")
					}
					fieldSelection := doc.Selections[mutation.SelectionSet.SelectionRefs[0]]
					if fieldSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					field := doc.Fields[fieldSelection.Ref]
					if in.ByteSliceString(field.Name) != "field" {
						panic("want field")
					}
				})
		})
		t.Run("complex nested subscription", func(t *testing.T) {
			run(`subscription StoryLikeSubscription($input: StoryLikeSubscribeInput) {
  								storyLikeSubscribe(input: $input) {
									story {
									  likers {
										count
									  }
									  likeSentence {
										text
									  }
									}
								  }
							}`,
				parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					subscription := doc.OperationDefinitions[0]
					if subscription.OperationType != ast.OperationTypeSubscription {
						panic("want OperationTypeSubscription")
					}
				})
		})
	})
	t.Run("variable definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`($devicePicSize: Int = 1 $var2: String)`, parseVariableDefinitionList, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					list := extra.(ast.VariableDefinitionList)

					var1 := doc.VariableDefinitions[list.Refs[0]]
					devicePicSize := doc.VariableValues[var1.Variable]
					if in.ByteSliceString(devicePicSize.Name) != "devicePicSize" {
						panic("want devicePicSize")
					}
					Int := doc.Types[var1.Type]
					if Int.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(Int.Name) != "Int" {
						panic("want Int")
					}
					if !var1.DefaultValue.IsDefined {
						panic("want true")
					}
					if var1.DefaultValue.Value.Kind != ast.ValueKindInteger {
						panic("want ValueKindInteger")
					}
					one := doc.IntValues[var1.DefaultValue.Value.Ref]
					if in.ByteSliceString(one.Raw) != "1" {
						panic("want 1")
					}

					var2 := doc.VariableDefinitions[list.Refs[1]]
					var2Variable := doc.VariableValues[var2.Variable]
					if in.ByteSliceString(var2Variable.Name) != "var2" {
						panic("want var2")
					}
					String := doc.Types[var2.Type]
					if String.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(String.Name) != "String" {
						panic("want String")
					}

					if len(list.Refs) != 2 {
						panic("want 2")
					}
				})
		})
	})
	t.Run("selection set", func(t *testing.T) {
		t.Run("No 8", func(t *testing.T) {
			run(`{
							  me {
								... on Person @foo {
									personID
								}
								...personFragment @bar
								id
								firstName
								lastName
								birthday {
								  month
								  day
								}
								friends {
								  name
								}
							  }
							}`, parseSelectionSet, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					set := extra.(ast.SelectionSet)

					// me
					meSelection := doc.Selections[set.SelectionRefs[0]]
					if meSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					me := doc.Fields[meSelection.Ref]
					if in.ByteSliceString(me.Name) != "me" {
						panic("want me")
					}

					// ... on Person

					onPersonSelection := doc.Selections[me.SelectionSet.SelectionRefs[0]]
					if onPersonSelection.Kind != ast.SelectionKindInlineFragment {
						panic("want SelectionKindInlineFragment")
					}
					onPersonFragment := doc.InlineFragments[onPersonSelection.Ref]
					if len(onPersonFragment.Directives.Refs) != 1 {
						panic("want 1")
					}
					Person := doc.Types[onPersonFragment.TypeCondition.Type]
					if in.ByteSliceString(Person.Name) != "Person" {
						panic("want Person")
					}
					personIdSelection := doc.Selections[onPersonFragment.SelectionSet.SelectionRefs[0]]
					if personIdSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					personId := doc.Fields[personIdSelection.Ref]
					if in.ByteSliceString(personId.Name) != "personID" {
						panic("want personID")
					}

					// ...personFragment
					personFragmentSelection := doc.Selections[me.SelectionSet.SelectionRefs[1]]
					if personFragmentSelection.Kind != ast.SelectionKindFragmentSpread {
						panic("want SelectionKindFragmentSpread")
					}
					personFragment := doc.FragmentSpreads[personFragmentSelection.Ref]
					if in.ByteSliceString(personFragment.FragmentName) != "personFragment" {
						panic("want personFragment")
					}
					if len(personFragment.Directives.Refs) != 1 {
						panic("want 1")
					}

					// id
					idSelection := doc.Selections[me.SelectionSet.SelectionRefs[2]]
					if idSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					id := doc.Fields[idSelection.Ref]
					if in.ByteSliceString(id.Name) != "id" {
						panic("want id")
					}

					// birthday
					birthdaySelection := doc.Selections[me.SelectionSet.SelectionRefs[5]]
					if birthdaySelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					birthday := doc.Fields[birthdaySelection.Ref]
					if in.ByteSliceString(birthday.Name) != "birthday" {
						panic("want birthday")
					}

					// month
					monthSelection := doc.Selections[birthday.SelectionSet.SelectionRefs[0]]
					if monthSelection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					month := doc.Fields[monthSelection.Ref]
					if in.ByteSliceString(month.Name) != "month" {
						panic("want month")
					}
				})
		})
	})
	t.Run("fragment spread", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`friendFields @foo`, parseFragmentSpread, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					fragmentSpread := extra.(ast.FragmentSpread)
					if in.ByteSliceString(fragmentSpread.FragmentName) != "friendFields" {
						panic("want friendFields")
					}
					if len(fragmentSpread.Directives.Refs) != 1 {
						panic("want 1")
					}
				})
		})
		t.Run("err fragment name must not be on", func(t *testing.T) {
			run(`on`, parseFragmentSpread, true)
		})
	})
	t.Run("inline fragment", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`on User {
							  friends {
								count
							  }
							}`, parseInlineFragment, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					fragment := extra.(ast.InlineFragment)
					user := doc.Types[fragment.TypeCondition.Type]
					if user.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(user.Name) != "User" {
						panic("want User")
					}

					selection := doc.Selections[fragment.SelectionSet.SelectionRefs[0]]
					if selection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					friends := doc.Fields[selection.Ref]
					if in.ByteSliceString(friends.Name) != "friends" {
						panic("want friends")
					}

					selection = doc.Selections[friends.SelectionSet.SelectionRefs[0]]
					if selection.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					count := doc.Fields[selection.Ref]
					if in.ByteSliceString(count.Name) != "count" {
						panic("want count")
					}
				})
		})
	})
	t.Run("fragment definition", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			run(`fragment friendFields on User {
							  id
							  name
							  profilePic(size: 50)
							}`, parse, false,
				func(in *input.Input, doc *ast.Document, extra interface{}) {
					fragment := doc.FragmentDefinitions[0]
					if in.ByteSliceString(fragment.Name) != "friendFields" {
						panic("want friendFields")
					}
					onUser := doc.Types[fragment.TypeCondition.Type]
					if onUser.TypeKind != ast.TypeKindNamed {
						panic("want TypeKindNamed")
					}
					if in.ByteSliceString(onUser.Name) != "User" {
						panic("want User")
					}

					selection1 := doc.Selections[fragment.SelectionSet.SelectionRefs[0]]
					if selection1.Kind != ast.SelectionKindField {
						panic("want SelectionKindField")
					}
					id := doc.Fields[selection1.Ref]
					if in.ByteSliceString(id.Name) != "id" {
						panic("want id")
					}
				})
		})
	})
}

func TestParseStarwars(t *testing.T) {

	inputFileName := "./testdata/starwars.schema.graphql"
	starwarsSchema, err := ioutil.ReadFile(inputFileName)
	if err != nil {
		t.Fatal(err)
	}

	in := &input.Input{}
	in.ResetInputBytes(starwarsSchema)
	doc := &ast.Document{}

	parser := NewParser()

	err = parser.Parse(in, doc)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkParseStarwars(b *testing.B) {

	inputFileName := "./testdata/starwars.schema.graphql"
	starwarsSchema, err := ioutil.ReadFile(inputFileName)
	if err != nil {
		b.Fatal(err)
	}

	in := &input.Input{}
	in.ResetInputBytes(starwarsSchema)

	doc := ast.NewDocument()

	parser := NewParser()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in.ResetInputBytes(starwarsSchema)
		doc.Reset()
		err = parser.Parse(in, doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectionSet(b *testing.B) {
	in := &input.Input{}
	doc := ast.NewDocument()
	parser := NewParser()

	var err error

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in.ResetInputBytes(selectionSet)
		doc.Reset()
		err = parser.Parse(in, doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIntrospectionQuery(b *testing.B) {
	in := &input.Input{}
	doc := ast.NewDocument()
	parser := NewParser()

	var err error

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in.ResetInputBytes(introspectionQuery)
		doc.Reset()
		err = parser.Parse(in, doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKitchenSink(b *testing.B) {
	in := &input.Input{}
	doc := ast.NewDocument()
	parser := NewParser()

	var err error

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in.ResetInputBytes(kitchenSinkData)
		doc.Reset()
		err = parser.Parse(in, doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKitchenSinkFastest(b *testing.B) {
	in := &input.Input{}
	lex := &lexer.Lexer{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in.ResetInputBytes(kitchenSinkData)
		lex.SetInput(in)
		for i := 0; i <= 174; i++ {
			lex.Read()
		}
	}
}

func BenchmarkParse(b *testing.B) {

	in := &input.Input{}
	doc := ast.NewDocument()
	parser := NewParser()

	var err error

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		in.ResetInputBytes(inputBytes)
		doc.Reset()
		err = parser.Parse(in, doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

var selectionSet = []byte(`{
							  me {
								... on Person @foo {
									personID
								}
								...personFragment @bar
								id
								firstName
								lastName
								birthday {
								  month
								  day
								}
								friends {
								  name
								}
							  }
							}`)

var inputBytes = []byte(`	schema @foo @bar(baz: "bal") {
								query: Query
								mutation: Mutation
								subscription: Subscription 
							}

							"""
							Person type description
							"""
							type Person implements Foo & Bar {
								name: String
								"age of the person"
								age: Int
								"""
								date of birth
								"""
								dateOfBirth: Date
							}

							type PersonWithArgs { 
								"name description"
								name(
									a: String!
									"b description"
									b: Int
									"""
									c description
									"""
									c: Float
								): String
							}

							"scalars"
							scalar JSON

							"inputs"
							input Person {
								name: String = "Gopher"
							}

							"unions"
							union SearchResult = Photo | Person

							"interfaces"
							interface NamedEntity @foo {
 								name: String
							}

							"enums"
							enum Direction {
							  NORTH
							  EAST
							  SOUTH
							  WEST
							}

							"directives"
							directive @example on FIELD | SCALAR | SCHEMA

							query MyQuery {
							  me {
								... on Person @foo {
									personID
								}
								...personFragment @bar
								id
								firstName
								lastName
								birthday {
								  month
								  day
								}
								friends {
								  name
								}
							  }
							}
`)

var kitchenSinkData = []byte(`# Copyright (c) 2015-present, Facebook, Inc.
#
# This source code is licensed under the MIT license found in the
# LICENSE file in the root directory of this source tree.

query queryName($foo: ComplexType, $site: Site = MOBILE) {
  whoever123is: node(id: [123, 456]) {
    id ,
    ... on User @defer {
      field2 {
        id ,
        alias: field1(first:10, after:$foo,) @include(if: $foo) {
          id,
          ...frag
        }
      }
    }
    ... @skip(unless: $foo) {
      id
    }
    ... {
      id
    }
  }
}

mutation likeStory {
  like(story: 123) @defer {
    story {
      id
    }
  }
}

subscription StoryLikeSubscription($input: StoryLikeSubscribeInput) {
  storyLikeSubscribe(input: $input) {
    story {
      likers {
        count
      }
      likeSentence {
        text
      }
    }
  }
}

fragment frag on Friend {
  foo(size: $size, bar: $b, obj: {key: "value", block: """

      block string uses \"""

  """})
}

{
  unnamed(truthy: true, falsey: false, nullish: null),
  query
}
`)

var introspectionQuery = []byte(`query IntrospectionQuery {
  __schema {
    queryType {
      name
    }
    mutationType {
      name
    }
    subscriptionType {
      name
    }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type {
    ...TypeRef
  }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
              }
            }
          }
        }
      }
    }
  }
}`)
