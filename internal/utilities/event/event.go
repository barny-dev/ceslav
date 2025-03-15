package event

var eventTypeError = "error"
var eventTypeHeader = "header"
var eventTypeRow = "row"
var eventTypeSuccess = "success"

var SUCCESS = Success{}

type Success = struct{}

type Event struct {
	eventType    string
	eventError   error
	eventHeader  []string
	eventRow     []string
	eventSuccess *Success
}

func OfError(value error) Event {
	if value == nil {
		panic("value is nil")
	}
	return Event{
		eventType:  eventTypeError,
		eventError: value,
	}
}

func OfHeader(value []string) Event {
	if value == nil {
		panic("value is nil")
	}
	return Event{
		eventType:   eventTypeHeader,
		eventHeader: value,
	}
}

func OfRow(value []string) Event {
	if value == nil {
		panic("value is nil")
	}
	return Event{
		eventType: eventTypeRow,
		eventRow:  value,
	}
}

func OfSuccess(success Success) Event {
	return Event{
		eventType:    eventTypeSuccess,
		eventSuccess: &success,
	}
}

func (event *Event) AsError() error {
	return event.eventError
}

func (event *Event) AsHeader() []string {
	return event.eventHeader
}

func (event *Event) AsRow() []string {
	return event.eventRow
}

func (event *Event) AsSuccess() *struct{} {
	return event.eventSuccess
}

func (event *Event) IsError() bool {
	return event.eventType == eventTypeError
}

func (event *Event) IsHeader() bool {
	return event.eventType == eventTypeHeader
}

func (event *Event) IsRow() bool {
	return event.eventType == eventTypeRow
}

func (event *Event) IsSuccess() bool {
	return event.eventType == eventTypeSuccess
}
