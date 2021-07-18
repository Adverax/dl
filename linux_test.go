// +build linux

package dl

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestCall(t *testing.T) {
	lib, err := Open("libc", 0)
	require.NoError(t, err)
	defer Close()

	err = Define(&Routine{
		Name:   "strlen",
		Result: &Arg{Type: reflect.Int},
		Args: []*Arg{
			{Type: reflect.String},
		},
	})
	require.NoError(t, err)

	l, err := Call("strlen", "this")
	require.NoError(t, err)

	assert.Equal(t, int(4), l.(int))
}
