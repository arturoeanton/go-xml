package xml

import (
	"encoding/xml"
	"fmt"
)

// SyntaxError wraps the standard xml.SyntaxError but exposes fields publicly
// and adds context if available.
type SyntaxError struct {
	Msg  string
	Line int
	Err  error // Original underlying error
}

func (e *SyntaxError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("xml error at line %d: %s", e.Line, e.Msg)
	}
	return fmt.Sprintf("xml error: %s", e.Msg)
}

func (e *SyntaxError) Unwrap() error {
	return e.Err
}

// wrapError attempts to extract line info from standard XML errors
func wrapError(err error) error {
	if err == nil {
		return nil
	}

	// If it's already our type, return
	if _, ok := err.(*SyntaxError); ok {
		return err
	}

	// encoding/xml.SyntaxError
	if syntaxErr, ok := err.(*xml.SyntaxError); ok {
		return &SyntaxError{
			Msg:  syntaxErr.Msg,
			Line: syntaxErr.Line,
			Err:  err,
		}
	}

	// Fallback
	return err
}
