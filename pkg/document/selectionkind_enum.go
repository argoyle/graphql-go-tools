// Code generated by go-enum
// DO NOT EDIT!

package document

import (
	"fmt"
)

const (
	// SelectionKindField is a SelectionKind of type Field
	SelectionKindField SelectionKind = iota
	// SelectionKindInlineFragment is a SelectionKind of type InlineFragment
	SelectionKindInlineFragment
	// SelectionKindFragmentSpread is a SelectionKind of type FragmentSpread
	SelectionKindFragmentSpread
)

const _SelectionKindName = "FieldInlineFragmentFragmentSpread"

var _SelectionKindMap = map[SelectionKind]string{
	0: _SelectionKindName[0:5],
	1: _SelectionKindName[5:19],
	2: _SelectionKindName[19:33],
}

// String implements the Stringer interface.
func (x SelectionKind) String() string {
	if str, ok := _SelectionKindMap[x]; ok {
		return str
	}
	return fmt.Sprintf("SelectionKind(%d)", x)
}

var _SelectionKindValue = map[string]SelectionKind{
	_SelectionKindName[0:5]:   0,
	_SelectionKindName[5:19]:  1,
	_SelectionKindName[19:33]: 2,
}

// ParseSelectionKind attempts to convert a string to a SelectionKind
func ParseSelectionKind(name string) (SelectionKind, error) {
	if x, ok := _SelectionKindValue[name]; ok {
		return x, nil
	}
	return SelectionKind(0), fmt.Errorf("%s is not a valid SelectionKind", name)
}