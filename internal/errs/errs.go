package errs

import (
	"errors"
	"fmt"
	"io"
)

type Code = string

type ClipError struct {
	Code    Code
	Message string
	Hint    string
	Cause   error
}

func (e *ClipError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s %s", e.Code, e.Message)
}

func (e *ClipError) Unwrap() error { return e.Cause }

type opt func(*ClipError)

func HintOpt(h string) opt {
	return func(e *ClipError) { e.Hint = h }
}

func Cause(err error) opt {
	return func(e *ClipError) { e.Cause = err }
}

func E(code Code, msg string, opts ...opt) error {
	e := &ClipError{Code: code, Message: msg}
	for _, o := range opts {
		o(e)
	}
	return e
}

func Hint(h string) opt { return HintOpt(h) }

func WrapCode(code Code, msg string, err error) error {
	if err == nil {
		return nil
	}
	return E(code, msg, Cause(err))
}

func CodeOf(err error) Code {
	var ce *ClipError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

func Fprint(w io.Writer, err error) {
	if err == nil {
		return
	}
	var ce *ClipError
	if errors.As(err, &ce) {
		if ce.Code != "" {
			fmt.Fprintf(w, "%s %s", ce.Code, ce.Message)
		} else {
			fmt.Fprint(w, ce.Message)
		}
		if ce.Hint != "" {
			fmt.Fprintf(w, "\n提示: %s", ce.Hint)
		}
		if ce.Cause != nil {
			fmt.Fprintf(w, "\n原因: %v", ce.Cause)
		}
		return
	}

	fmt.Fprint(w, err.Error())
}
