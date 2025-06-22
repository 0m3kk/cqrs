package event

import "github.com/0m3kk/eventus/eventsrc"

func RegisterAllEvents() {
	eventsrc.RegisterEvent(&ProductCreated{})
}
