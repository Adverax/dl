package dl

import (
	"github.com/adverax/echo/generic"
	"reflect"
	"unsafe"
)

func MakeValue(kind reflect.Kind, pointer bool) interface{} {
	var v interface{}

	switch kind {
	case reflect.Bool:
		v = false
	case reflect.Int:
		v = int(0)
	case reflect.Int8:
		v = int8(0)
	case reflect.Int16:
		v = int16(0)
	case reflect.Int32:
		v = int32(0)
	case reflect.Int64:
		v = int64(0)
	case reflect.Uint:
		v = uint(0)
	case reflect.Uint8:
		v = uint8(0)
	case reflect.Uint16:
		v = uint16(0)
	case reflect.Uint32:
		v = uint32(0)
	case reflect.Uint64:
		v = uint64(0)
	case reflect.Uintptr:
		v = uintptr(0)
	case reflect.Float32:
		v = float32(0)
	case reflect.Float64:
		v = float64(0)
	case reflect.Complex64:
		v = complex(0, 0)
	case reflect.Ptr:
		v = unsafe.Pointer(nil)
	case reflect.String:
		v = ""
	case reflect.UnsafePointer:
		v = unsafe.Pointer(nil)
	default:
		v = false
	}

	if pointer {
		return generic.MakePointerTo(v)
	}

	return v
}
