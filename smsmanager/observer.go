package smsmanager

import (
	"nrmodule/atserial"
)

type SMSToObserver interface {
	OnNewSMS(sms atserial.NRModuleSMS)
}

type SMSObserverManager struct {
	observers []SMSToObserver
}

func NewSMSObserverManager() *SMSObserverManager {

	return &SMSObserverManager{
		observers: make([]SMSToObserver, 0),
	}
}

func (om *SMSObserverManager) Register(observer SMSToObserver) {
	om.observers = append(om.observers, observer)
}

func (om *SMSObserverManager) NotifyNewSMS(sms atserial.NRModuleSMS) {

	for _, ob := range om.observers {
		go ob.OnNewSMS(sms)
	}
}
