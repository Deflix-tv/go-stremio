package stremio

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// -- string Value
// This code is copied from the stdlib.
type stringValue string

// This code is copied from the stdlib.
func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

// This code is copied from the stdlib.
func (s *stringValue) Get() interface{} { return string(*s) }

// This code is copied from the stdlib.
func (s *stringValue) String() string { return string(*s) }

// usage prints the usage of the flags an SDK user sets.
// It skips printing Fiber's `-prefork` and `-child`, that it defines as of Fiber v1.12.0.
// This code is based on the stdlib with the only change to skip those two flags.
func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		// Skip printing usage info for Fiber's flags
		if f.Name == "prefork" || f.Name == "child" {
			return
		}

		s := fmt.Sprintf("  -%s", f.Name) // Two spaces before -; see next two comments.
		name, usage := flag.UnquoteUsage(f)
		if len(name) > 0 {
			s += " " + name
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if len(s) <= 4 { // space, space, '-', 'x'.
			s += "\t"
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			s += "\n    \t"
		}
		s += strings.ReplaceAll(usage, "\n", "\n    \t")

		if !isZeroValue(f, f.DefValue) {
			if _, ok := f.Value.(*stringValue); ok {
				// put quotes on the value
				s += fmt.Sprintf(" (default %q)", f.DefValue)
			} else {
				s += fmt.Sprintf(" (default %v)", f.DefValue)
			}
		}
		fmt.Fprint(flag.CommandLine.Output(), s, "\n")
	})
}

// isZeroValue determines whether the string represents the zero
// value for a flag.
// This code is copied from the stdlib.
func isZeroValue(f *flag.Flag, value string) bool {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Ptr {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	return value == z.Interface().(flag.Value).String()
}
