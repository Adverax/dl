// +build linux

package dl

import "C"

/*#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

enum {
    ARG_FLAG_SIZE_8 = 1 << 0,
    ARG_FLAG_SIZE_16 = 1 << 1,
    ARG_FLAG_SIZE_32 = 1 << 2,
    ARG_FLAG_SIZE_64 = 1 << 3,
    ARG_FLAG_SIZE_PTR = 1 << 4,
    ARG_FLAG_FLOAT = 1 << 5,
};

extern int call(void *f, void **args, int *flags, int count, void **out);

#define MAX_STACK_COUNT 100
#define MAX_INTEGER_COUNT (6)
#define MAX_FLOAT_COUNT (8)

#define _xstr(s) _str(s)
#define _str(s) #s

extern void * make_call(void *fn, void *regs, void *floats, int stack_count, void *stack, int is_float);

int call(void *f, void **args, int *flags, int count, void **out)
{
    void *integers[MAX_INTEGER_COUNT];
    void *floats[MAX_FLOAT_COUNT];
    void *stack[MAX_STACK_COUNT];
    int integer_count = 0;
    int float_count = 0;
    int stack_count = 0;
    int ii;
    for (ii = 0; ii < count; ii++) {
        if (flags[ii] & ARG_FLAG_FLOAT) {
            if (float_count < MAX_FLOAT_COUNT) {
                floats[float_count++] = args[ii];
                continue;
            }
        } else {
            if (integer_count < MAX_INTEGER_COUNT) {
                integers[integer_count++] = args[ii];
                continue;
            }
        }
        if (stack_count > MAX_STACK_COUNT) {
            *out = strdup("maximum number of stack arguments reached (" _xstr(MAX_STACK_COUNT) ")");
            return 1;
        }
        // Argument on the stack
        stack[stack_count++] = args[ii];
    }
    void *floats_ptr = NULL;
    if (float_count > 0) {
        floats_ptr = floats;
    }
    if (stack_count & 1) {
        stack_count++;
    }
    for (ii = 0; ii < stack_count / 2; ii++) {
        int idx = stack_count-1-ii;
        void *tmp = stack[idx];
        stack[idx] = stack[ii];
        stack[ii] = tmp;
    }
    *out = make_call(f, integers, floats_ptr, stack_count, stack, flags[count] & ARG_FLAG_FLOAT);
    return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"github.com/adverax/echo/generic"
	"math"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

// http://github.com/rainycape/dl

const (
	LibExt = ".so"
)

const (
	// dlopen() flags. See man dlopen.
	RTLD_LAZY     = int(C.RTLD_LAZY)
	RTLD_NOW      = int(C.RTLD_NOW)
	RTLD_GLOBAL   = int(C.RTLD_GLOBAL)
	RTLD_LOCAL    = int(C.RTLD_LOCAL)
	RTLD_NODELETE = int(C.RTLD_NODELETE)
	RTLD_NOLOAD   = int(C.RTLD_NOLOAD)
)

type library struct {
	sync.Mutex
	handle   unsafe.Pointer
	routines map[string]*Routine
}

func (lib *library) Close() error {
	if lib.handle != nil {
		lib.Lock()
		defer lib.Unlock()

		if lib.handle != nil {
			mu.Lock()
			defer mu.Unlock()

			if C.dlclose(lib.handle) != 0 {
				return fmt.Errorf("close library: %w", dlerror())
			}
			lib.handle = nil
		}
	}

	return nil
}

func (lib *library) Define(routine *Routine) error {
	lib.Lock()
	defer lib.Unlock()

	s := C.CString(routine.Name)
	defer C.free(unsafe.Pointer(s))

	lib.routines[routine.Name] = routine
	handle := C.dlsym(lib.handle, s)
	if handle == nil {
		return dlerror()
	}

	routine.handle = handle

	return nil
}

func (lib *library) Symbol(name string, out interface{}) error {
	s := C.CString(name)
	defer C.free(unsafe.Pointer(s))

	mu.Lock()
	handle := C.dlsym(lib.handle, s)
	if handle == nil {
		err := dlerror()
		mu.Unlock()
		return fmt.Errorf("symbol: %w", err)
	}
	mu.Unlock()

	val := reflect.ValueOf(out)
	if !val.IsValid() || val.Kind() != reflect.Ptr {
		return fmt.Errorf("out must be a pointer, not %T", out)
	}
	if val.IsNil() {
		return errors.New("out can't be nil")
	}

	elem := val.Elem()
	switch elem.Kind() {
	case reflect.Int:
		// We treat Go's int as long, since it
		// varies depending on the platform bit size
		elem.SetInt(int64(*(*int)(handle)))
	case reflect.Int8:
		elem.SetInt(int64(*(*int8)(handle)))
	case reflect.Int16:
		elem.SetInt(int64(*(*int16)(handle)))
	case reflect.Int32:
		elem.SetInt(int64(*(*int32)(handle)))
	case reflect.Int64:
		elem.SetInt(int64(*(*int64)(handle)))
	case reflect.Uint:
		// We treat Go's uint as unsigned long, since it
		// varies depending on the platform bit size
		elem.SetUint(uint64(*(*uint)(handle)))
	case reflect.Uint8:
		elem.SetUint(uint64(*(*uint8)(handle)))
	case reflect.Uint16:
		elem.SetUint(uint64(*(*uint16)(handle)))
	case reflect.Uint32:
		elem.SetUint(uint64(*(*uint32)(handle)))
	case reflect.Uint64:
		elem.SetUint(uint64(*(*uint64)(handle)))
	case reflect.Uintptr:
		elem.SetUint(uint64(*(*uintptr)(handle)))
	case reflect.Float32:
		elem.SetFloat(float64(*(*float32)(handle)))
	case reflect.Float64:
		elem.SetFloat(float64(*(*float64)(handle)))
	case reflect.Func:
		typ := elem.Type()
		tr, err := makeTrampoline(typ, handle)
		if err != nil {
			return fmt.Errorf("symbol: %w", err)
		}
		v := reflect.MakeFunc(typ, tr)
		elem.Set(v)
	case reflect.Ptr:
		v := reflect.NewAt(elem.Type().Elem(), handle)
		elem.Set(v)
	case reflect.String:
		elem.SetString(C.GoString(*(**C.char)(handle)))
	case reflect.UnsafePointer:
		elem.SetPointer(handle)
	default:
		return fmt.Errorf("symbol: invalid out type %T", out)
	}

	return nil
}

func (lib *library) Call(name string, arguments ...interface{}) (res interface{}, err error) {
	// Find routine
	routine, err := lib.find(name)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}

	// Prepare arguments
	var argp *unsafe.Pointer
	count := len(routine.Args)
	args := make([]unsafe.Pointer, count)
	flags := make([]C.int, count+1)

	outFlag := C.int(0)
	if routine.Result != nil {
		kind := routine.Result.Type
		if kind == reflect.Float32 || kind == reflect.Float64 {
			outFlag |= C.ARG_FLAG_FLOAT
		}
	}
	flags[count] = outFlag

	if count > 0 {
		if len(arguments) < count {
			return false, fmt.Errorf("call: %w", fmt.Errorf("too few arguments in func %s", routine.Name))
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
				s := C.CString(v.String())
				defer C.free(unsafe.Pointer(s))
				args[ii] = unsafe.Pointer(s)
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			case reflect.Int:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				if v.Type().Size() == 4 {
					flags[ii] = C.ARG_FLAG_SIZE_32
				} else {
					flags[ii] = C.ARG_FLAG_SIZE_64
				}
			case reflect.Int8:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_8
			case reflect.Int16:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_16
			case reflect.Int32:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_32
			case reflect.Int64:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_64
			case reflect.Uint:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				if v.Type().Size() == 4 {
					flags[ii] = C.ARG_FLAG_SIZE_32
				} else {
					flags[ii] = C.ARG_FLAG_SIZE_64
				}
			case reflect.Uint8:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_8
			case reflect.Uint16:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_16
			case reflect.Uint32:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_32
			case reflect.Uint64:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_64
			case reflect.Float32:
				args[ii] = unsafe.Pointer(uintptr(math.Float32bits(float32(v.Float()))))
				flags[ii] |= C.ARG_FLAG_FLOAT | C.ARG_FLAG_SIZE_32
			case reflect.Float64:
				args[ii] = unsafe.Pointer(uintptr(math.Float64bits(v.Float())))
				flags[ii] |= C.ARG_FLAG_FLOAT | C.ARG_FLAG_SIZE_64
			case reflect.Ptr:
				args[ii] = unsafe.Pointer(v.Pointer())
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			case reflect.Slice:
				if v.Len() > 0 {
					args[ii] = unsafe.Pointer(v.Index(0).UnsafeAddr())
				}
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			case reflect.Uintptr:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			default:
				return false, fmt.Errorf("call: %w", fmt.Errorf("can't bind value of type %s", v.Type()))
			}
		}
		argp = &args[0]
	}

	// Call routine
	var ret unsafe.Pointer
	if C.call(routine.handle, argp, &flags[0], C.int(count), &ret) != 0 {
		s := C.GoString((*C.char)(ret))
		C.free(ret)
		return 0, fmt.Errorf("call: %w", errors.New(s))
	}

	// Prepare result
	if routine.Result == nil || routine.Result.Type == reflect.Invalid {
		return
	}

	var v reflect.Value
	vv := MakeValue(routine.Result.Type, routine.Result.Pointer)
	out := reflect.TypeOf(vv)

	switch out.Kind() {
	case reflect.Int:
		v = reflect.ValueOf(int(uintptr(ret)))
	case reflect.Int8:
		v = reflect.ValueOf(int8(uintptr(ret)))
	case reflect.Int16:
		v = reflect.ValueOf(int16(uintptr(ret)))
	case reflect.Int32:
		v = reflect.ValueOf(int32(uintptr(ret)))
	case reflect.Int64:
		v = reflect.ValueOf(int64(uintptr(ret)))
	case reflect.Uint:
		v = reflect.ValueOf(uint(uintptr(ret)))
	case reflect.Uint8:
		v = reflect.ValueOf(uint8(uintptr(ret)))
	case reflect.Uint16:
		v = reflect.ValueOf(uint16(uintptr(ret)))
	case reflect.Uint32:
		v = reflect.ValueOf(uint32(uintptr(ret)))
	case reflect.Uint64:
		v = reflect.ValueOf(uint64(uintptr(ret)))
	case reflect.Float32:
		v = reflect.ValueOf(math.Float32frombits(uint32(uintptr(ret))))
	case reflect.Float64:
		v = reflect.ValueOf(math.Float64frombits(uint64(uintptr(ret))))
	case reflect.Ptr:
		if out.Elem().Kind() == reflect.String && ret != nil {
			s := C.GoString((*C.char)(ret))
			v = reflect.ValueOf(&s)
			break
		}
		v = reflect.NewAt(out.Elem(), ret)
	case reflect.String:
		s := C.GoString((*C.char)(ret))
		v = reflect.ValueOf(s)
	case reflect.Uintptr:
		v = reflect.ValueOf(uintptr(ret))
	case reflect.UnsafePointer:
		v = reflect.ValueOf(ret)
	default:
		return reflect.Value{}, fmt.Errorf("call: %w", fmt.Errorf("can't retrieve value of type"))
	}

	return v.Interface(), nil
}

func (lib *library) find(name string) (*Routine, error) {
	lib.Lock()
	defer lib.Unlock()

	if routine, ok := lib.routines[name]; ok {
		return routine, nil
	}

	return nil, fmt.Errorf("find: %w", fmt.Errorf("function %q not found", name))
}

func Open(name string, flag int) (l Library, err error) {
	if flag&RTLD_LAZY == 0 && flag&RTLD_NOW == 0 {
		flag |= RTLD_NOW
	}
	if name != "" && filepath.Ext(name) == "" {
		name = name + LibExt
	}
	s := C.CString(name)
	defer C.free(unsafe.Pointer(s))
	mu.Lock()
	handle := C.dlopen(s, C.int(flag))
	if handle == nil {
		err = dlerror()
	}
	mu.Unlock()
	if err != nil {
		if runtime.GOOS == "linux" && name == "libc.so" {
			// In most distros libc.so is now a text file
			// and in order to dlopen() it the name libc.so.6
			// must be used.
			return Open(name+".6", flag)
		}
		return nil, fmt.Errorf("Open: %w", err)
	}

	l = &library{
		handle:   handle,
		routines: make(map[string]*Routine),
	}

	return l, nil
}

func dlerror() error {
	s := C.dlerror()
	return errors.New(C.GoString(s))
}

func makeTrampoline(typ reflect.Type, handle unsafe.Pointer) (rFunc, error) {
	numOut := typ.NumOut()
	if numOut > 1 {
		return nil, fmt.Errorf("makeTranspoline: %w", fmt.Errorf("C functions can return 0 or 1 values, not %d", numOut))
	}
	var out reflect.Type
	var kind reflect.Kind
	outFlag := C.int(0)
	if numOut == 1 {
		out = typ.Out(0)
		kind = out.Kind()
		if kind == reflect.Float32 || kind == reflect.Float64 {
			outFlag |= C.ARG_FLAG_FLOAT
		}
	}

	return func(in []reflect.Value) []reflect.Value {
		if typ.IsVariadic() && len(in) > 0 {
			last := in[len(in)-1]
			in = in[:len(in)-1]
			if last.Len() > 0 {
				for ii := 0; ii < last.Len(); ii++ {
					in = append(in, last.Index(ii))
				}
			}
		}

		count := len(in)
		args := make([]unsafe.Pointer, count)
		flags := make([]C.int, count+1)
		flags[count] = outFlag
		for ii, v := range in {
			if v.Type() == emptyType {
				v = reflect.ValueOf(v.Interface())
			}
			switch v.Kind() {
			case reflect.String:
				s := C.CString(v.String())
				defer C.free(unsafe.Pointer(s))
				args[ii] = unsafe.Pointer(s)
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			case reflect.Int:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				if v.Type().Size() == 4 {
					flags[ii] = C.ARG_FLAG_SIZE_32
				} else {
					flags[ii] = C.ARG_FLAG_SIZE_64
				}
			case reflect.Int8:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_8
			case reflect.Int16:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_16
			case reflect.Int32:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_32
			case reflect.Int64:
				args[ii] = unsafe.Pointer(uintptr(v.Int()))
				flags[ii] = C.ARG_FLAG_SIZE_64
			case reflect.Uint:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				if v.Type().Size() == 4 {
					flags[ii] = C.ARG_FLAG_SIZE_32
				} else {
					flags[ii] = C.ARG_FLAG_SIZE_64
				}
			case reflect.Uint8:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_8
			case reflect.Uint16:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_16
			case reflect.Uint32:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_32
			case reflect.Uint64:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] = C.ARG_FLAG_SIZE_64
			case reflect.Float32:
				args[ii] = unsafe.Pointer(uintptr(math.Float32bits(float32(v.Float()))))
				flags[ii] |= C.ARG_FLAG_FLOAT | C.ARG_FLAG_SIZE_32
			case reflect.Float64:
				args[ii] = unsafe.Pointer(uintptr(math.Float64bits(v.Float())))
				flags[ii] |= C.ARG_FLAG_FLOAT | C.ARG_FLAG_SIZE_64
			case reflect.Ptr:
				args[ii] = unsafe.Pointer(v.Pointer())
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			case reflect.Slice:
				if v.Len() > 0 {
					args[ii] = unsafe.Pointer(v.Index(0).UnsafeAddr())
				}
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			case reflect.Uintptr:
				args[ii] = unsafe.Pointer(uintptr(v.Uint()))
				flags[ii] |= C.ARG_FLAG_SIZE_PTR
			default:
				panic(fmt.Errorf("can't bind value of type %s", v.Type()))
			}
		}
		var argp *unsafe.Pointer
		if count > 0 {
			argp = &args[0]
		}
		var ret unsafe.Pointer
		if C.call(handle, argp, &flags[0], C.int(count), &ret) != 0 {
			s := C.GoString((*C.char)(ret))
			C.free(ret)
			panic(errors.New(s))
		}
		if numOut > 0 {
			var v reflect.Value
			switch kind {
			case reflect.Int:
				v = reflect.ValueOf(int(uintptr(ret)))
			case reflect.Int8:
				v = reflect.ValueOf(int8(uintptr(ret)))
			case reflect.Int16:
				v = reflect.ValueOf(int16(uintptr(ret)))
			case reflect.Int32:
				v = reflect.ValueOf(int32(uintptr(ret)))
			case reflect.Int64:
				v = reflect.ValueOf(int64(uintptr(ret)))
			case reflect.Uint:
				v = reflect.ValueOf(uint(uintptr(ret)))
			case reflect.Uint8:
				v = reflect.ValueOf(uint8(uintptr(ret)))
			case reflect.Uint16:
				v = reflect.ValueOf(uint16(uintptr(ret)))
			case reflect.Uint32:
				v = reflect.ValueOf(uint32(uintptr(ret)))
			case reflect.Uint64:
				v = reflect.ValueOf(uint64(uintptr(ret)))
			case reflect.Float32:
				v = reflect.ValueOf(math.Float32frombits(uint32(uintptr(ret))))
			case reflect.Float64:
				v = reflect.ValueOf(math.Float64frombits(uint64(uintptr(ret))))
			case reflect.Ptr:
				if out.Elem().Kind() == reflect.String && ret != nil {
					s := C.GoString((*C.char)(ret))
					v = reflect.ValueOf(&s)
					break
				}
				v = reflect.NewAt(out.Elem(), ret)
			case reflect.String:
				s := C.GoString((*C.char)(ret))
				v = reflect.ValueOf(s)
			case reflect.Uintptr:
				v = reflect.ValueOf(uintptr(ret))
			case reflect.UnsafePointer:
				v = reflect.ValueOf(ret)
			default:
				panic(fmt.Errorf("can't retrieve value of type %s", out))
			}
			return []reflect.Value{v}
		}
		return nil
	}, nil
}
