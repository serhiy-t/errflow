package errf

import (
	"fmt"
)

// IfErrorAssignTo is a helper method to set function return error value in defer calls.
// It is useful in functions that don't use 'defer errf.IfError()...' handlers.
// It is possible to use most of errf.With(...) configs.
// Unsupported configs (e.g. ReturnStrategyLast) will panic when used.
//
// Note: don't mix IfErrorAssignTo with 'defer errf.IfError()...' and Check* functions
// in the same function. They are not designed to work together.
//
// Example:
//  func example() (err error) {
//  	writer, err := openWriter(...)
//  	defer errf.With(
//  		errf.LogStrategyIfSuppressed,
//  	).IfErrorAssignTo(&err, writer.Close)
//
//  	// ...
//  }
func (ef *Errflow) IfErrorAssignTo(outErr *error, closeFn func() error) {
	err := closeFn()
	ef.applyDeferredOptions()
	if maySuppressFirstError(ef.returnStrategy) {
		panic(fmt.Errorf("%v is not supported for IfErrorAssignTo(...)", ef.returnStrategy))
	}
	if err != nil {
		if ef.wrapper != nil {
			err = ef.wrapper(err)
		}
		if err == nil {
			panic("error wrapper returned nil error")
		}
		if *outErr == nil {
			*outErr = err
			if ef.logStrategy == logStrategyAlways {
				globalLogFn(&LogMessage{
					Format: "%s",
					A:      []interface{}{err.Error()},
					Stack:  getStringErrorStackTraceFn(),
					Tags:   []string{"errorflow", "error"},
				})
			}
		} else {
			_, supp2, resultErr := getReturnStrategyImpl(ef.returnStrategy)(*outErr, err)
			*outErr = resultErr
			if (supp2 && ef.logStrategy == logStrategyIfSuppressed) || ef.logStrategy == logStrategyAlways {
				globalLogFn(&LogMessage{
					Format: "%s",
					A:      []interface{}{err.Error()},
					Stack:  getStringErrorStackTraceFn(),
					Tags:   []string{"errorflow", "suppressed-error"},
				})
			}
		}
	}
}

// IfErrorAssignTo is an alias for DefaultErrflow.IfErrorAssignTo(...).
func IfErrorAssignTo(outErr *error, closeFn func() error) {
	DefaultErrflow.IfErrorAssignTo(outErr, closeFn)
}
