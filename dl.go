package dl

import "C"
import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"unsafe"
)

// Todo: rewrite error generation

type Library interface {
	// Close library
	Close() error
	// Call function
	Call(name string, args ...interface{}) (res interface{}, err error)
	// Define routine
	Define(routine *Routine) error
	// Get symbol (not implemented for windows yet)
	Symbol(name string, out interface{}) error
}

type Arg struct {
	Type    reflect.Kind
	Pointer bool
}

type Routine struct {
	Name    string
	Result  *Arg
	Args    []*Arg
	handle  unsafe.Pointer
	address uintptr
}

type rFunc func([]reflect.Value) []reflect.Value

var (
	emptyType = reflect.TypeOf((*interface{})(nil)).Elem()
	mu        sync.Mutex
)

var (
	funcRe = regexp.MustCompile(`(\w+)\s+(\*?)\s*([_\w\d]+)\(([^)]*)\)`)
	argsRe = regexp.MustCompile(`(\w+)\s+(\*?)\s*([_\w\d]+)`)
	types  = map[string]reflect.Kind{
		"bool":    reflect.Bool,
		"int":     reflect.Int,
		"int8":    reflect.Int8,
		"int16":   reflect.Int16,
		"int32":   reflect.Int32,
		"int64":   reflect.Int64,
		"uint":    reflect.Uint,
		"uint8":   reflect.Uint8,
		"uint16":  reflect.Uint16,
		"uint32":  reflect.Uint32,
		"uint64":  reflect.Uint64,
		"float32": reflect.Float32,
		"float64": reflect.Float64,
		"string":  reflect.String,
		"void":    reflect.UnsafePointer,
	}
)

// Parse routine definition
// signature in C format
// For example:
//   void diskSize(string device, int64 *size)
func ParseRoutineDefinition(def string) (*Routine, error) {
	matches := funcRe.FindStringSubmatch(def)
	if len(matches) < 3 {
		return nil, nil
	}

	typ := matches[1]
	ptr := matches[2]
	name := matches[3]
	var arguments []string
	s := strings.TrimSpace(matches[4])
	if s != "" {
		arguments = strings.Split(s, ",")
	}
	args := make([]*Arg, 0, len(arguments))
	for i, arg := range arguments {
		matches := argsRe.FindStringSubmatch(arg)
		if len(matches) < 2 {
			return nil, fmt.Errorf("error in %d argument", i)
		}
		typ := matches[1]
		ptr := matches[2]
		a, err := newArg(typ, ptr == "*")
		if err != nil {
			return nil, fmt.Errorf("ParseRoutineDefinition: %w", err)
		}
		args = append(args, a)
	}

	var res *Arg
	if typ != "void" {
		var err error
		res, err = newArg(typ, ptr == "*")
		if err != nil {
			return nil, fmt.Errorf("ParseRoutineDefinition: %w", err)
		}
	}

	return &Routine{
		Name:   name,
		Result: res,
		Args:   args,
	}, nil
}

func newArg(typ string, ptr bool) (*Arg, error) {
	tp, ok := types[typ]
	if !ok {
		return nil, fmt.Errorf("unknown type %q", typ)
	}

	return &Arg{
		Type:    tp,
		Pointer: ptr,
	}, nil
}

func OpenEx(
	filename string,
	routines []string,
) (Library, error) {
	lib, err := Open(filename, 0)
	if err != nil {
		return nil, fmt.Errorf("OpenEx: %w", err)
	}

	for _, routine := range routines {
		r, err := ParseRoutineDefinition(routine)
		if err != nil {
			return nil, fmt.Errorf("OpenEx: error in routine definition %q: %w", routine, err)
		}

		err = lib.Define(r)
		if err != nil {
			return nil, fmt.Errorf("OpenEx: %w", err)
		}
	}

	return lib, nil
}
