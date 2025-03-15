package local_errors

import "fmt"

type InputError struct {
	Cause error
}

func Input(cause error) InputError {
	return InputError{Cause: cause}
}

func (err InputError) Error() string {
	return fmt.Sprintf("InputError: %v", err.Cause.Error())
}

type OutputError struct {
	Cause error
}

func Output(cause error) OutputError {
	return OutputError{Cause: cause}
}

func (err OutputError) Error() string {
	return fmt.Sprintf("OutputError: %v", err.Cause.Error())
}

type ProcessorError struct {
	Cause error
}

func Processor(cause error) OutputError {
	return OutputError{Cause: cause}
}

func (err ProcessorError) Error() string {
	return fmt.Sprintf("Processor: %v", err.Cause.Error())
}
