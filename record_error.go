package playback

import (
	"context"
)

const errTypeContextDeadlineExceeded = "context.DeadlineExceeded"

var errTypes = map[string]error{
	errTypeContextDeadlineExceeded: context.DeadlineExceeded,
}

type RecordError struct {
	error
}

func (e RecordError) MarshalYAML() (interface{}, error) {
	if e.error == nil {
		return nil, nil
	}

	if e.error == context.DeadlineExceeded {
		return errTypeContextDeadlineExceeded, nil
	}

	return e.Error(), nil
}

func (e *RecordError) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var errString string
	if err := unmarshal(&errString); err != nil {
		return err
	}

	if err, ok := errTypes[errString]; ok {
		e.error = err
		return nil
	}

	err := unmarshal(&e)
	return err
}
