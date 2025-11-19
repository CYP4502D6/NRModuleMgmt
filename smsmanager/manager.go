package smsmanager

import (
	"log"
	"sync"
	"time"

	"nrmodule/atserial"
)

type Manager struct {
	nri             *atserial.NRInterface
	db              *SMSDatabase
	observerManager *SMSObserverManager

	checkInterval time.Duration
	lastCheckTime time.Time
	mu            sync.Mutex

	running  bool
	stopChan chan struct{}
}

func NewManager(nri *atserial.NRInterface, dbPath string, checkInterval time.Duration) (*Manager, error) {

	db, err := NewSMSDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		nri:             nri,
		db:              db,
		observerManager: NewSMSObserverManager(),
		checkInterval:   checkInterval,
		stopChan:        make(chan struct{}),
	}

	return manager, nil
}

func (m *Manager) RegisterObserver(observer SMSToObserver) {
	m.observerManager.Register(observer)
}

func (m *Manager) Start() error {

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}

	m.running = true
	m.mu.Unlock()

	log.Println("[SMSManager] start listening")

	go m.monitorLoop()
	return nil
}

func (m *Manager) Stop() {

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	close(m.stopChan)
	log.Println("[SMSManager] stop listening")
}

func (m *Manager) monitorLoop() {

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	m.checkAndProcessSMS()

	for {
		select {
		case <-ticker.C:
			m.checkAndProcessSMS()
		case <-m.stopChan:
			log.Println("[SMSManager] exiting the monitor loop")
		}
	}
}

func (m *Manager) checkAndProcessSMS() {

	log.Println("[SMSManager] checking SMS")

	smsList, err := m.nri.FetchSMS()
	if err != nil {
		log.Println("[SMSManager] fetch sms failed,", err)
		return
	}

	if len(smsList) == 0 {
		log.Println("[SMSManager] no incoming sms")
		return
	}

	var indicesToDelete []int
	newSMSCount := 0

	for _, sms := range smsList {

		dbID, isNew, err := m.db.InsertSMS(sms)
		if err != nil {
			log.Println("[SMSManager] insert sms to database failed,", err)
			continue
		}
		if isNew {
			log.Println("[SMSManager] new sms", dbID, " from", sms.Sender)
			m.observerManager.NotifyNewSMS(sms)
			newSMSCount++
		} else {
			log.Println("[SMSManager] existed sms", dbID, " from", sms.Sender)
		}

		indicesToDelete = append(indicesToDelete, sms.Indices)
	}
	
	if len(indicesToDelete) > 0 && len(indicesToDelete) <= 10 {
		err := m.nri.DeleteSMS(indicesToDelete)
		if err != nil {
			log.Println("[SMSManager] delete incoming sms failed", err)
		}
	} else if len(indicesToDelete) > 10 {

		lenDel := len(indicesToDelete)
		for i := 0; i < lenDel; i += 10 {
			end := i + 10
			if end > lenDel {
				end = lenDel
			}
			err := m.nri.DeleteSMS(indicesToDelete[i:end])
			if err != nil {
				log.Println("[SMSManager] delete incoming sms failed", err)
			}
		}
	}

	log.Println("[SMSManager] process complete, sms count:", len(smsList), ", new sms count", newSMSCount)
}

func (m *Manager) Close() error {

	m.Stop()
	return m.db.Close()
}

func (m *Manager) GetDBStats() (int64, error) {
	return m.db.GetSMSCount()
}

func (m *Manager) GetSMSByID(id int64) (*SMSRecord, error) {
	return m.db.GetSMSByID(id)
}

func (m *Manager) GetSMSByIDRange(start, end int64) ([]*SMSRecord, error) {
	return m.db.GetSMSByRange(start, end)
}
