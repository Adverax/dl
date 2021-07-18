package dl

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestParseRoutineDefinition(t *testing.T) {
	type Test struct {
		src string
		dst *Routine
		err error
	}

	tests := map[string]Test{
		"Invalid type": {
			src: "xxx print()",
			dst: nil,
			err: fmt.Errorf("Unknown type %q", "xxx"),
		},
		"Error in argument type": {
			src: "void print(abc)",
			dst: nil,
			err: fmt.Errorf("Error in %d argument", 0),
		},
		"Empty func": {
			src: "void print()",
			dst: &Routine{
				Name: "print",
				Args: []*Arg{},
			},
		},
		"Complex func": {
			src: "int size(string disk, int64 *value)",
			dst: &Routine{
				Name:   "size",
				Result: &Arg{Type: reflect.Int},
				Args: []*Arg{
					{Type: reflect.String},
					{Type: reflect.Int64, Pointer: true},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := ParseRoutineDefinition(test.src)
			require.Equal(t, test.err, err)
			assert.Equal(t, test.dst, actual)
		})
	}
}
