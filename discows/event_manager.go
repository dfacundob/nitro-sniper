package discows

import (
	"sync"
)

type EventManager struct {
	handlers map[EventType][]*eventHandlerInstance
	mu       sync.RWMutex

	// run the events in their own goroutine
	RunEventsRoutine bool
}

func (em *EventManager) AddHandler(handler interface{}) {
	eh := handlerForInterface(handler)
	if eh == nil {
		return
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	if em.handlers == nil {
		em.handlers = make(map[EventType][]*eventHandlerInstance)
	}

	ehi := &eventHandlerInstance{eh}
	em.handlers[eh.Type()] = append(em.handlers[eh.Type()], ehi)
}

func (em *EventManager) DispatchEvent(s *Session, message *WSMessage) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.handlers == nil {
		return
	}

	// first emit the raw
	if handlers, has := em.handlers[EventTypeRaw]; has {
		var r = EventRaw{
			WSMessage: *message,
		}

		for _, handler := range handlers {
			if em.RunEventsRoutine {
				go handler.eventHandler.Handle(s, r)
			} else {
				handler.eventHandler.Handle(s, r)
			}
		}
	}

	// emit all the other handlers
	if message.DataParsed != nil {
		if handlers, has := em.handlers[message.T]; has {
			for _, handler := range handlers {
				if em.RunEventsRoutine {
					go handler.eventHandler.Handle(s, message.DataParsed)
				} else {
					handler.eventHandler.Handle(s, message.DataParsed)
				}
			}
		}
	}
}

func (em *EventManager) DispatchEventSessionClose(s *Session, err error, reconnect bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.handlers == nil {
		return
	}

	if handlers, has := em.handlers[EventTypeSessionClose]; has {
		var r = EventSessionClose{
			Error:     err,
			Reconnect: reconnect,
		}

		for _, handler := range handlers {
			if em.RunEventsRoutine {
				go handler.eventHandler.Handle(s, r)
			} else {
				handler.eventHandler.Handle(s, r)
			}
		}
	}
}

// eventHandlerInstance is a wrapper around an event handler, as functions
// cannot be compared directly.
type eventHandlerInstance struct {
	eventHandler EventHandler
}

func handlerForInterface(handler interface{}) EventHandler {
	switch v := handler.(type) {
	case func(*Session, EventRaw):
		return eventRawHandler(v)
	case func(*Session, EventSessionClose):
		return eventSessionCloseHandler(v)
	case func(*Session, EventReady):
		return eventReadyHandler(v)
	case func(*Session, EventResumed):
		return eventResumedHandler(v)
	case func(*Session, EventGuildCreate):
		return eventGuildCreateHandler(v)
	case func(*Session, EventGuildUpdate):
		return eventGuildUpdateHandler(v)
	case func(*Session, EventGuildDelete):
		return eventGuildDeleteHandler(v)
	case func(*Session, EventMessageCreate):
		return eventMessageCreateHandler(v)
	case func(*Session, EventMessageUpdate):
		return eventMessageUpdateHandler(v)
	case func(*Session, EventPresenceUpdate):
		return eventPresenceUpdateHandler(v)
	case func(*Session, EventUserUpdate):
		return eventUserUpdateHandler(v)
	}

	return nil
}
