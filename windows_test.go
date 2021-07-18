// +build windows

package dl

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestCall(t *testing.T) {
	lib, err := Open("Kernel32.dll", 0)
	require.NoError(t, err)
	defer Close()

	err = Define(&Routine{
		Name:   "GetDiskFreeSpaceExW",
		Result: nil,
		Args: []*Arg{
			{Type: reflect.String},
			{Type: reflect.Int64, Pointer: true},
			{Type: reflect.Int64, Pointer: true},
			{Type: reflect.Int64, Pointer: true},
		},
	})
	require.NoError(t, err)

	var FreeBytesAvailable int64
	var TotalNumberOfBytes int64
	var TotalNumberOfFreeBytes int64

	_, err = Call("GetDiskFreeSpaceExW", "C:", &FreeBytesAvailable, &TotalNumberOfBytes, &TotalNumberOfFreeBytes)
	require.NoError(t, err)

	assert.True(t, FreeBytesAvailable != 0)
	assert.True(t, TotalNumberOfBytes != 0)
	assert.True(t, TotalNumberOfFreeBytes != 0)
}
