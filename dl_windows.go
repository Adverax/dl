// +build windows

package dl

import "C"

import (
	"errors"
	"fmt"
	"github.com/adverax/echo/generic"
	"math"
	"reflect"
	"sync"
	"syscall"
	"unsafe"
)

type library struct {
	sync.Mutex
	handle   syscall.Handle
	routines map[string]*Routine
}

func (lib *library) Close() error {
	if lib.handle != 0 {
		lib.Lock()
		defer lib.Unlock()

		if lib.handle != 0 {
			mu.Lock()
			defer mu.Unlock()

			if err := syscall.FreeLibrary(lib.handle); err != nil {
				return fmt.Errorf("close library^ %w", err)
			}
			lib.handle = 0
		}
	}

	return nil
}

func (lib *library) Define(routine *Routine) error {
	lib.Lock()
	defer lib.Unlock()

	lib.routines[routine.Name] = routine
	address, err := syscall.GetProcAddress(syscall.Handle(lib.handle), routine.Name)
	if err != nil {
		return fmt.Errorf("library define: %w", err)
	}

	routine.address = address

	return nil
}

func (lib *library) Symbol(name string, out interface{}) error {
	// Not yet implemented
	return nil
}

func (lib *library) Call(name string, arguments ...interface{}) (res interface{}, err error) {
	// Find function
	routine, err := lib.find(name)
	if err != nil {
		return 0, fmt.Errorf("call: %w", err)
	}

	// Prepare arguments
	count := len(routine.Args)
	args := make([]uintptr, count)
	if count > 0 {
		if len(arguments) < count {
			return false, fmt.Errorf("call: %w", fmt.Errorf("Too few arguments in func %s", routine.Name))
		}

		for ii, arg := range routine.Args {
			val := MakeValue(arg.Type, arg.Pointer)
			err = generic.ConvertAssign(&val, arguments[ii])
			if err != nil {
				return nil, fmt.Errorf("call: %w", err)
			}
			v := reflect.ValueOf(val)
			if v.Type() == emptyType {
				v = reflect.ValueOf(v.Interface())
			}

			switch v.Kind() {
			case reflect.String:
				args[ii] = uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(v.String())))
			case reflect.Int:
				args[ii] = uintptr(v.Int())
			case reflect.Int8:
				args[ii] = uintptr(v.Int())
			case reflect.Int16:
				args[ii] = uintptr(v.Int())
			case reflect.Int32:
				args[ii] = uintptr(v.Int())
			case reflect.Int64:
				args[ii] = uintptr(v.Int())
			case reflect.Uint:
				args[ii] = uintptr(v.Uint())
			case reflect.Uint8:
				args[ii] = uintptr(v.Uint())
			case reflect.Uint16:
				args[ii] = uintptr(v.Uint())
			case reflect.Uint32:
				args[ii] = uintptr(v.Uint())
			case reflect.Uint64:
				args[ii] = uintptr(v.Uint())
			case reflect.Float32:
				args[ii] = uintptr(math.Float32bits(float32(v.Float())))
			case reflect.Float64:
				args[ii] = uintptr(math.Float64bits(v.Float()))
			case reflect.Ptr:
				args[ii] = uintptr(unsafe.Pointer(v.Pointer()))
			case reflect.Slice:
				if v.Len() > 0 {
					args[ii] = uintptr(unsafe.Pointer(v.Index(0).UnsafeAddr()))
				}
			case reflect.Uintptr:
				args[ii] = uintptr(v.Uint())
			default:
				return false, fmt.Errorf("call: %w", fmt.Errorf("can't bind value of type %s", v.Type()))
			}

		}
		if err != nil {
			return false, err
		}
	}

	// Call function
	var val uintptr
	var errno syscall.Errno
	switch len(args) {
	case 0:
		val, _, errno = syscall.Syscall(routine.address, 0, 0, 0, 0)
	case 1:
		val, _, errno = syscall.Syscall(routine.address, 1, args[0], 0, 0)
	case 2:
		val, _, errno = syscall.Syscall(routine.address, 2, args[0], args[1], 0)
	case 3:
		val, _, errno = syscall.Syscall(routine.address, 3, args[0], args[1], args[2])
	case 4:
		val, _, errno = syscall.Syscall6(routine.address, 4, args[0], args[1], args[2], args[3], 0, 0)
	case 5:
		val, _, errno = syscall.Syscall6(routine.address, 5, args[0], args[1], args[2], args[3], args[4], 0)
	case 6:
		val, _, errno = syscall.Syscall6(routine.address, 6, args[0], args[1], args[2], args[3], args[4], args[5])
	case 7:
		val, _, errno = syscall.Syscall9(routine.address, 7, args[0], args[1], args[2], args[3], args[4], args[5], args[6], 0, 0)
	case 8:
		val, _, errno = syscall.Syscall9(routine.address, 8, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], 0)
	case 9:
		val, _, errno = syscall.Syscall9(routine.address, 9, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8])
	case 10:
		val, _, errno = syscall.Syscall12(routine.address, 10, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], 0, 0)
	case 11:
		val, _, errno = syscall.Syscall12(routine.address, 11, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], 0)
	case 12:
		val, _, errno = syscall.Syscall12(routine.address, 12, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11])
	case 13:
		val, _, errno = syscall.Syscall15(routine.address, 13, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], 0, 0)
	case 14:
		val, _, errno = syscall.Syscall15(routine.address, 14, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], args[13], 0)
	case 15:
		val, _, errno = syscall.Syscall15(routine.address, 15, args[0], args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8], args[9], args[10], args[11], args[12], args[13], args[14])
	default:
		return 0, fmt.Errorf("call: %w", errors.New("too many arguments"))
	}

	if errno != 0 {
		return 0, fmt.Errorf("call: %w", errno)
	}

	// Handle result
	if routine.Result == nil || routine.Result.Type == reflect.Invalid {
		return
	}

	var v reflect.Value
	vv := MakeValue(routine.Result.Type, routine.Result.Pointer)
	out := reflect.TypeOf(vv)

	switch out.Kind() {
	case reflect.Int:
		v = reflect.ValueOf(int(val))
	case reflect.Int8:
		v = reflect.ValueOf(int8(val))
	case reflect.Int16:
		v = reflect.ValueOf(int16(val))
	case reflect.Int32:
		v = reflect.ValueOf(int32(val))
	case reflect.Int64:
		v = reflect.ValueOf(int64(val))
	case reflect.Uint:
		v = reflect.ValueOf(uint(val))
	case reflect.Uint8:
		v = reflect.ValueOf(uint8(val))
	case reflect.Uint16:
		v = reflect.ValueOf(uint16(val))
	case reflect.Uint32:
		v = reflect.ValueOf(uint32(val))
	case reflect.Uint64:
		v = reflect.ValueOf(uint64(val))
	case reflect.Float32:
		v = reflect.ValueOf(math.Float32frombits(uint32(val)))
	case reflect.Float64:
		v = reflect.ValueOf(math.Float64frombits(uint64(val)))
	case reflect.Ptr:
		if out.Elem().Kind() == reflect.String && val != 0 {
			s := C.GoString((*C.char)(unsafe.Pointer(val)))
			v = reflect.ValueOf(&s)
			break
		}
		v = reflect.NewAt(out.Elem(), unsafe.Pointer(val))
	case reflect.String:
		s := C.GoString((*C.char)(unsafe.Pointer(val)))
		v = reflect.ValueOf(s)
	case reflect.Uintptr:
		v = reflect.ValueOf(uintptr(val))
	case reflect.UnsafePointer:
		v = reflect.ValueOf(val)
	default:
		return v, fmt.Errorf("call: %w", fmt.Errorf("can't retrieve value of type"))
	}

	return v.Interface(), nil
}

func (lib *library) find(name string) (*Routine, error) {
	lib.Lock()
	defer lib.Unlock()

	if routine, ok := lib.routines[name]; ok {
		return routine, nil
	}

	return nil, fmt.Errorf("call: %w", fmt.Errorf("Function %q not found", name))
}

func Open(name string, flag int) (Library, error) {
	handle, err := syscall.LoadLibrary(name)
	if err != nil {
		return nil, fmt.Errorf("open library: %w", err)
	}

	return &library{
		handle:   handle,
		routines: make(map[string]*Routine),
	}, nil
}
