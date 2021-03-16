# ErrorFlow
Declarative error handling for Go.

## Motivation

See articles:
* [Error Handling — Problem Overview](https://go.googlesource.com/proposal/+/master/design/go2draft-error-handling-overview.md)
* [Don't defer Close() on writable files
](https://www.joeshaw.org/dont-defer-close-on-writable-files/)

ErrorFlow goal is to provide a library solution to the issues above.

Library solution (as opposed to a language change), although less cleaner, has a very important benefit: it is optional.
Many language proposals for addressing this issue have been rejected because language change is required to be universally applicable.
Library solution can be used only for use cases where it works well.

## Features

* 'err'-variable-free type-safe branchless business logic
```go
reader := errf.Io.CheckReadCloser(os.Open(srcFilename))
	/* vs */
reader, err := os.Open(srcFilename)
if err != nil {
	return err
}
```

* Declarative multiple errors handling logic
```go
defer errf.IfError().ReturnFirst().ThenAssignTo(&err)
defer errf.CheckErr(writer.Close())
	/* vs */
defer func() {
	closeErr := writer.Close()
	if closeErr != nil and err == nil {
		err = closeErr
	}
}()
```
* Declarative errors logging logic
```go
defer errf.IfError().LogIfSuppressed().ThenAssignTo(&err)
defer errf.CheckErr(writer.Close())
	/* vs */
defer func() {
	closeErr := writer.Close()
	if closeErr != nil {
		if err == nil {
			err = closeErr
		} else {
			log.Printf("error closing writer: %w", err)
		}
	}
}()
```
* Doesn't affect APIs
  * Every use of ErrorFlow is scoped to a single function and doesn't leak into its API
* Extendable
  * Custom return types for type safety
  * Custom ErrorFlow config functions (e.g. creating a wrapper that converts errors from a third-party libraries into standard error types for internal codebase)

## Example: error handling for file gzip function

Error handling requirements for function:
* Returns error only in case of error that
affects result file correctness.
* Logs all internal errors that it didn't return.
* Wraps returned errors with "errors compression file: " prefix.
* Performs input parameters validation.

### ErrorFlow style error handling

```go
func GzipFile(dstFilename string, srcFilename string) (err error) {
	// defer IfError()... creates and configures
	// ErrorFlow error handler for this function.
	// When any of Check* functions encounters non-nil error
	// it immediately sends error to this handler
	// unwinding all stacked defers.
	errWrapper := errf.WrapperFmtErrorw("error compressing file")
	defer errf.IfError().ReturnFirst().LogIfSuppressed().Apply(errWrapper).ThenAssignTo(&err)

	errf.CheckCondition(len(dstFilename) == 0, "dst file should be specified")
	errf.CheckCondition(len(srcFilename) == 0, "src file should be specified")

	reader := errf.Io.CheckReadCloser(os.Open(srcFilename))
	defer errf.With(errWrapper).Log(reader.Close())

	writer := errf.Io.CheckWriteCloser(os.Create(dstFilename))
	defer errf.CheckErr(writer.Close())

	gzipWriter := gzip.NewWriter(writer)
	defer errf.CheckErr(gzipWriter.Close())

	return errf.CheckDiscard(io.Copy(gzipWriter, reader))
}
```

### Compare with

<details>
	<summary>Plain Go implementation without any error handling</summary>

```go
func GzipFile(dstFilename string, srcFilename string) error {
	reader, _ := os.Open(srcFilename)
	defer reader.Close()

	writer, _ := os.Create(dstFilename)
	defer writer.Close()

	gzipWriter := gzip.NewWriter(writer)
	defer gzipWriter.Close()

	_, _ = io.Copy(gzipWriter, reader)

	return nil
}
```
</details>

<details>
	<summary>Plain Go implementation (functionally roughly equivalent to ErrorFlow example above; not using helper functions)</summary>

```go
func GzipFile(dstFilename string, srcFilename string) (err error) {
	if len(dstFilename) == 0 {
		return fmt.Errorf("error compressing file: dst file should be specified")
	}
	if len(srcFilename) == 0 {
		return fmt.Errorf("error compressing file: src file should be specified")
	}

	reader, err := os.Open(srcFilename)
	if err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}
	defer func() {
		closeErr := reader.Close()
		if closeErr != nil {
			log.Println(closeErr)
		}
	}()

	writer, err := os.Create(dstFilename)
	if err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}
	defer func() {
		closeErr := writer.Close()
		if closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error compressing file: %w", closeErr)
			} else {
				log.Println(fmt.Errorf("[suppressed] error compressing file: %w", closeErr))
			}
		}
	}()

	gzipWriter := gzip.NewWriter(writer)
	defer func() {
		closeErr := gzipWriter.Close()
		if closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error compressing file: %w", closeErr)
			} else {
				log.Println(fmt.Errorf("[suppressed] error compressing file: %w", closeErr))
			}
		}
	}()

	_, err = io.Copy(gzipWriter, reader)
	if err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}

	return nil
}
```
</details>

<details>
	<summary>ErrorFlow-Lite style error handling (using only defer helper functions, but not IfError/Check* handler)</summary>

```go
func GzipFile(dstFilename string, srcFilename string) (err error) {
	errflow := errf.With(
		errf.LogStrategyIfSuppressed,
		errf.WrapperFmtErrorw("error compressing file"),
	)

	if len(dstFilename) == 0 {
		return fmt.Errorf("error compressing file: dst file should be specified")
	}
	if len(srcFilename) == 0 {
		return fmt.Errorf("error compressing file: src file should be specified")
	}

	reader, err := os.Open(srcFilename)
	if err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}
	defer errflow.Log(reader.Close())

	writer, err := os.Create(dstFilename)
	if err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}
	defer errflow.IfErrorAssignTo(&err, writer.Close())

	gzipWriter := gzip.NewWriter(writer)
	defer errflow.IfErrorAssignTo(&err, gzipWriter.Close())

	_, err = io.Copy(gzipWriter, reader)
	if err != nil {
		return fmt.Errorf("error compressing file: %w", err)
	}

	return nil
}
```
</details>